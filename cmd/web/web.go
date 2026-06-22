package main

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host/storage/postgres"
	"github.com/yazmeyaa/hosthalla/internal/web"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://hosthalla:hosthalla@localhost:5432/hosthalla?sslmode=disable")
	defer pool.Close()

	if err != nil {
		panic(err)
	}

	repo := postgres.NewHostRepository(pool)

	if err != nil {
		panic(err)
	}

	router := web.NewRouter(web.NewRouterParams{
		HostRepository: repo,
	})
	http.ListenAndServe(":8080", router)
}
