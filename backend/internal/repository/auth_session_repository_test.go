package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateAdminSessionHasTwelveHourAbsoluteExpiry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM auth_sessions WHERE expires_at <= ? LIMIT 500")).
		WithArgs(now).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO auth_sessions")).
		WithArgs(sqlmock.AnyArg(), "admin", sqlmock.AnyArg(), now.Add(12*time.Hour), now.Add(12*time.Hour), 7, "admin", "bcrypt-hash").
		WillReturnResult(sqlmock.NewResult(1, 1))

	session, refresh, err := NewAuthSessionRepository(db).Create(context.Background(), 7, "admin", "admin", "bcrypt-hash", now)
	if err != nil {
		t.Fatal(err)
	}
	if session.ExpiresAt != now.Add(12*time.Hour) || !session.AbsoluteExpiresAt.Valid {
		t.Fatalf("admin session expiry = %v; want absolute expiry at %v", session.ExpiresAt, now.Add(12*time.Hour))
	}
	if len(refresh) != 64 {
		t.Fatalf("refresh token length = %d; want 64", len(refresh))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshViewerSessionSlidesExpiryWithoutChangingCookie(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id, s.user_id, u.username, u.name, u.role, s.role_at_issue, s.expires_at, s.absolute_expires_at")).
		WithArgs(hashToken("old-refresh"), now).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "username", "name", "role", "role_at_issue", "expires_at", "absolute_expires_at"}).
			AddRow("session-id", 8, "viewer-user", "Viewer User", "viewer", "viewer", now.Add(time.Hour), nil))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE auth_sessions SET expires_at = ? WHERE id = ?")).
		WithArgs(now.Add(24*time.Hour), "session-id").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	session, returnedRefresh, err := NewAuthSessionRepository(db).Refresh(context.Background(), "old-refresh", now)
	if err != nil {
		t.Fatal(err)
	}
	if session.ExpiresAt != now.Add(24*time.Hour) || session.AbsoluteExpiresAt.Valid {
		t.Fatalf("viewer expiry = %v; absolute expiry valid = %v", session.ExpiresAt, session.AbsoluteExpiresAt.Valid)
	}
	if returnedRefresh != "old-refresh" {
		t.Fatal("refresh cookie changed during a retryable session refresh")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeRoleSupportsLegacyViewerRole(t *testing.T) {
	if got := NormalizeRole("view"); got != "viewer" {
		t.Fatalf("NormalizeRole(view) = %q; want viewer", got)
	}
}

func TestGetByIDRejectsSessionAfterRoleChange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT s.id, s.user_id, u.username, u.name, u.role, s.role_at_issue, s.expires_at, s.absolute_expires_at")).
		WithArgs("session-id", now).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "username", "name", "role", "role_at_issue", "expires_at", "absolute_expires_at"}).
			AddRow("session-id", 8, "user", "User", "admin", "viewer", now.Add(time.Hour), nil))
	_, err = NewAuthSessionRepository(db).GetByID(context.Background(), "session-id", now)
	if err != ErrSessionInvalid {
		t.Fatalf("role-changed session error = %v; want ErrSessionInvalid", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
