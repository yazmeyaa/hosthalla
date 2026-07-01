package commands

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
	"github.com/yazmeyaa/hosthalla/internal/version"
)

type ServeRunner func(ctx context.Context, configPath string) error

type RootParams struct {
	ServeRunner ServeRunner
}

func NewRoot(params RootParams) *cliapp.Command {
	return &cliapp.Command{
		Name:  "hosthalla",
		Usage: "hosthalla [--config <file>] [--json] <command> [arguments]",
		Short: "Hosthalla command line interface.",
		Children: []*cliapp.Command{
			newServeCommand(params.ServeRunner),
			newBootstrapCommand(),
			newVersionCommand(),
			newConfigCommand(),
			newDBCommand(),
			newUsersCommand(),
			newAgentCommand(),
			newAgentsCommand(),
			newTokensCommand(),
			newHostsCommand(),
		},
	}
}

func newVersionCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "version",
		Usage: "hosthalla version",
		Short: "Print version information.",
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			if len(args) != 0 {
				return cliapp.UsageError{Message: "version does not accept arguments", Usage: "hosthalla version"}
			}
			fmt.Fprintln(env.Stdout, version.VersionString())
			return nil
		},
	}
}

func newBootstrapCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "bootstrap",
		Usage: "hosthalla [--config <file>] bootstrap [--username <username> --password <password>]",
		Short: "Run first-time setup.",
		Run:   runBootstrap,
	}
}

func runBootstrap(ctx context.Context, env *cliapp.Env, args []string) error {
	flags := flag.NewFlagSet("hosthalla bootstrap", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	username := flags.String("username", "", "first user username")
	password := flags.String("password", "", "first user password")
	if err := flags.Parse(args); err != nil {
		return cliapp.UsageError{Message: err.Error(), Usage: "hosthalla [--config <file>] bootstrap [--username <username> --password <password>]"}
	}
	if flags.NArg() != 0 {
		return cliapp.UsageError{Message: "bootstrap does not accept positional arguments", Usage: "hosthalla [--config <file>] bootstrap [--username <username> --password <password>]"}
	}

	exists, err := config.ConfigExists(env.ConfigPath)
	if err != nil {
		return err
	}
	if !exists {
		if err := config.GenerateDefaultConfig(env.ConfigPath, false); err != nil {
			return fmt.Errorf("generate config: %w", err)
		}
		fmt.Fprintf(env.Stdout, "Config generated at %q\n", env.ConfigPath)
		fmt.Fprintln(env.Stdout, "Edit the database settings, then run bootstrap again.")
		return nil
	}

	cfg := config.AppConfig{}
	if err := cfg.LoadFromPath(env.ConfigPath); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	env.Config = &cfg

	if err := runDBMigrate(ctx, env, nil); err != nil {
		return err
	}
	if *username != "" || *password != "" {
		if *username == "" || *password == "" {
			return cliapp.UsageError{Message: "--username and --password must be provided together", Usage: "hosthalla [--config <file>] bootstrap [--username <username> --password <password>]"}
		}
		pool, err := pgxpool.New(ctx, env.Config.Database.ConnectionString())
		if err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		defer pool.Close()
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("ping database: %w", err)
		}
		env.DB = pool
		if err := runUsersCreate(ctx, env, []string{*username, *password}); err != nil {
			return err
		}
	}
	fmt.Fprintln(env.Stdout, "Bootstrap complete")
	fmt.Fprintln(env.Stdout, "Next command: hosthalla serve")
	return nil
}
