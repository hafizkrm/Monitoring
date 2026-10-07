package users

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"fmt"

	"github.com/golang-jwt/jwt/v5"
	nms_middleware "github.com/hafizkrm/Monitoring/backend/internal/api/middleware"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type UsersHandler struct {
	repo      *repository.UserRepository
	db        *sql.DB
	jwtSecret string
}

func NewUsersHandler(db *sql.DB, jwtSecret string) *UsersHandler {
	return &UsersHandler{repo: repository.NewUserRepository(db), db: db, jwtSecret: jwtSecret}
}

type loginAttempt struct {
	count     int
	lockoutAt time.Time
}

var (
	loginMu       sync.Mutex
	loginAttempts = make(map[string]*loginAttempt)
)

func checkLoginRateLimit(ip string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()

	attempt, exists := loginAttempts[ip]
	if !exists {
		return true
	}
	if time.Now().Before(attempt.lockoutAt) {
		return false
	}
	if attempt.count >= 5 {
		attempt.count = 0
	}
	return true
}

func recordLoginAttempt(ip string, success bool) {
	loginMu.Lock()
	defer loginMu.Unlock()

	if success {
		delete(loginAttempts, ip)
		return
	}

	attempt, exists := loginAttempts[ip]
	if !exists {
		attempt = &loginAttempt{}
		loginAttempts[ip] = attempt
	}
	attempt.count++
	if attempt.count >= 5 {
		attempt.lockoutAt = time.Now().Add(1 * time.Minute) // 1 minute lockout
	}
}

func (h *UsersHandler) insertActivityLog(r *http.Request, user, action, module, description string) {
	if user == "" {
		if nameRaw := r.Context().Value(nms_middleware.UserNameContextKey); nameRaw != nil {
			user = nameRaw.(string)
		} else {
			user = "System"
		}
	}
	query := `INSERT INTO activity_logs (username, action, module, description, ip_address) VALUES (?, ?, ?, ?, ?)`
	_, _ = h.db.ExecContext(r.Context(), query, user, action, module, description, r.RemoteAddr)
}

func (h *UsersHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if users == nil {
		users = []*models.User{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   users,
	})
}

func (h *UsersHandler) AddUser(w http.ResponseWriter, r *http.Request) {
	var payload models.UserPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if payload.Username == "" || len(payload.Password) < 6 {
		http.Error(w, "Username must not be empty and password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	validRole := payload.Role == "admin" || payload.Role == "viewer" || payload.Role == "view"
	if !validRole {
		http.Error(w, "Invalid role specified", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	user := &models.User{
		Name:         payload.Name,
		Username:     payload.Username,
		PasswordHash: string(hash),
		Role:         payload.Role,
	}
	if err := h.repo.Create(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	h.insertActivityLog(r, "", "CREATE", "Users", fmt.Sprintf("Menambahkan user baru %s", payload.Username))
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "data": user})
}

func (h *UsersHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var payload models.UserPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	validRole := payload.Role == "admin" || payload.Role == "viewer" || payload.Role == "view"
	if !validRole {
		http.Error(w, "Invalid role specified", http.StatusBadRequest)
		return
	}
	user := &models.User{
		ID:       payload.ID,
		Name:     payload.Name,
		Username: payload.Username,
		Role:     payload.Role,
	}
	if payload.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
		user.PasswordHash = string(hash)
	}
	if err := h.repo.Update(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	h.insertActivityLog(r, "", "UPDATE", "Users", fmt.Sprintf("Mengubah data user dengan ID %d", payload.ID))
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
}

func (h *UsersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userIDRaw := r.Context().Value(nms_middleware.UserContextKey)
	if userIDRaw != nil {
		currentUserID, ok := userIDRaw.(int)
		if ok && currentUserID == payload.ID {
			http.Error(w, "Admin cannot delete their own account", http.StatusForbidden)
			return
		}
	}
	if err := h.repo.Delete(payload.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	h.insertActivityLog(r, "", "DELETE", "Users", fmt.Sprintf("Menghapus user dengan ID %d", payload.ID))
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
}

func (h *UsersHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1048576) // 1 MB limit
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Format request tidak valid"})
		return
	}
	ip := nms_middleware.GetRealIP(r)
	if !checkLoginRateLimit(ip) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"message": "Terlalu banyak percobaan login. Silakan coba lagi sebentar lagi."})
		return
	}

	username := strings.TrimSpace(payload.Username)
	user, err := h.repo.GetByUsername(username)
	if err != nil || user == nil {
		recordLoginAttempt(ip, false)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Username atau password salah"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)); err != nil {
		recordLoginAttempt(ip, false)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Username atau password salah"})
		return
	}

	recordLoginAttempt(ip, true)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Hour * 1).Unix(),
	})

	secret := h.jwtSecret
	if secret == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "JWT_SECRET is not configured on the server"})
		return
	}
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Gagal menghasilkan token"})
		return
	}

	// Set HttpOnly Cookie for enhanced XSS protection
	http.SetCookie(w, &http.Cookie{
		Name:     "netmon_token",
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(1 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	h.insertActivityLog(r, user.Username, "LOGIN", "Auth", "User berhasil login")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user":   user,
	})
}

func (h *UsersHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "netmon_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Berhasil logout"})
}

// Refresh renews the JWT token using the existing valid cookie
func (h *UsersHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("netmon_token")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	secret := h.jwtSecret
	token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Create new token
	claims["exp"] = time.Now().Add(1 * time.Hour).Unix()
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := newToken.SignedString([]byte(secret))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "netmon_token",
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(1 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
