package web

import (
	"log/slog"
	"net/http"

	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	authentication_repository "github.com/yazmeyaa/hosthalla/internal/authentication/storage"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/web/handlers"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	ui_assets "github.com/yazmeyaa/hosthalla/ui/assets"
)

type NewRouterParams struct {
	HostRepository                 host.HostRepository
	HostManagementMethodRepository host.HostManagementMethodRepository
	HostSystemInfoRepository       host.HostSystemInfoRepository
	HostMetricSnapshotRepository   host.HostMetricSnapshotRepository
	SessionRepository              authentication_repository.SessionRepository
	AuthService                    *auth_service.Service
	Logger                         *slog.Logger
}

func NewRouter(params NewRouterParams) http.Handler {
	hostService := host.New(
		params.HostRepository,
		params.HostManagementMethodRepository,
		params.HostSystemInfoRepository,
		params.HostMetricSnapshotRepository,
		params.Logger,
	)
	indexHandler := handlers.NewIndexHandler(params.HostRepository, params.Logger, params.AuthService)
	authHandler := handlers.NewAuthHandler(params.Logger, params.AuthService)
	hostHandler := handlers.NewHostsHandler(hostService, params.AuthService, params.Logger)
	administrationHandler := handlers.NewAdministrationHandler(params.AuthService, params.Logger)

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(ui_assets.Files))))
	mux.Handle("GET /", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(indexHandler.Index)))
	mux.HandleFunc("GET /auth", authHandler.Auth)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.Handle("GET /hosts", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.ListHosts)))
	mux.Handle("POST /hosts/create", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.CreateHost)))
	mux.Handle("POST /hosts/{id}/update", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.UpdateHost)))
	mux.Handle("POST /hosts/{id}/delete", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.DeleteHost)))
	mux.Handle("POST /hosts/{id}/ping", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.PingHost)))
	mux.Handle("POST /hosts/{id}/management-methods/create", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.CreateHostManagementMethod)))
	mux.Handle("POST /hosts/{id}/agent/register-command", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.CreateAgentRegisterCommand)))
	mux.Handle("POST /hosts/ping-all", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.PingAllHosts)))
	mux.Handle("GET /administration", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.Administration)))
	mux.Handle("POST /administration/api-tokens/create", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.CreateAPIToken)))
	mux.Handle("POST /administration/api-tokens/{id}/revoke", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.RevokeAPIToken)))

	return middlewares.RequestLoggingMiddleware(params.Logger, mux)
}
