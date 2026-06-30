package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	auth_repository "github.com/yazmeyaa/hosthalla/internal/authentication/storage"
	auth_storage "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	app_cli "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
	app_logger "github.com/yazmeyaa/hosthalla/internal/logger"
	app_migrations "github.com/yazmeyaa/hosthalla/internal/migrations"
)

func main() {
	bootstrapLogger := app_logger.NewLogger(app_logger.LoggerParams{
		Output: os.Stdout,
		Level:  slog.LevelWarn,
	})
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", config.DefaultConfigPath, "path to config file")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			printHelp()
			return
		}
		bootstrapLogger.Error("failed to parse command flags", slog.String("error", err.Error()))
		os.Exit(1)
	}

	args := flags.Args()
	if len(args) == 0 {
		printHelp()
		return
	}

	command := args[0]
	if command == "help" || command == "-h" || command == "--help" {
		printHelp()
		return
	}
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
	if err := pool.Ping(context.Background()); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("database connection pool initialized")

	logger.Info("running cli command", slog.String("command", command))
	switch command {
	case "create-user":
		createUser(logger, pool, args[1:])
	case "database":
		processDatabaseCommand(logger, cfg.Database.ConnectionString(), args[1:])
	default:
		logger.Error("unknown command", slog.String("command", command))
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Usage:")
	fmt.Println("  hosthalla [--config <file>] <command> [arguments]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  help                               Show this help message")
	fmt.Println("  config generate [--path <file>] [--overwrite]")
	fmt.Println("  config show [--path <file>]")
	fmt.Println("  create-user <username> <password>")
	fmt.Println("  database up")
	fmt.Println("  agent register --host <server> --host-id <uuid> --token <token> [--scheme <http|https>] [--config <file>]")
	fmt.Println("  agent run [--config <file>]")
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

func processDatabaseCommand(logger *slog.Logger, connectionString string, args []string) {
	if len(args) != 1 {
		logger.Error("invalid arguments", slog.String("command", "database"), slog.String("usage", "database up"))
		os.Exit(1)
	}

	switch args[0] {
	case "up":
		runDatabaseUp(logger, connectionString)
	default:
		logger.Error("unknown database command", slog.String("command", args[0]), slog.String("usage", "database up"))
		os.Exit(1)
	}
}

func runDatabaseUp(logger *slog.Logger, connectionString string) {
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		logger.Error("failed to open database connection", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	migrator, err := app_migrations.NewMigrator(db)
	if err != nil {
		logger.Error("failed to initialize migrator", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := migrator.Up(); err != nil {
		logger.Error("failed to apply migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("database migrations applied successfully")
}
