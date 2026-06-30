package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	"github.com/yazmeyaa/hosthalla/internal/api/middlewares"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type HostsHandler struct {
	logger       *slog.Logger
	agentService *agent.Service
	hostService  *host.Service
}

type HTTPErrorResponse struct {
	Error string `json:"error"`
}

type AgentRegistrationSuccessResponse struct {
	AgentID uuid.UUID `json:"agentID"`
	HostID  uuid.UUID `json:"hostID"`
	Version string    `json:"version"`
}

type RegisterAgentRequest struct {
	Version string `json:"version"`
}

type HostHandlerParams struct {
	AgentService *agent.Service
	HostService  *host.Service
	Logger       *slog.Logger
}

func NewHostsHandler(
	params HostHandlerParams,
) *HostsHandler {
	return &HostsHandler{
		agentService: params.AgentService,
		hostService:  params.HostService,
		logger:       params.Logger,
	}
}

func (h *HostsHandler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiToken, err := middlewares.GetAPITokenFromContext(ctx)
	if err != nil {
		h.logger.Error("failed to get api token from context", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasScope(apiToken.Scopes, "hosts:register") {
		h.writeErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}

	hostID, err := h.parseAndEnsureHostExists(ctx, w, r)
	if err != nil {
		return
	}

	var request RegisterAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Error("failed to decode register agent payload", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	version := strings.TrimSpace(request.Version)
	if version == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "version is required")
		return
	}

	currentAgent, err := h.agentService.RegisterHostAgent(ctx, hostID, version)
	if err != nil {
		h.logger.Error("failed to register host agent", slog.String("error", err.Error()), slog.String("host_id", hostID.String()))
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to resolve host agent")
		return
	}

	if err := h.hostService.AssignMonitoringAgent(ctx, hostID, currentAgent.ID); err != nil {
		h.logger.Error("failed to persist host monitoring agent", slog.String("error", err.Error()), slog.String("host_id", hostID.String()), slog.String("agent_id", currentAgent.ID.String()))
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to update host monitoring agent")
		return
	}

	response := AgentRegistrationSuccessResponse{
		AgentID: currentAgent.ID,
		HostID:  hostID,
		Version: version,
	}
	if err := h.writeJSONResponse(w, http.StatusCreated, response); err != nil {
		h.logger.Error("failed to encode agent", slog.String("error", err.Error()))
	}
}

func (h *HostsHandler) UpsertHostSystemInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiToken, err := middlewares.GetAPITokenFromContext(ctx)
	if err != nil {
		h.logger.Error("failed to get api token from context", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !hasScope(apiToken.Scopes, "hosts:register") {
		h.writeErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}

	hostID, err := h.parseAndEnsureHostExists(ctx, w, r)
	if err != nil {
		return
	}

	var request host.HostSystemInfo
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Error("failed to decode host system info payload", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	request.HostID = hostID
	if _, err := h.hostService.UpsertHostSystemInfo(ctx, request); err != nil {
		h.logger.Error("failed to upsert host system info", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to upsert host system info")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *HostsHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorMessage string) {
	if err := h.writeJSONResponse(w, statusCode, HTTPErrorResponse{Error: errorMessage}); err != nil {
		h.logger.Error("failed to encode error response", slog.String("error", err.Error()))
	}
}

func (h *HostsHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(payload)
}

func hasScope(scopes []string, requiredScope string) bool {
	normalizedRequiredScope := strings.ToLower(strings.TrimSpace(requiredScope))
	if normalizedRequiredScope == "" {
		return false
	}
	for _, scope := range scopes {
		if strings.ToLower(strings.TrimSpace(scope)) == normalizedRequiredScope {
			return true
		}
	}
	return false
}

func (h *HostsHandler) parseAndEnsureHostExists(ctx context.Context, w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	hostIDStr := strings.TrimSpace(r.PathValue("host_id"))
	if hostIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "host_id is required")
		return uuid.UUID{}, errors.New("host_id is required")
	}
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		h.logger.Error("failed to parse host id", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid host_id")
		return uuid.UUID{}, err
	}

	if _, err := h.hostService.GetHostByID(ctx, hostID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.writeErrorResponse(w, http.StatusNotFound, "host not found")
			return uuid.UUID{}, err
		}
		h.logger.Error("failed to get host by id", slog.String("error", err.Error()), slog.String("host_id", hostID.String()))
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to get host by id")
		return uuid.UUID{}, err
	}

	return hostID, nil
}
