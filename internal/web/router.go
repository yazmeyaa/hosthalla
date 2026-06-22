package web

import (
	"net/http"

	"github.com/yazmeyaa/hosthalla/internal/host/storage/postgres"
	"github.com/yazmeyaa/hosthalla/internal/web/handlers"
)

type NewRouterParams struct {
	HostRepository *postgres.HostRepositoryPostgresImpl
}

func NewRouter(params NewRouterParams) *http.ServeMux {
	indexHandler := handlers.NewIndexHandler(params.HostRepository)
	authHandler := handlers.NewAuthHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler.Index)
	mux.HandleFunc("/auth", authHandler.Auth)

	return mux
}
