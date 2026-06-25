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
	host_repository "github.com/yazmeyaa/hosthalla/internal/host/postgres"
	app_logger "github.com/yazmeyaa/hosthalla/internal/logger"
	"github.com/yazmeyaa/hosthalla/internal/web"
)

func main() {
	ctx := context.Background()

	bootstrapLogger := app_logger.NewLogger(app_logger.LoggerParams{
		Output: os.Stdout,
		Level:  slog.LevelWarn,
	})

	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configPath := flags.String("config", config.DefaultConfigPath, "path to config file")
	if err := flags.Parse(os.Args[1:]); err != nil {
		bootstrapLogger.Error("failed to parse command flags", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg := config.AppConfig{}
	if err := cfg.LoadFromPath(*configPath); err != nil {
		bootstrapLogger.Error("failed to load config", slog.String("path", *configPath), slog.String("error", err.Error()))
		os.Exit(1)
	}

	logLevel, err := cfg.SlogLevel()
	if err != nil {
		bootstrapLogger.Error("invalid config value", slog.String("field", "log_level"), slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger := app_logger.NewLogger(app_logger.LoggerParams{
		Output: os.Stdout,
		Level:  logLevel,
	})
	logger.Info("web logger configured", slog.String("log_level", cfg.LogLevel))

	pool, err := pgxpool.New(ctx, cfg.Database.ConnectionString())

	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("database connection pool initialized")

	hostRepositories := host_repository.NewRepositories(pool)
	authService := auth_service.New(auth_service.NewParams{
		ProfileRepository:                authentication_repository.NewProfileRepository(pool),
		PasswordAuthenticationRepository: authentication_repository.NewPasswordAuthenticationRepository(pool),
		SessionRepository:                authentication_repository.NewSessionRepository(pool),
		APITokenRepository:               authentication_repository.NewAPITokenRepository(pool),
	})
	router := web.NewRouter(web.NewRouterParams{
		HostRepository:                 hostRepositories.Host,
		HostManagementMethodRepository: hostRepositories.HostManagementMethod,
		AuthService:                    authService,
		SessionRepository:              authentication_repository.NewSessionRepository(pool),
		Logger:                         logger,
	})
	listenAddress := cfg.WEB.ListenAddress()
	logger.Info("starting web server", slog.String("listen_address", listenAddress))
	if err := http.ListenAndServe(listenAddress, router); err != nil {
		logger.Error("failed to start web server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
