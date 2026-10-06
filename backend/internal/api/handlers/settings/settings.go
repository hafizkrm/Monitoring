package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/yourusername/viscod/internal/models"
	"github.com/yourusername/viscod/internal/repository"
)

type SettingsHandler struct {
	repo *repository.SettingRepository
}

func NewSettingsHandler(db *sql.DB) *SettingsHandler {
	return &SettingsHandler{repo: repository.NewSettingRepository(db)}
}

func (h *SettingsHandler) GetAllSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.repo.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Format as key: value (parsed JSON) map
	result := make(map[string]interface{})
	for _, s := range settings {
		var val interface{}
		json.Unmarshal([]byte(s.Value), &val)
		result[s.Key] = val
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   result,
	})
}

func (h *SettingsHandler) UpsertSetting(w http.ResponseWriter, r *http.Request) {
	var payload models.SettingPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	valBytes, _ := json.Marshal(payload.Value)

	setting := &models.Setting{
		Key:   payload.Key,
		Value: string(valBytes),
	}
	if err := h.repo.Upsert(setting); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
}
