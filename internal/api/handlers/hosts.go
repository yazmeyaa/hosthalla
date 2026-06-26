package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	"github.com/yazmeyaa/hosthalla/internal/api/middlewares"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type HostsHandler struct {
	logger          *slog.Logger
	agentRepository agent.Repository
	hostRepository  host.HostRepository
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

func NewHostsHandler(agentRepository agent.Repository, hostRepository host.HostRepository, logger *slog.Logger) *HostsHandler {
	return &HostsHandler{
		agentRepository: agentRepository,
		hostRepository:  hostRepository,
		logger:          logger,
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

	hostIDStr := strings.TrimSpace(r.PathValue("host_id"))
	if hostIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "host_id is required")
		return
	}
	hostID, err := uuid.Parse(hostIDStr)

	if err != nil {
		h.logger.Error("failed to parse host id", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid host_id")
		return
	}

	_, err = h.hostRepository.GetHostByID(ctx, hostID)
	if err != nil {
		h.logger.Error("failed to get host by id", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to get host by id")
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

	agent, err := h.agentRepository.Create(ctx, agent.CreateAgentDTO{
		HostID:  hostID,
		Version: version,
	})

	if err != nil {
		h.logger.Error("failed to create agent", slog.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	response := AgentRegistrationSuccessResponse{
		AgentID: agent.ID,
		HostID:  hostID,
		Version: version,
	}
	if err := h.writeJSONResponse(w, http.StatusCreated, response); err != nil {
		h.logger.Error("failed to encode agent", slog.String("error", err.Error()))
	}
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
