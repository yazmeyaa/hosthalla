package web

import (
	"log/slog"
	"net/http"

	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	authentication_repository "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	host_repository "github.com/yazmeyaa/hosthalla/internal/host/storage/postgres"
	"github.com/yazmeyaa/hosthalla/internal/web/handlers"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
)

type NewRouterParams struct {
	HostRepository    *host_repository.HostRepositoryPostgresImpl
	SessionRepository *authentication_repository.SessionRepositoryPostgresImpl
	AuthService       *auth_service.Service
	Logger            *slog.Logger
}

func NewRouter(params NewRouterParams) *http.ServeMux {
	indexHandler := handlers.NewIndexHandler(params.HostRepository, params.Logger, params.AuthService)
	authHandler := handlers.NewAuthHandler(params.Logger, params.AuthService)
	hostHandler := handlers.NewHostsHandler(params.HostRepository, params.AuthService)

	mux := http.NewServeMux()
	mux.Handle("GET /", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(indexHandler.Index)))
	mux.HandleFunc("GET /auth", authHandler.Auth)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.Handle("GET /hosts", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.ListHosts)))

	return mux
}
