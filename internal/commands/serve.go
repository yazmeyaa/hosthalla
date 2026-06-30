package commands

import (
	"context"
	"fmt"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/web"
)

func newServeCommand(runner ServeRunner) *cliapp.Command {
	return &cliapp.Command{
		Name:  "serve",
		Usage: "hosthalla [--config <file>] serve",
		Short: "Start the web server.",
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			if len(args) != 0 {
				return cliapp.UsageError{Message: "serve does not accept arguments", Usage: "hosthalla [--config <file>] serve"}
			}
			if runner == nil {
				runner = func(ctx context.Context, configPath string) error {
					return web.Run(ctx, web.RunParams{ConfigPath: configPath})
				}
			}
			if err := runner(ctx, env.ConfigPath); err != nil {
				return fmt.Errorf("run web server: %w", err)
			}
			return nil
		},
	}
}
