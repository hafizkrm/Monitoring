package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hafizkrm/Monitoring/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginCreatesRevocableSecureSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	password := "correct-horse-battery"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT id, name, username, password_hash, role, created_at, updated_at FROM users WHERE username = \\? AND is_active = 1").
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "username", "password_hash", "role", "created_at", "updated_at"}).
			AddRow(4, "Admin", "admin", string(hash), "admin", time.Now(), time.Now()))
	mock.ExpectExec("DELETE FROM auth_sessions WHERE expires_at <= \\? LIMIT 500").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO auth_sessions").
		WithArgs(sqlmock.AnyArg(), "admin", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 4, "admin", string(hash)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO activity_logs").
		WithArgs("admin", "LOGIN", "Auth", "User berhasil login", "192.0.2.1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	handler := NewUsersHandler(db, "test-secret")
	req := httptest.NewRequest(http.MethodPost, "https://nms.example/api/login", strings.NewReader(`{"username":"admin","password":"correct-horse-battery"}`))
	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	user := response["user"].(map[string]any)
	if user["role"] != "admin" || user["id"] != float64(4) {
		t.Fatalf("unexpected login user response: %#v", user)
	}
	if _, hasHash := user["password_hash"]; hasHash {
		t.Fatal("login response exposed password hash")
	}
	secureCookies := 0
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "netmon_token" || cookie.Name == "netmon_refresh" {
			if !cookie.HttpOnly || !cookie.Secure || !cookie.Expires.IsZero() || cookie.MaxAge != 0 {
				t.Fatalf("unexpected auth cookie settings: %#v", cookie)
			}
			secureCookies++
		}
	}
	if secureCookies != 2 {
		t.Fatalf("got %d auth cookies; want access and refresh", secureCookies)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoginDatabaseFailureReturnsServiceUnavailable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id, name, username, password_hash, role, created_at, updated_at FROM users WHERE username = \\? AND is_active = 1").
		WithArgs("admin").WillReturnError(errors.New("database unavailable"))

	handler := NewUsersHandler(db, "test-secret")
	req := httptest.NewRequest(http.MethodPost, "https://nms.example/api/login", strings.NewReader(`{"username":"admin","password":"password"}`))
	rr := httptest.NewRecorder()
	handler.Login(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("login status = %d; want %d", rr.Code, http.StatusServiceUnavailable)
	}
	if strings.Contains(strings.ToLower(rr.Body.String()), "password") && strings.Contains(rr.Body.String(), "database unavailable") {
		t.Fatal("database error leaked in response")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoginAuditFailureRevokesNewSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT id, name, username, password_hash, role, created_at, updated_at FROM users WHERE username = \\? AND is_active = 1").
		WithArgs("admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "username", "password_hash", "role", "created_at", "updated_at"}).
			AddRow(4, "Admin", "admin", string(hash), "admin", time.Now(), time.Now()))
	mock.ExpectExec("DELETE FROM auth_sessions WHERE expires_at <= \\? LIMIT 500").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO auth_sessions").
		WithArgs(sqlmock.AnyArg(), "admin", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 4, "admin", string(hash)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO activity_logs").
		WillReturnError(errors.New("audit storage unavailable"))
	mock.ExpectExec("UPDATE auth_sessions SET revoked_at").
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))

	handler := NewUsersHandler(db, "test-secret")
	req := httptest.NewRequest(http.MethodPost, "https://nms.example/api/login", strings.NewReader(`{"username":"admin","password":"password"}`))
	rr := httptest.NewRecorder()
	handler.Login(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("login status = %d; want %d", rr.Code, http.StatusInternalServerError)
	}
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == "netmon_token" || cookie.Name == "netmon_refresh" {
			t.Fatal("session cookies set despite audit failure")
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyRoleNormalize(t *testing.T) {
	if repository.NormalizeRole("view") != "viewer" {
		t.Fatal("legacy view role not normalized")
	}
}

func TestConcurrentLoginReservationsEnforceLimit(t *testing.T) {
	loginMu.Lock()
	loginAttempts = make(map[string]*loginAttempt)
	lastLoginAttemptCleanup = time.Time{}
	loginMu.Unlock()
	t.Cleanup(func() {
		loginMu.Lock()
		loginAttempts = make(map[string]*loginAttempt)
		lastLoginAttemptCleanup = time.Time{}
		loginMu.Unlock()
	})

	const ip = "192.0.2.200"
	for i := 0; i < 5; i++ {
		if !beginLoginAttempt(ip) {
			t.Fatalf("reservation %d unexpectedly blocked", i+1)
		}
	}
	if beginLoginAttempt(ip) {
		t.Fatal("sixth concurrent attempt was not blocked")
	}
	for i := 0; i < 5; i++ {
		cancelLoginAttempt(ip)
	}
}
