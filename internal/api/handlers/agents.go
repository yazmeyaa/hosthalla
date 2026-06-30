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
	agentService *agent.Service
	hostService  *host.Service
	logger       *slog.Logger
}

type AgentsHandlerParams struct {
	AgentService *agent.Service
	HostService  *host.Service
	Logger       *slog.Logger
}

func NewAgentsHandler(
	params AgentsHandlerParams,
) *AgentsHandler {
	return &AgentsHandler{
		agentService: params.AgentService,
		hostService:  params.HostService,
		logger:       params.Logger,
	}
}

type HeartbeatResponse struct {
	Version int `json:"version"`
}

type ConfigResponse struct {
	Version                  int `json:"version"`
	HeartbeatIntervalSeconds int `json:"heartbeatIntervalSeconds"`
	MetricsIntervalSeconds   int `json:"metricsIntervalSeconds"`
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

	currentAgent, err := h.agentService.GetByID(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get agent", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.agentService.UpdateLastSeenAt(ctx, currentAgent, time.Now()); err != nil {
		h.logger.Error("failed to update last seen at", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	config, err := h.agentService.GetConfigByAgentID(ctx, agentID)
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

	currentAgent, err := h.agentService.GetByID(ctx, agentID)
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

	_, err = h.hostService.CreateHostMetricSnapshot(ctx, host.HostMetricSnapshot{
		HostID:    currentAgent.HostID,
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

func (h *AgentsHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.agentService.GetByID(ctx, agentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get agent", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	config, err := h.agentService.GetConfigByAgentID(ctx, agentID)
	if err != nil {
		h.logger.Error("failed to get agent config", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := ConfigResponse{
		Version:                  config.Version,
		HeartbeatIntervalSeconds: int(config.Heartbeat.Interval / time.Second),
		MetricsIntervalSeconds:   int(config.Metrics.Interval / time.Second),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}
