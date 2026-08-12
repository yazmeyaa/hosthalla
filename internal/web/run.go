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

	"github.com/yazmeyaa/hosthalla/internal/agent"
	"github.com/yazmeyaa/hosthalla/internal/api"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/config"
	appdatabase "github.com/yazmeyaa/hosthalla/internal/database"
	"github.com/yazmeyaa/hosthalla/internal/events"
	"github.com/yazmeyaa/hosthalla/internal/host"
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
	webOrigin, err := cfg.PublicWebOrigin()
	if err != nil {
		bootstrapLogger.Error("invalid config value", slog.String("field", "web_origin"), slog.String("error", err.Error()))
		return err
	}

	logger := app_logger.NewLogger(app_logger.LoggerParams{
		Output: os.Stdout,
		Level:  logLevel,
	})
	logger.Info("web logger configured", slog.String("log_level", cfg.LogLevel))

	store, err := appdatabase.Open(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		return err
	}
	defer store.Close()
	logger.Info("database connection pool initialized")

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

	authService := auth_service.New(auth_service.NewParams{
		ProfileRepository:                store.ProfileRepository,
		PasswordAuthenticationRepository: store.PasswordAuthenticationRepository,
		SessionRepository:                store.SessionRepository,
		APITokenRepository:               store.APITokenRepository,
	})
	hostService := host.NewService(host.NewServiceParams{
		HostRepository:                 store.HostRepository,
		HostManagementMethodRepository: store.HostManagementMethodRepository,
		HostSystemInfoRepository:       store.HostSystemInfoRepository,
		HostMetricSnapshotRepository:   store.HostMetricSnapshotRepository,
		SecretCipher:                   secretCipher,
		Logger:                         logger,
		EventBus:                       eventBus,
	})
	agentService := agent.NewService(agent.NewServiceParams{
		AgentRepository:       store.AgentRepository,
		AgentConfigRepository: store.AgentConfigRepository,
		EventBus:              eventBus,
		Logger:                logger,
	})
	router := NewRouter(NewRouterParams{
		HostService:       hostService,
		AgentService:      agentService,
		AuthService:       authService,
		SessionRepository: store.SessionRepository,
		Logger:            logger,
		EventBus:          eventBus,
		WebOrigin:         webOrigin,
	})
	apiRouter := api.NewRouter(
		api.RouterParams{
			AgentService:       agentService,
			HostService:        hostService,
			APITokenRepository: store.APITokenRepository,
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
