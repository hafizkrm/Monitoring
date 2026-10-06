package settings

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hafizkrm/Monitoring/backend/internal/cache"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
	"github.com/hafizkrm/Monitoring/backend/internal/repository"
)

type ThresholdRulesHandler struct {
	repo  repository.ThresholdRepository
	cache cache.ThresholdRuleCache
}

func NewThresholdRulesHandler(repo repository.ThresholdRepository, ruleCache cache.ThresholdRuleCache) *ThresholdRulesHandler {
	return &ThresholdRulesHandler{repo: repo, cache: ruleCache}
}

func (h *ThresholdRulesHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	rules, err := h.repo.GetAllRules(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

func (h *ThresholdRulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var rule models.ThresholdRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := h.repo.CreateRule(r.Context(), &rule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.cache.Refresh(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

func (h *ThresholdRulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	var rule models.ThresholdRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateRule(r.Context(), &rule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.cache.Refresh(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

func (h *ThresholdRulesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.repo.DeleteRule(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.cache.Refresh(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
