package repository

import (
	"context"
	"database/sql"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetAll() ([]*models.User, error) {
	rows, err := r.db.Query("SELECT id, name, username, password_hash, role, created_at, updated_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.Name, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow("SELECT id, name, username, password_hash, role, created_at, updated_at FROM users WHERE username = ? AND is_active = 1", username).
		Scan(&u.ID, &u.Name, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) Create(u *models.User) error {
	res, err := r.db.Exec("INSERT INTO users (name, username, password_hash, role) VALUES (?, ?, ?, ?)", u.Name, u.Username, u.PasswordHash, u.Role)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	u.ID = int(id)
	return nil
}

func (r *UserRepository) UpdateAndRevokeSessions(ctx context.Context, u *models.User) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var oldUsername, oldRole, oldPasswordHash string
	if err := tx.QueryRowContext(ctx, `SELECT username, role, password_hash FROM users WHERE id = ? FOR UPDATE`, u.ID).
		Scan(&oldUsername, &oldRole, &oldPasswordHash); err != nil {
		return err
	}

	if u.PasswordHash != "" {
		_, err = tx.ExecContext(ctx, `UPDATE users SET name=?, username=?, password_hash=?, role=? WHERE id=?`,
			u.Name, u.Username, u.PasswordHash, u.Role, u.ID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE users SET name=?, username=?, role=? WHERE id=?`,
			u.Name, u.Username, u.Role, u.ID)
	}
	if err != nil {
		return err
	}
	credentialsChanged := oldUsername != u.Username || oldRole != u.Role || (u.PasswordHash != "" && oldPasswordHash != u.PasswordHash)
	if credentialsChanged {
		if _, err := tx.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE user_id = ? AND revoked_at IS NULL`, u.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *UserRepository) DeleteAndRevokeSessions(ctx context.Context, id int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE user_id = ? AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}
