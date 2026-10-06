package repository

import (
	"database/sql"
	"github.com/yourusername/viscod/internal/models"
)

type SettingRepository struct {
	db *sql.DB
}

func NewSettingRepository(db *sql.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) GetAll() ([]*models.Setting, error) {
	rows, err := r.db.Query("SELECT setting_key, setting_value, updated_at FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []*models.Setting
	for rows.Next() {
		s := &models.Setting{}
		if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, nil
}

func (r *SettingRepository) GetByKey(key string) (*models.Setting, error) {
	s := &models.Setting{}
	err := r.db.QueryRow("SELECT setting_key, setting_value, updated_at FROM settings WHERE setting_key = ?", key).
		Scan(&s.Key, &s.Value, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SettingRepository) Upsert(s *models.Setting) error {
	_, err := r.db.Exec("INSERT INTO settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_value=?", s.Key, s.Value, s.Value)
	return err
}
