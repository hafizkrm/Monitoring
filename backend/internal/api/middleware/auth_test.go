package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourusername/viscod/internal/config"
)

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name           string
		userRole       interface{} // Role to put in context
		allowedRoles   []string    // Roles passed to RequireRole
		expectedStatus int
	}{
		{
			name:           "Admin allowed",
			userRole:       "admin",
			allowedRoles:   []string{"admin"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "View blocked from admin route",
			userRole:       "view",
			allowedRoles:   []string{"admin"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "View allowed on view route",
			userRole:       "view",
			allowedRoles:   []string{"admin", "view", "viewer"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "No role blocked",
			userRole:       nil,
			allowedRoles:   []string{"admin"},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireRole(tt.allowedRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.userRole != nil {
				ctx := context.WithValue(req.Context(), UserRoleContextKey, tt.userRole)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.JWTSecret = "test_secret"

	// Create valid token
	validToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  float64(1),
		"username": "admin_user",
		"role":     "admin",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	validTokenStr, _ := validToken.SignedString([]byte("test_secret"))

	// invalidToken section removed

	tests := []struct {
		name           string
		cookieName     string
		cookieValue    string
		expectedStatus int
	}{
		{
			name:           "Valid Cookie Token",
			cookieName:     "netmon_token",
			cookieValue:    validTokenStr,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing Token",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := AuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookieName != "" {
				req.AddCookie(&http.Cookie{Name: tt.cookieName, Value: tt.cookieValue})
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}
		})
	}
}
