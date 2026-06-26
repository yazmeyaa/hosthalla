package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/yazmeyaa/hosthalla/internal/agent"
)

type AgentsHandler struct {
	agentRepository       agent.Repository
	agentConfigRepository agent.AgentConfigRepository
	logger                *slog.Logger
}

func NewAgentsHandler(agentRepository agent.Repository, agentConfigRepository agent.AgentConfigRepository, logger *slog.Logger) *AgentsHandler {
	return &AgentsHandler{
		agentRepository:       agentRepository,
		agentConfigRepository: agentConfigRepository,
		logger:                logger,
	}
}

type HeartbeatResponse struct {
	Version int `json:"version"`
}

func (h *AgentsHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentIDStr := r.Header.Get("Hosthalla-Agent-ID")
	if agentIDStr == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}

	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		http.Error(w, "failed to parse agent_id", http.StatusBadRequest)
		return
	}

	_, err = h.agentRepository.GetByID(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get agent", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.agentRepository.UpdateLastSeenAt(ctx, agentID, time.Now()); err != nil {
		h.logger.Error("failed to update last seen at", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	config, err := h.agentConfigRepository.GetByAgentID(ctx, agentID)
	if err != nil {
		h.logger.Error("failed to get agent config", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := HeartbeatResponse{
		Version: config.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode response", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *AgentsHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {}
func (h *AgentsHandler) GetConfig(w http.ResponseWriter, r *http.Request)     {}
