package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/config"
	app_logger "github.com/yazmeyaa/hosthalla/internal/logger"
)

type Dependencies struct {
	LoadConfig func(path string) (*config.AppConfig, error)
	OpenDB     func(ctx context.Context, cfg *config.AppConfig) (*pgxpool.Pool, error)
	NewLogger  func(output io.Writer, level slog.Level) *slog.Logger
}

func DefaultDependencies() Dependencies {
	return Dependencies{
		LoadConfig: func(path string) (*config.AppConfig, error) {
			cfg := config.AppConfig{}
			if err := cfg.LoadFromPath(path); err != nil {
				return nil, err
			}
			return &cfg, nil
		},
		OpenDB: func(ctx context.Context, cfg *config.AppConfig) (*pgxpool.Pool, error) {
			pool, err := pgxpool.New(ctx, cfg.Database.ConnectionString())
			if err != nil {
				return nil, err
			}
			if err := pool.Ping(ctx); err != nil {
				pool.Close()
				return nil, err
			}
			return pool, nil
		},
		NewLogger: func(output io.Writer, level slog.Level) *slog.Logger {
			return app_logger.NewLogger(app_logger.LoggerParams{
				Output: output,
				Level:  level,
			})
		},
	}
}

func Execute(ctx context.Context, root *Command, args []string, stdout io.Writer, stderr io.Writer, deps Dependencies) int {
	if deps.LoadConfig == nil {
		deps.LoadConfig = DefaultDependencies().LoadConfig
	}
	if deps.OpenDB == nil {
		deps.OpenDB = DefaultDependencies().OpenDB
	}
	if deps.NewLogger == nil {
		deps.NewLogger = DefaultDependencies().NewLogger
	}

	env := &Env{
		Stdout:     stdout,
		Stderr:     stderr,
		ConfigPath: config.DefaultConfigPath,
		Logger:     deps.NewLogger(stderr, slog.LevelWarn),
	}

	remaining, helpRequested, err := parseGlobalFlags(root, args, env)
	if err != nil {
		return handleError(stderr, err)
	}
	if helpRequested {
		PrintHelp(stdout, root)
		return ExitCodeOK
	}

	resolved, err := resolve(root, remaining)
	if err != nil {
		return handleError(stderr, err)
	}
	if resolved.Help {
		PrintHelp(stdout, resolved.Command)
		return ExitCodeOK
	}

	if err := prepare(ctx, env, resolved.Command, deps); err != nil {
		return handleError(stderr, err)
	}
	if env.DB != nil {
		defer env.DB.Close()
	}

	if err := resolved.Command.Run(ctx, env, resolved.Args); err != nil {
		return handleError(stderr, err)
	}
	return ExitCodeOK
}

func parseGlobalFlags(root *Command, args []string, env *Env) ([]string, bool, error) {
	flags := flag.NewFlagSet(root.Name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	configPath := flags.String("config", env.ConfigPath, "path to config file")
	jsonOutput := flags.Bool("json", false, "print machine-readable JSON")
	help := flags.Bool("help", false, "show help")
	flags.BoolVar(help, "h", false, "show help")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, true, nil
		}
		return nil, false, UsageError{Message: err.Error(), Usage: root.Usage}
	}

	env.ConfigPath = *configPath
	env.JSON = *jsonOutput
	return flags.Args(), *help, nil
}

func prepare(ctx context.Context, env *Env, cmd *Command, deps Dependencies) error {
	if cmd.NeedsConfig || cmd.NeedsDB {
		cfg, err := deps.LoadConfig(env.ConfigPath)
		if err != nil {
			return fmt.Errorf("load config %q: %w", env.ConfigPath, err)
		}
		env.Config = cfg

		logLevel, err := cfg.SlogLevel()
		if err != nil {
			return fmt.Errorf("invalid config value log_level: %w", err)
		}
		env.Logger = deps.NewLogger(env.Stderr, logLevel)
	}

	if cmd.NeedsDB {
		db, err := deps.OpenDB(ctx, env.Config)
		if err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		env.DB = db
	}

	return nil
}

func handleError(stderr io.Writer, err error) int {
	var usageErr UsageError
	if errors.As(err, &usageErr) {
		if usageErr.Message != "" {
			fmt.Fprintf(stderr, "Error: %s\n", usageErr.Message)
		}
		if usageErr.Usage != "" {
			fmt.Fprintf(stderr, "Usage:\n  %s\n", usageErr.Usage)
		}
		return ExitCode(err)
	}

	var exitErr ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", exitErr.Err)
		}
		return ExitCode(err)
	}

	fmt.Fprintf(stderr, "Error: %s\n", err)
	return ExitCode(err)
}
