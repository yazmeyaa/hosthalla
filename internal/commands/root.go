package commands

import (
	"context"
	"fmt"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
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
			newDatabaseAliasCommand(),
			newUsersCommand(),
			newCreateUserAliasCommand(),
			newAgentCommand(),
			newAgentsCommand(),
			newPlaceholderCommand("tokens", "Manage API tokens."),
			newPlaceholderCommand("hosts", "Manage hosts."),
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
		Usage: "hosthalla bootstrap",
		Short: "Run first-time setup.",
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			return fmt.Errorf("bootstrap is not implemented yet")
		},
	}
}

func newAgentsCommand() *cliapp.Command {
	return newPlaceholderCommand("agents", "Manage registered agents.")
}

func newPlaceholderCommand(name string, short string) *cliapp.Command {
	return &cliapp.Command{
		Name:  name,
		Usage: "hosthalla " + name + " <command>",
		Short: short,
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			return fmt.Errorf("%s commands are not implemented yet", name)
		},
	}
}
