package users

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net"
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
	sessions  *repository.AuthSessionRepository
	db        *sql.DB
	jwtSecret string
}

func NewUsersHandler(db *sql.DB, jwtSecret string) *UsersHandler {
	return &UsersHandler{
		repo: repository.NewUserRepository(db), sessions: repository.NewAuthSessionRepository(db),
		db: db, jwtSecret: jwtSecret,
	}
}

type loginAttempt struct {
	count     int
	inFlight  int
	lockoutAt time.Time
	lastSeen  time.Time
}

var (
	loginMu                 sync.Mutex
	loginAttempts           = make(map[string]*loginAttempt)
	lastLoginAttemptCleanup time.Time
)

func beginLoginAttempt(ip string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	now := time.Now()
	cleanupLoginAttempts(now, ip)

	attempt, exists := loginAttempts[ip]
	if !exists {
		if len(loginAttempts) >= 10000 {
			evictOldestLoginAttempt()
		}
		attempt = &loginAttempt{}
		loginAttempts[ip] = attempt
		exists = true
	}
	attempt.lastSeen = now
	if now.Before(attempt.lockoutAt) {
		return false
	}
	if attempt.count+attempt.inFlight >= 5 {
		attempt.lockoutAt = now.Add(time.Minute)
		return false
	}
	attempt.inFlight++
	return true
}

func cancelLoginAttempt(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	if attempt := loginAttempts[ip]; attempt != nil {
		if attempt.inFlight > 0 {
			attempt.inFlight--
		}
		attempt.lastSeen = time.Now()
	}
}

func recordLoginAttempt(ip string, success bool) {
	loginMu.Lock()
	defer loginMu.Unlock()
	now := time.Now()
	cleanupLoginAttempts(now, ip)
	attempt := loginAttempts[ip]
	if attempt == nil {
		return
	}
	if attempt.inFlight > 0 {
		attempt.inFlight--
	}
	attempt.lastSeen = now
	if success {
		delete(loginAttempts, ip)
		return
	}
	attempt.count++
	if attempt.count >= 5 {
		attempt.lockoutAt = now.Add(time.Minute)
	}
}

func cleanupLoginAttempts(now time.Time, preserveIP string) {
	if !lastLoginAttemptCleanup.IsZero() && now.Sub(lastLoginAttemptCleanup) < time.Minute {
		return
	}
	lastLoginAttemptCleanup = now
	for ip, attempt := range loginAttempts {
		if ip != preserveIP && now.Sub(attempt.lastSeen) > 10*time.Minute && !now.Before(attempt.lockoutAt) {
			delete(loginAttempts, ip)
		}
	}
}

func evictOldestLoginAttempt() {
	var oldestIP string
	var oldest time.Time
	for ip, attempt := range loginAttempts {
		if oldestIP == "" || attempt.lastSeen.Before(oldest) {
			oldestIP, oldest = ip, attempt.lastSeen
		}
	}
	delete(loginAttempts, oldestIP)
}

func (h *UsersHandler) insertActivityLog(r *http.Request, user, action, module, description string) error {
	if user == "" {
		if nameRaw := r.Context().Value(nms_middleware.UserNameContextKey); nameRaw != nil {
			user, _ = nameRaw.(string)
		} else {
			user = "System"
		}
	}
	query := `INSERT INTO activity_logs (username, action, module, description, ip_address) VALUES (?, ?, ?, ?, ?)`
	_, err := h.db.ExecContext(r.Context(), query, user, action, module, description, nms_middleware.GetRealIP(r))
	return err
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
	payload.Username = strings.TrimSpace(payload.Username)

	if strings.TrimSpace(payload.Username) == "" || len(payload.Password) < 6 || len(payload.Password) > 72 {
		http.Error(w, "Username wajib diisi dan password harus 6-72 byte", http.StatusBadRequest)
		return
	}

	validRole := payload.Role == "admin" || payload.Role == "viewer"
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
	payload.Username = strings.TrimSpace(payload.Username)

	validRole := payload.Role == "admin" || payload.Role == "viewer"
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
		if len(payload.Password) < 6 || len(payload.Password) > 72 {
			http.Error(w, "Password harus 6-72 byte", http.StatusBadRequest)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Gagal memproses password", http.StatusInternalServerError)
			return
		}
		user.PasswordHash = string(hash)
	}
	if err := h.repo.UpdateAndRevokeSessions(r.Context(), user); err != nil {
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
	if err := h.repo.DeleteAndRevokeSessions(r.Context(), payload.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	h.insertActivityLog(r, "", "DELETE", "Users", fmt.Sprintf("Menghapus user dengan ID %d", payload.ID))
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
}

func (h *UsersHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !nms_middleware.IsHTTPSRequest(r) && !isLoopbackRequest(r) {
		writeLoginError(w, http.StatusBadRequest, "Login membutuhkan HTTPS")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
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
	if !beginLoginAttempt(ip) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"message": "Terlalu banyak percobaan login. Silakan coba lagi sebentar lagi."})
		return
	}

	username := strings.TrimSpace(payload.Username)
	if username == "" || payload.Password == "" {
		recordLoginAttempt(ip, false)
		writeLoginError(w, http.StatusUnauthorized, "Username atau password salah")
		return
	}
	user, err := h.repo.GetByUsername(username)
	if err != nil {
		cancelLoginAttempt(ip)
		writeLoginError(w, http.StatusServiceUnavailable, "Layanan autentikasi sementara tidak tersedia")
		return
	}
	if user == nil {
		recordLoginAttempt(ip, false)
		writeLoginError(w, http.StatusUnauthorized, "Username atau password salah")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)); err != nil {
		recordLoginAttempt(ip, false)
		writeLoginError(w, http.StatusUnauthorized, "Username atau password salah")
		return
	}

	if h.jwtSecret == "" || h.sessions == nil {
		cancelLoginAttempt(ip)
		writeLoginError(w, http.StatusInternalServerError, "Layanan autentikasi belum dikonfigurasi")
		return
	}

	storedRole := user.Role
	user.Role = repository.NormalizeRole(storedRole)
	if user.Role != "admin" && user.Role != "viewer" {
		recordLoginAttempt(ip, false)
		writeLoginError(w, http.StatusUnauthorized, "Username atau password salah")
		return
	}
	now := time.Now().UTC()
	session, refreshToken, err := h.sessions.Create(r.Context(), user.ID, user.Role, storedRole, user.PasswordHash, now)
	if err != nil {
		cancelLoginAttempt(ip)
		writeLoginError(w, http.StatusInternalServerError, "Gagal membuat sesi")
		return
	}
	if err := h.insertActivityLog(r, user.Username, "LOGIN", "Auth", "User berhasil login"); err != nil {
		_ = h.sessions.RevokeByID(r.Context(), session.ID)
		cancelLoginAttempt(ip)
		writeLoginError(w, http.StatusInternalServerError, "Login gagal dicatat; coba lagi")
		return
	}
	if err := h.setSessionCookies(w, r, session, refreshToken, now); err != nil {
		_ = h.sessions.RevokeByID(r.Context(), session.ID)
		cancelLoginAttempt(ip)
		writeLoginError(w, http.StatusInternalServerError, "Gagal membuat sesi")
		return
	}
	recordLoginAttempt(ip, true)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user":   user,
	})
}

func (h *UsersHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var revokeErr error
	if cookie, err := r.Cookie("netmon_refresh"); err == nil && h.sessions != nil {
		revokeErr = h.sessions.RevokeByRefreshToken(r.Context(), cookie.Value)
	}
	if cookie, err := r.Cookie("netmon_token"); err == nil && h.sessions != nil && h.jwtSecret != "" {
		if sessionID := verifiedSessionID(cookie.Value, h.jwtSecret); sessionID != "" {
			if err := h.sessions.RevokeByID(r.Context(), sessionID); revokeErr == nil {
				revokeErr = err
			}
		}
	}
	clearSessionCookies(w, r)
	if revokeErr != nil {
		writeLoginError(w, http.StatusInternalServerError, "Gagal mencabut sesi")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Berhasil logout"})
}

func verifiedSessionID(rawToken, secret string) string {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithoutClaimsValidation())
	if err != nil || token == nil || !token.Valid {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	sessionID, _ := claims["sid"].(string)
	return sessionID
}

func (h *UsersHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("netmon_refresh")
	if err != nil || h.sessions == nil || h.jwtSecret == "" {
		clearSessionCookies(w, r)
		writeLoginError(w, http.StatusUnauthorized, "Sesi berakhir; silakan masuk kembali")
		return
	}
	now := time.Now().UTC()
	session, refreshToken, err := h.sessions.Refresh(r.Context(), cookie.Value, now)
	if err != nil {
		clearSessionCookies(w, r)
		if errors.Is(err, repository.ErrSessionInvalid) {
			writeLoginError(w, http.StatusUnauthorized, "Sesi berakhir; silakan masuk kembali")
		} else {
			writeLoginError(w, http.StatusServiceUnavailable, "Layanan sesi sementara tidak tersedia")
		}
		return
	}
	if err := h.setSessionCookies(w, r, session, refreshToken, now); err != nil {
		_ = h.sessions.RevokeByID(r.Context(), session.ID)
		writeLoginError(w, http.StatusInternalServerError, "Gagal memperbarui sesi")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "user": map[string]interface{}{
		"id": session.UserID, "username": session.Username, "name": session.Name, "role": session.Role,
	}})
}

func (h *UsersHandler) setSessionCookies(w http.ResponseWriter, r *http.Request, session repository.AuthSession, refreshToken string, now time.Time) error {
	expiresAt := now.Add(time.Hour)
	if session.ExpiresAt.Before(expiresAt) {
		expiresAt = session.ExpiresAt
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sid": session.ID,
		"exp": expiresAt.Unix(),
	})
	accessToken, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return err
	}
	secure := nms_middleware.IsHTTPSRequest(r)
	for _, cookie := range []*http.Cookie{
		{Name: "netmon_token", Value: accessToken},
		{Name: "netmon_refresh", Value: refreshToken},
	} {
		cookie.Path = "/"
		cookie.HttpOnly = true
		cookie.Secure = secure
		cookie.SameSite = http.SameSiteLaxMode
		http.SetCookie(w, cookie)
	}
	return nil
}

func clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{"netmon_token", "netmon_refresh"} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1,
			HttpOnly: true, Secure: nms_middleware.IsHTTPSRequest(r), SameSite: http.SameSiteLaxMode,
		})
	}
}

func writeLoginError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func isLoopbackRequest(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

func (h *UsersHandler) Session(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(nms_middleware.UserContextKey).(int)
	username, nameOK := r.Context().Value(nms_middleware.UserNameContextKey).(string)
	displayName, displayNameOK := r.Context().Value(nms_middleware.UserDisplayNameContextKey).(string)
	role, roleOK := r.Context().Value(nms_middleware.UserRoleContextKey).(string)
	if !ok || !nameOK || !displayNameOK || !roleOK {
		writeLoginError(w, http.StatusUnauthorized, "Sesi tidak valid")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "user": map[string]interface{}{
		"id": userID, "username": username, "name": displayName, "role": role,
	}})
}
