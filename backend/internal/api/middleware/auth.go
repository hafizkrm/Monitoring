package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourusername/viscod/internal/config"
)

type contextKey string

const UserContextKey = contextKey("user_id")
const UserNameContextKey = contextKey("username")
const UserRoleContextKey = contextKey("role")

// AuthMiddleware extracts the JWT from the Authorization header and verifies it.
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle preflight OPTIONS requests to bypass auth
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			tokenStr := ""
			if cookie, err := r.Cookie("netmon_token"); err == nil {
				tokenStr = cookie.Value
			}

			if tokenStr == "" {
				fmt.Println("[Auth Debug] Missing netmon_token cookie for URL:", r.URL.Path)
				http.Error(w, "Missing authentication cookie", http.StatusUnauthorized)
				return
			}
			jwtSecret := cfg.App.GetJWTSecret()
			if jwtSecret == "" {
				http.Error(w, "JWT_SECRET is not configured on the server", http.StatusInternalServerError)
				return
			}

			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				fmt.Println("[Auth Debug] Invalid token error:", err)
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			userIDFloat, ok := claims["user_id"].(float64)
			if !ok {
				http.Error(w, "Invalid user id in token", http.StatusUnauthorized)
				return
			}

			userID := int(userIDFloat)

			username, _ := claims["username"].(string)
			if username == "" {
				username = "Admin"
			}

			role, _ := claims["role"].(string)
			if role == "" {
				role = "viewer"
			}

			ctx := context.WithValue(r.Context(), UserContextKey, userID)
			ctx = context.WithValue(ctx, UserNameContextKey, username)
			ctx = context.WithValue(ctx, UserRoleContextKey, role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks if the user has one of the allowed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRoleRaw := r.Context().Value(UserRoleContextKey)
			if userRoleRaw == nil {
				http.Error(w, "Forbidden - No role found", http.StatusForbidden)
				return
			}

			userRole, ok := userRoleRaw.(string)
			if !ok {
				http.Error(w, "Forbidden - Invalid role type", http.StatusForbidden)
				return
			}

			allowed := false
			for _, role := range roles {
				if userRole == role {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Forbidden - Insufficient Permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
