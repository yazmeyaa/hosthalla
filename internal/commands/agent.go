package commands

import (
	"context"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
)

func newAgentCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "agent",
		Usage: "hosthalla agent <command>",
		Short: "Run local agent commands.",
		Children: []*cliapp.Command{
			{
				Name:  "register",
				Usage: "hosthalla agent register --host <server> --host-id <uuid> --token <token> [--scheme <http|https>] [--config <file>]",
				Short: "Register this machine as an agent.",
				Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
					return runAgentCommand(ctx, env.Stdout, env.Stderr, append([]string{"register"}, args...))
				},
			},
			{
				Name:  "run",
				Usage: "hosthalla agent run [--config <file>]",
				Short: "Run the local agent worker.",
				Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
					return runAgentCommand(ctx, env.Stdout, env.Stderr, append([]string{"run"}, args...))
				},
			},
		},
	}
}
