package middleware

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/repository"
)

type contextKey string

const UserContextKey = contextKey("user_id")
const UserNameContextKey = contextKey("username")
const UserDisplayNameContextKey = contextKey("user_name")
const UserRoleContextKey = contextKey("role")
const SessionIDContextKey = contextKey("session_id")

type SessionLookup interface {
	GetByID(context.Context, string, time.Time) (repository.AuthSession, error)
}

func AuthMiddleware(cfg *config.Config, sessions SessionLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			cookie, err := r.Cookie("netmon_token")
			if err != nil || cookie.Value == "" {
				http.Error(w, "Missing authentication cookie", http.StatusUnauthorized)
				return
			}
			secret := cfg.App.GetJWTSecret()
			if secret == "" {
				http.Error(w, "Authentication is not configured", http.StatusInternalServerError)
				return
			}

			token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(secret), nil
			}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
			if err != nil || !token.Valid {
				http.Error(w, "Invalid session", http.StatusUnauthorized)
				return
			}
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid session", http.StatusUnauthorized)
				return
			}
			sessionID, ok := claims["sid"].(string)
			if !ok || sessionID == "" || sessions == nil {
				http.Error(w, "Invalid session", http.StatusUnauthorized)
				return
			}

			session, err := sessions.GetByID(r.Context(), sessionID, time.Now().UTC())
			if err != nil {
				if errors.Is(err, repository.ErrSessionInvalid) {
					http.Error(w, "Session expired or revoked", http.StatusUnauthorized)
				} else {
					http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
				}
				return
			}
			role := repository.NormalizeRole(session.Role)
			if role != "admin" && role != "viewer" {
				http.Error(w, "Invalid session role", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserContextKey, session.UserID)
			ctx = context.WithValue(ctx, UserNameContextKey, session.Username)
			ctx = context.WithValue(ctx, UserDisplayNameContextKey, session.Name)
			ctx = context.WithValue(ctx, UserRoleContextKey, role)
			ctx = context.WithValue(ctx, SessionIDContextKey, session.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// IsHTTPSRequest only trusts forwarded protocol when request came from a trusted proxy.
func IsHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || !isTrustedProxy(parsed) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

// RequireRole checks if the current session has one of the allowed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, _ := r.Context().Value(UserRoleContextKey).(string)
			for _, role := range roles {
				if userRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "Forbidden - Insufficient Permissions", http.StatusForbidden)
		})
	}
}
