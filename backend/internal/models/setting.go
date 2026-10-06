package models

import "time"

type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"` // Stored as JSON string in DB
	UpdatedAt time.Time `json:"updated_at"`
}

type SettingPayload struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}
