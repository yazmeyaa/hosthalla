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
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type AgentsHandler struct {
	agentRepository       agent.Repository
	agentConfigRepository agent.AgentConfigRepository
	hostMetricRepository  host.HostMetricSnapshotRepository
	logger                *slog.Logger
}

func NewAgentsHandler(
	agentRepository agent.Repository,
	agentConfigRepository agent.AgentConfigRepository,
	hostMetricRepository host.HostMetricSnapshotRepository,
	logger *slog.Logger,
) *AgentsHandler {
	return &AgentsHandler{
		agentRepository:       agentRepository,
		agentConfigRepository: agentConfigRepository,
		hostMetricRepository:  hostMetricRepository,
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
		h.logger.Error("failed to get agent", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.agentRepository.UpdateLastSeenAt(ctx, agentID, time.Now()); err != nil {
		h.logger.Error("failed to update last seen at", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	config, err := h.agentConfigRepository.GetByAgentID(ctx, agentID)
	if err != nil {
		h.logger.Error("failed to get agent config", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := HeartbeatResponse{
		Version: config.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *AgentsHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
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

	currentAgent, err := h.agentRepository.GetByID(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get agent", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var metric host.HostMetric
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	_, err = h.hostMetricRepository.CreateHostMetricSnapshot(ctx, host.HostMetricSnapshot{
		HostID:    host.HostID(currentAgent.HostID),
		Timestamp: time.Now().UTC(),
		Metrics:   []host.HostMetric{metric},
	})
	if err != nil {
		h.logger.Error("failed to save host metric snapshot", slog.String("error", err.Error()), slog.String("host_id", currentAgent.HostID.String()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
func (h *AgentsHandler) GetConfig(w http.ResponseWriter, r *http.Request) {}
