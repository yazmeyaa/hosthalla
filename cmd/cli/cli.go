package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	auth_repository "github.com/yazmeyaa/hosthalla/internal/authentication/storage"
	auth_storage "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	app_cli "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
	app_logger "github.com/yazmeyaa/hosthalla/internal/logger"
)

func main() {
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

	args := flags.Args()
	if len(args) == 0 {
		bootstrapLogger.Error("no arguments provided")
		os.Exit(1)
	}

	command := args[0]
	if command == "config" {
		app_cli.ProcessConfigCommand(args[1:])
		return
	}
	if command == "agent" {
		app_cli.ProcessAgentCommand(args[1:])
		return
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
	logger.Info("cli logger configured", slog.String("log_level", cfg.LogLevel))

	pool, err := pgxpool.New(context.Background(), cfg.Database.ConnectionString())
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("database connection pool initialized")

	logger.Info("running cli command", slog.String("command", command))
	switch command {
	case "create-user":
		createUser(logger, pool, args[1:])
	default:
		logger.Error("unknown command", slog.String("command", command))
		os.Exit(1)
	}
}

func createUser(logger *slog.Logger, pool *pgxpool.Pool, args []string) {
	if len(args) != 2 {
		logger.Error("invalid arguments", slog.String("command", "create-user"), slog.String("usage", "create-user <username> <password>"))
		os.Exit(1)
	}

	var profileRepo auth_repository.ProfileRepository = auth_storage.NewProfileRepository(pool)
	var passwordRepo auth_repository.PasswordAuthenticationRepository = auth_storage.NewPasswordAuthenticationRepository(pool)
	var sessionRepo auth_repository.SessionRepository = auth_storage.NewSessionRepository(pool)
	var apiTokenRepo auth_repository.APITokenRepository = auth_storage.NewAPITokenRepository(pool)
	svc := auth_service.New(auth_service.NewParams{
		ProfileRepository:                profileRepo,
		PasswordAuthenticationRepository: passwordRepo,
		SessionRepository:                sessionRepo,
		APITokenRepository:               apiTokenRepo,
	})
	username := args[0]
	password := args[1]

	user, err := svc.CreateUser(context.Background(), auth_service.CreateUserDTO{
		Username: username,
		Password: password,
	})
	if err != nil {
		logger.Error("failed to create user", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("user created",
		slog.String("user_id", user.ID),
		slog.String("username", user.Username),
		slog.String("created_at", user.CreatedAt.Format(time.RFC3339)),
		slog.String("updated_at", user.UpdatedAt.Format(time.RFC3339)),
	)
}
