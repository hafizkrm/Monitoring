package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

func TestUpdateRoleRevokesUserSessionsAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT username, role, password_hash FROM users WHERE id = \\? FOR UPDATE").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"username", "role", "password_hash"}).AddRow("operator", "admin", "old-hash"))
	mock.ExpectExec("UPDATE users SET name=\\?, username=\\?, role=\\? WHERE id=\\?").
		WithArgs("Operator", "operator", "viewer", 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE auth_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE user_id = \\? AND revoked_at IS NULL").
		WithArgs(2).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	err = NewUserRepository(db).UpdateAndRevokeSessions(context.Background(), &models.User{
		ID: 2, Name: "Operator", Username: "operator", Role: "viewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
