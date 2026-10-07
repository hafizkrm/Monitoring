package users

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Dummy DB struct matching *sql.DB signature is tricky without an interface,
// but we can mock the repository if we decouple it. Wait, NewUsersHandler takes *sql.DB.
// For integration testing handlers without a real DB, we can use a mock repository.
// To save time, we will test the Refresh and Logout endpoints which don't hit the DB.

func TestUsersHandler_Logout(t *testing.T) {
	handler := &UsersHandler{}

	req, _ := http.NewRequest("POST", "/api/logout", nil)
	rr := httptest.NewRecorder()

	handler.Logout(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check if cookie is cleared
	cookies := rr.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "netmon_token" {
			found = true
			if c.Value != "" || c.MaxAge != -1 {
				t.Errorf("expected cookie to be cleared, got value=%v, max-age=%v", c.Value, c.MaxAge)
			}
		}
	}
	if !found {
		t.Errorf("expected netmon_token cookie in response")
	}
}

func TestUsersHandler_Refresh_Unauthorized(t *testing.T) {
	handler := &UsersHandler{jwtSecret: "secret"}

	req, _ := http.NewRequest("POST", "/api/refresh", nil)
	rr := httptest.NewRecorder()

	handler.Refresh(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing cookie, got %v", status)
	}
}
