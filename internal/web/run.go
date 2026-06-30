package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	agent_repository "github.com/yazmeyaa/hosthalla/internal/agent/postgres"
	"github.com/yazmeyaa/hosthalla/internal/api"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	authentication_repository "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	"github.com/yazmeyaa/hosthalla/internal/config"
	"github.com/yazmeyaa/hosthalla/internal/events"
	"github.com/yazmeyaa/hosthalla/internal/host"
	host_repository "github.com/yazmeyaa/hosthalla/internal/host/postgres"
	app_logger "github.com/yazmeyaa/hosthalla/internal/logger"
	"github.com/yazmeyaa/hosthalla/internal/version"
)

type RunParams struct {
	ConfigPath string
}

func Run(ctx context.Context, params RunParams) error {
	eventBus := events.NewInMemoryEventBus()

	bootstrapLogger := app_logger.NewLogger(app_logger.LoggerParams{
		Output: os.Stdout,
		Level:  slog.LevelWarn,
	})

	configPath := params.ConfigPath
	if configPath == "" {
		configPath = config.DefaultConfigPath
	}

	cfg := config.AppConfig{}
	if err := cfg.LoadFromPath(configPath); err != nil {
		bootstrapLogger.Error("failed to load config", slog.String("path", configPath), slog.String("error", err.Error()))
		return err
	}

	logLevel, err := cfg.SlogLevel()
	if err != nil {
		bootstrapLogger.Error("invalid config value", slog.String("field", "log_level"), slog.String("error", err.Error()))
		return err
	}

	logger := app_logger.NewLogger(app_logger.LoggerParams{
		Output: os.Stdout,
		Level:  logLevel,
	})
	logger.Info("web logger configured", slog.String("log_level", cfg.LogLevel))

	pool, err := pgxpool.New(ctx, cfg.Database.ConnectionString())
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		return err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		return err
	}
	defer pool.Close()
	logger.Info("database connection pool initialized")

	hostRepositories := host_repository.NewRepositories(pool)
	secretEncryptionKey, err := cfg.SecretEncryptionKey()
	if err != nil {
		logger.Error("invalid secret encryption key", slog.String("error", err.Error()))
		return err
	}
	secretCipher, err := host.NewAESGCMSecretCipher(secretEncryptionKey)
	if err != nil {
		logger.Error("failed to initialize secret cipher", slog.String("error", err.Error()))
		return err
	}

	sessionRepository := authentication_repository.NewSessionRepository(pool)
	apiTokenRepository := authentication_repository.NewAPITokenRepository(pool)
	profileRepository := authentication_repository.NewProfileRepository(pool)
	passwordAuthenticationRepository := authentication_repository.NewPasswordAuthenticationRepository(pool)
	agentConfigRepository := agent_repository.NewAgentConfigRepository(pool)
	agentRepository := agent_repository.NewAgentRepository(pool)

	authService := auth_service.New(auth_service.NewParams{
		ProfileRepository:                profileRepository,
		PasswordAuthenticationRepository: passwordAuthenticationRepository,
		SessionRepository:                sessionRepository,
		APITokenRepository:               apiTokenRepository,
	})
	hostService := host.NewService(host.NewServiceParams{
		HostRepository:                 hostRepositories.Host,
		HostManagementMethodRepository: hostRepositories.HostManagementMethod,
		HostSystemInfoRepository:       hostRepositories.HostSystemInfo,
		HostMetricSnapshotRepository:   hostRepositories.HostMetricSnapshot,
		SecretCipher:                   secretCipher,
		Logger:                         logger,
		EventBus:                       eventBus,
	})
	agentService := agent.NewService(agent.NewServiceParams{
		AgentRepository:       agentRepository,
		AgentConfigRepository: agentConfigRepository,
		EventBus:              eventBus,
		Logger:                logger,
	})
	router := NewRouter(NewRouterParams{
		HostService:       hostService,
		AuthService:       authService,
		SessionRepository: sessionRepository,
		Logger:            logger,
		EventBus:          eventBus,
	})
	apiRouter := api.NewRouter(
		api.RouterParams{
			AgentService:       agentService,
			HostService:        hostService,
			APITokenRepository: apiTokenRepository,
			Logger:             logger,
		},
	)
	rootRouter := http.NewServeMux()
	rootRouter.Handle("/api/v1/", http.StripPrefix("/api/v1", apiRouter))
	rootRouter.Handle("/", router)
	listenAddress := cfg.WEB.ListenAddress()
	server := &http.Server{
		Addr:    listenAddress,
		Handler: rootRouter,
	}
	logger.Info(
		"starting web server",
		slog.String("listen_address", listenAddress),
		slog.String("version", version.VersionString()),
	)

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	shutdownSignalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("web server stopped unexpectedly", slog.String("error", err.Error()))
			return err
		}
	case <-shutdownSignalCtx.Done():
		logger.Info("shutdown signal received, shutting down web server")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to gracefully shut down web server", slog.String("error", err.Error()))
		return err
	}

	logger.Info("web server stopped gracefully")
	return nil
}
