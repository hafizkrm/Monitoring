package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var ErrSessionInvalid = errors.New("session invalid or expired")

type AuthSession struct {
	ID                string
	UserID            int
	Username          string
	Name              string
	Role              string
	RoleAtIssue       string
	ExpiresAt         time.Time
	AbsoluteExpiresAt sql.NullTime
}

type AuthSessionRepository struct {
	db *sql.DB
}

func NewAuthSessionRepository(db *sql.DB) *AuthSessionRepository {
	return &AuthSessionRepository{db: db}
}

func (r *AuthSessionRepository) Create(ctx context.Context, userID int, role, storedRole, passwordHash string, now time.Time) (AuthSession, string, error) {
	role = NormalizeRole(role)
	if role != "admin" && role != "viewer" {
		return AuthSession{}, "", ErrSessionInvalid
	}
	id, err := randomToken()
	if err != nil {
		return AuthSession{}, "", err
	}
	refreshToken, err := randomToken()
	if err != nil {
		return AuthSession{}, "", err
	}

	session := AuthSession{ID: id, UserID: userID, Role: role}
	session.ExpiresAt = now.Add(24 * time.Hour)
	var absolute any
	if role == "admin" {
		session.AbsoluteExpiresAt = sql.NullTime{Time: now.Add(12 * time.Hour), Valid: true}
		session.ExpiresAt = session.AbsoluteExpiresAt.Time
		absolute = session.AbsoluteExpiresAt.Time
	}

	session.RoleAtIssue = role
	if _, err := r.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at <= ? LIMIT 500`, now); err != nil {
		return AuthSession{}, "", fmt.Errorf("cleanup expired auth sessions: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO auth_sessions
		(id, user_id, role_at_issue, refresh_token_hash, expires_at, absolute_expires_at)
		SELECT ?, users.id, ?, ?, ?, ? FROM users
		WHERE users.id = ? AND users.role = ? AND users.password_hash = ? AND users.is_active = 1`,
		id, role, hashToken(refreshToken), session.ExpiresAt, absolute,
		userID, storedRole, passwordHash)
	if err != nil {
		return AuthSession{}, "", fmt.Errorf("create auth session: %w", err)
	}
	if created, err := result.RowsAffected(); err != nil || created != 1 {
		if err != nil {
			return AuthSession{}, "", err
		}
		return AuthSession{}, "", ErrSessionInvalid
	}
	return session, refreshToken, nil
}

func (r *AuthSessionRepository) GetByID(ctx context.Context, id string, now time.Time) (AuthSession, error) {
	var session AuthSession
	err := r.db.QueryRowContext(ctx, `SELECT s.id, s.user_id, u.username, u.name, u.role, s.role_at_issue, s.expires_at, s.absolute_expires_at
		FROM auth_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.is_active = 1`, id, now).
		Scan(&session.ID, &session.UserID, &session.Username, &session.Name, &session.Role, &session.RoleAtIssue, &session.ExpiresAt, &session.AbsoluteExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthSession{}, ErrSessionInvalid
	}
	if err == nil && NormalizeRole(session.Role) != NormalizeRole(session.RoleAtIssue) {
		return AuthSession{}, ErrSessionInvalid
	}
	return session, err
}

func (r *AuthSessionRepository) Refresh(ctx context.Context, refreshToken string, now time.Time) (AuthSession, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthSession{}, "", err
	}
	defer tx.Rollback()

	var session AuthSession
	err = tx.QueryRowContext(ctx, `SELECT s.id, s.user_id, u.username, u.name, u.role, s.role_at_issue, s.expires_at, s.absolute_expires_at
		FROM auth_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.refresh_token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND u.is_active = 1
		FOR UPDATE`, hashToken(refreshToken), now).
		Scan(&session.ID, &session.UserID, &session.Username, &session.Name, &session.Role, &session.RoleAtIssue, &session.ExpiresAt, &session.AbsoluteExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthSession{}, "", ErrSessionInvalid
	}
	if err != nil {
		return AuthSession{}, "", err
	}

	session.Role = NormalizeRole(session.Role)
	session.RoleAtIssue = NormalizeRole(session.RoleAtIssue)
	if session.Role != session.RoleAtIssue || (session.Role != "admin" && session.Role != "viewer") {
		return AuthSession{}, "", ErrSessionInvalid
	}
	if session.AbsoluteExpiresAt.Valid && !now.Before(session.AbsoluteExpiresAt.Time) {
		return AuthSession{}, "", ErrSessionInvalid
	}

	newExpiry := now.Add(24 * time.Hour)
	if session.AbsoluteExpiresAt.Valid {
		newExpiry = session.AbsoluteExpiresAt.Time
	}
	_, err = tx.ExecContext(ctx, `UPDATE auth_sessions SET expires_at = ? WHERE id = ?`, newExpiry, session.ID)
	if err != nil {
		return AuthSession{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return AuthSession{}, "", err
	}
	session.ExpiresAt = newExpiry
	return session, refreshToken, nil
}

func (r *AuthSessionRepository) RevokeByRefreshToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at = CURRENT_TIMESTAMP
		WHERE refresh_token_hash = ? AND revoked_at IS NULL`, hashToken(token))
	return err
}

func (r *AuthSessionRepository) RevokeByID(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revoked_at IS NULL`, id)
	return err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func NormalizeRole(role string) string {
	if role == "view" {
		return "viewer"
	}
	return role
}
