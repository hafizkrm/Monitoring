package repository

import (
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
	err := r.db.QueryRow("SELECT id, name, username, password_hash, role, created_at, updated_at FROM users WHERE username = ?", username).
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

func (r *UserRepository) Update(u *models.User) error {
	if u.PasswordHash != "" {
		_, err := r.db.Exec("UPDATE users SET name=?, username=?, password_hash=?, role=? WHERE id=?", u.Name, u.Username, u.PasswordHash, u.Role, u.ID)
		return err
	}
	_, err := r.db.Exec("UPDATE users SET name=?, username=?, role=? WHERE id=?", u.Name, u.Username, u.Role, u.ID)
	return err
}

func (r *UserRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM users WHERE id=?", id)
	return err
}
