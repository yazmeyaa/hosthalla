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

type RouterParams struct {
	AgentService       *agent.Service
	HostService        *host.Service
	APITokenRepository authentication_storage.APITokenRepository
	Logger             *slog.Logger
}

func NewRouter(
	params RouterParams,
) http.Handler {
	mux := http.NewServeMux()
	hostsHandler := handlers.NewHostsHandler(handlers.HostHandlerParams{
		AgentService: params.AgentService,
		HostService:  params.HostService,
		Logger:       params.Logger,
	})
	agentsHandler := handlers.NewAgentsHandler(handlers.AgentsHandlerParams{
		AgentService: params.AgentService,
		HostService:  params.HostService,
		Logger:       params.Logger,
	})

	mux.Handle(
		"POST /hosts/{host_id}/register-agent",
		middlewares.APITokenAuthMiddleware(params.APITokenRepository, http.HandlerFunc(hostsHandler.RegisterAgent)),
	)
	mux.Handle(
		"POST /hosts/{host_id}/system-info",
		middlewares.APITokenAuthMiddleware(params.APITokenRepository, http.HandlerFunc(hostsHandler.UpsertHostSystemInfo)),
	)

	mux.Handle(
		"POST /heartbeat",
		middlewares.APITokenAuthMiddleware(params.APITokenRepository, http.HandlerFunc(agentsHandler.HandleHeartbeat)),
	)
	mux.Handle(
		"POST /metrics",
		middlewares.APITokenAuthMiddleware(params.APITokenRepository, http.HandlerFunc(agentsHandler.HandleMetrics)),
	)
	mux.Handle(
		"GET /config",
		middlewares.APITokenAuthMiddleware(params.APITokenRepository, http.HandlerFunc(agentsHandler.GetConfig)),
	)
	return web_middlewares.RequestLoggingMiddleware(params.Logger, mux)
}
