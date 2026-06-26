package api

import (
	"log/slog"
	"net/http"

	"github.com/yazmeyaa/hosthalla/internal/agent"
	"github.com/yazmeyaa/hosthalla/internal/api/handlers"
	"github.com/yazmeyaa/hosthalla/internal/api/middlewares"
	authentication_storage "github.com/yazmeyaa/hosthalla/internal/authentication/storage"
	"github.com/yazmeyaa/hosthalla/internal/host"
	web_middlewares "github.com/yazmeyaa/hosthalla/internal/web/middlewares"
)

func NewRouter(
	agentRepository agent.Repository,
	agentConfigRepository agent.AgentConfigRepository,
	hostRepository host.HostRepository,
	apiTokenRepository authentication_storage.APITokenRepository,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	hostsHandler := handlers.NewHostsHandler(agentRepository, hostRepository, logger)
	agentsHandler := handlers.NewAgentsHandler(agentRepository, agentConfigRepository, logger)

	mux.Handle(
		"POST /hosts/{host_id}/register-agent",
		middlewares.APITokenAuthMiddleware(apiTokenRepository, http.HandlerFunc(hostsHandler.RegisterAgent)),
	)

	mux.Handle(
		"POST /api/v1/heartbeat",
		middlewares.APITokenAuthMiddleware(apiTokenRepository, http.HandlerFunc(agentsHandler.HandleHeartbeat)),
	)
	return web_middlewares.RequestLoggingMiddleware(logger, mux)
}
