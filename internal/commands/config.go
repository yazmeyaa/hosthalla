package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
)

func newConfigCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "config",
		Usage: "hosthalla config <command>",
		Short: "Manage application config.",
		Children: []*cliapp.Command{
			newConfigGenerateCommand(),
			newConfigShowCommand(),
			newConfigValidateCommand(),
		},
	}
}

func newConfigGenerateCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "generate",
		Usage: "hosthalla config generate [--path <file>] [--overwrite]",
		Short: "Generate default application config.",
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			flags := flag.NewFlagSet("hosthalla config generate", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			path := flags.String("path", config.DefaultConfigPath, "path to config file")
			overwrite := flags.Bool("overwrite", false, "overwrite existing config file")
			if err := flags.Parse(args); err != nil {
				return cliapp.UsageError{Message: err.Error(), Usage: "hosthalla config generate [--path <file>] [--overwrite]"}
			}
			if flags.NArg() != 0 {
				return cliapp.UsageError{Message: "config generate does not accept positional arguments", Usage: "hosthalla config generate [--path <file>] [--overwrite]"}
			}
			if err := config.GenerateDefaultConfig(*path, *overwrite); err != nil {
				if errors.Is(err, config.ErrConfigAlreadyExists) {
					return cliapp.ExitError{Code: cliapp.ExitCodeError, Err: fmt.Errorf("config already exists at %q; use --overwrite to replace it", *path)}
				}
				return fmt.Errorf("generate config: %w", err)
			}
			fmt.Fprintf(env.Stdout, "Config generated at %q\n", *path)
			return nil
		},
	}
}

func newConfigShowCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "show",
		Usage: "hosthalla config show [--path <file>]",
		Short: "Print application config.",
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			flags := flag.NewFlagSet("hosthalla config show", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			path := flags.String("path", config.DefaultConfigPath, "path to config file")
			if err := flags.Parse(args); err != nil {
				return cliapp.UsageError{Message: err.Error(), Usage: "hosthalla config show [--path <file>]"}
			}
			if flags.NArg() != 0 {
				return cliapp.UsageError{Message: "config show does not accept positional arguments", Usage: "hosthalla config show [--path <file>]"}
			}
			content, err := config.ReadYAMLFromPath(*path)
			if err != nil {
				return fmt.Errorf("read config: %w", err)
			}
			_, err = env.Stdout.Write(content)
			return err
		},
	}
}

func newConfigValidateCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "validate",
		Usage: "hosthalla config validate [--path <file>]",
		Short: "Validate application config.",
		Run: func(ctx context.Context, env *cliapp.Env, args []string) error {
			flags := flag.NewFlagSet("hosthalla config validate", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			path := flags.String("path", config.DefaultConfigPath, "path to config file")
			if err := flags.Parse(args); err != nil {
				return cliapp.UsageError{Message: err.Error(), Usage: "hosthalla config validate [--path <file>]"}
			}
			if flags.NArg() != 0 {
				return cliapp.UsageError{Message: "config validate does not accept positional arguments", Usage: "hosthalla config validate [--path <file>]"}
			}
			cfg := config.AppConfig{}
			if err := cfg.LoadFromPath(*path); err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if _, err := cfg.SlogLevel(); err != nil {
				return fmt.Errorf("validate log_level: %w", err)
			}
			if _, err := cfg.SecretEncryptionKey(); err != nil {
				return fmt.Errorf("validate security.secret_encryption_key: %w", err)
			}
			fmt.Fprintf(env.Stdout, "Config %q is valid\n", *path)
			return nil
		},
	}
}
