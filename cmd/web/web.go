package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	authentication_repository "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	"github.com/yazmeyaa/hosthalla/internal/config"
	host_repository "github.com/yazmeyaa/hosthalla/internal/host/storage/postgres"
	"github.com/yazmeyaa/hosthalla/internal/logger"
	"github.com/yazmeyaa/hosthalla/internal/web"
)

func main() {
	ctx := context.Background()

	logger := logger.NewLogger(logger.LoggerParams{
		Output: os.Stdout,
		Level:  slog.LevelInfo,
	})

	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configPath := flags.String("config", config.DefaultConfigPath, "path to config file")
	if err := flags.Parse(os.Args[1:]); err != nil {
		logger.Error("failed to parse command flags", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg := config.AppConfig{}
	if err := cfg.LoadFromPath(*configPath); err != nil {
		logger.Error("failed to load config", slog.String("path", *configPath), slog.String("error", err.Error()))
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, cfg.Database.ConnectionString())

	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	repo := host_repository.NewHostRepository(pool)
	hostManagementMethodRepository := host_repository.NewHostManagementMethodRepository(pool)
	authService := auth_service.New(auth_service.NewParams{
		ProfileRepository:                authentication_repository.NewProfileRepository(pool),
		PasswordAuthenticationRepository: authentication_repository.NewPasswordAuthenticationRepository(pool),
		SessionRepository:                authentication_repository.NewSessionRepository(pool),
		APITokenRepository:               authentication_repository.NewAPITokenRepository(pool),
	})
	router := web.NewRouter(web.NewRouterParams{
		HostRepository:                 repo,
		HostManagementMethodRepository: hostManagementMethodRepository,
		AuthService:                    authService,
		SessionRepository:              authentication_repository.NewSessionRepository(pool),
		Logger:                         logger,
	})
	if err := http.ListenAndServe(cfg.WEB.ListenAddress(), router); err != nil {
		logger.Error("failed to start web server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
