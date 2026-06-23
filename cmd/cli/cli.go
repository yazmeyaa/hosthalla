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
	"github.com/yazmeyaa/hosthalla/internal/config"
	"github.com/yazmeyaa/hosthalla/internal/logger"
)

func main() {
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

	args := flags.Args()
	if len(args) == 0 {
		logger.Error("no arguments provided")
		os.Exit(1)
	}

	cfg := config.AppConfig{}
	if err := cfg.LoadFromPath(*configPath); err != nil {
		logger.Error("failed to load config", slog.String("path", *configPath), slog.String("error", err.Error()))
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), cfg.Database.ConnectionString())
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	command := args[0]
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
	svc := auth_service.New(profileRepo, passwordRepo, sessionRepo)
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
		slog.String("password", password),
		slog.String("created_at", user.CreatedAt.Format(time.RFC3339)),
		slog.String("updated_at", user.UpdatedAt.Format(time.RFC3339)),
	)
}
