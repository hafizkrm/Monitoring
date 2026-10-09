package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/repository"
)

type testSessionLookup struct {
	session repository.AuthSession
	err     error
}

func (s testSessionLookup) GetByID(context.Context, string, time.Time) (repository.AuthSession, error) {
	return s.session, s.err
}

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
			name:           "Viewer blocked from admin route",
			userRole:       "viewer",
			allowedRoles:   []string{"admin"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Viewer allowed on dashboard route",
			userRole:       "viewer",
			allowedRoles:   []string{"admin", "viewer"},
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
		"sid": "session-id",
		"exp": time.Now().Add(time.Hour).Unix(),
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
			sessions := testSessionLookup{session: repository.AuthSession{ID: "session-id", UserID: 1, Username: "admin_user", Role: "admin"}}
			handler := AuthMiddleware(cfg, sessions)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestIsHTTPSRequestTrustsConfiguredProxyOnly(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "192.0.2.1/32")
	req := httptest.NewRequest(http.MethodGet, "http://nms.example/", nil)
	req.RemoteAddr = "192.0.2.1:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	if !IsHTTPSRequest(req) {
		t.Fatal("trusted proxy HTTPS request not recognized")
	}
	req.RemoteAddr = "198.51.100.2:8080"
	if IsHTTPSRequest(req) {
		t.Fatal("untrusted forwarded protocol was accepted")
	}
}
