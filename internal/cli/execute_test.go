package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/config"
)

func TestExecuteEmptyArgsPrintsRootHelp(t *testing.T) {
	root := testRoot(nil)
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, nil, &stdout, &stderr, testDeps(nil))

	if code != ExitCodeOK {
		t.Fatalf("exit code = %d, want %d", code, ExitCodeOK)
	}
	if !strings.Contains(stdout.String(), "Usage:") || !strings.Contains(stdout.String(), "users") {
		t.Fatalf("stdout does not contain root help:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestExecuteResolvesSimpleCommand(t *testing.T) {
	var gotArgs []string
	root := testRoot(func(ctx context.Context, env *Env, args []string) error {
		gotArgs = append([]string(nil), args...)
		return nil
	})
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, []string{"users", "list", "one", "two"}, &stdout, &stderr, testDeps(nil))

	if code != ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !reflect.DeepEqual(gotArgs, []string{"one", "two"}) {
		t.Fatalf("args = %#v", gotArgs)
	}
}

func TestExecuteResolvesNestedCommand(t *testing.T) {
	var ran bool
	root := &Command{
		Name:  "hosthalla",
		Usage: "hosthalla <command>",
		Children: []*Command{
			{
				Name:  "users",
				Usage: "hosthalla users <command>",
				Children: []*Command{
					{
						Name:  "password",
						Usage: "hosthalla users password <command>",
						Children: []*Command{
							{
								Name:  "set",
								Usage: "hosthalla users password set <user>",
								Run: func(ctx context.Context, env *Env, args []string) error {
									ran = true
									if !reflect.DeepEqual(args, []string{"alice"}) {
										t.Fatalf("args = %#v", args)
									}
									return nil
								},
							},
						},
					},
				},
			},
		},
	}
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, []string{"users", "password", "set", "alice"}, &stdout, &stderr, testDeps(nil))

	if code != ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !ran {
		t.Fatal("nested command did not run")
	}
}

func TestExecuteUnknownCommandReturnsUsageError(t *testing.T) {
	root := testRoot(nil)
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, []string{"nope"}, &stdout, &stderr, testDeps(nil))

	if code != ExitCodeUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitCodeUsage)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestExecuteSupportsHelpForms(t *testing.T) {
	tests := [][]string{
		{"help", "users"},
		{"users", "help"},
		{"users", "list", "--help"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := testRoot(nil)
			var stdout, stderr bytes.Buffer

			code := Execute(context.Background(), root, args, &stdout, &stderr, testDeps(nil))

			if code != ExitCodeOK {
				t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("stdout does not contain help:\n%s", stdout.String())
			}
		})
	}
}

func TestExecuteParsesGlobalFlags(t *testing.T) {
	var gotConfigPath string
	var gotJSON bool
	root := testRoot(func(ctx context.Context, env *Env, args []string) error {
		gotConfigPath = env.ConfigPath
		gotJSON = env.JSON
		return nil
	})
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, []string{"--config", "custom.yaml", "--json", "users", "list"}, &stdout, &stderr, testDeps(nil))

	if code != ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if gotConfigPath != "custom.yaml" {
		t.Fatalf("ConfigPath = %q", gotConfigPath)
	}
	if !gotJSON {
		t.Fatal("JSON flag was not set")
	}
}

func TestExecuteParsesJSONFlagAfterCommand(t *testing.T) {
	var gotArgs []string
	var gotJSON bool
	root := testRoot(func(ctx context.Context, env *Env, args []string) error {
		gotArgs = append([]string(nil), args...)
		gotJSON = env.JSON
		return nil
	})
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, []string{"users", "list", "--json"}, &stdout, &stderr, testDeps(nil))

	if code != ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !gotJSON {
		t.Fatal("JSON flag was not set")
	}
	if len(gotArgs) != 0 {
		t.Fatalf("args = %#v", gotArgs)
	}
}

func TestExecuteLoadsConfigOnlyWhenNeeded(t *testing.T) {
	var loadCount int
	deps := testDeps(&loadCount)
	root := &Command{
		Name:  "hosthalla",
		Usage: "hosthalla <command>",
		Children: []*Command{
			{Name: "plain", Usage: "hosthalla plain", Run: func(ctx context.Context, env *Env, args []string) error { return nil }},
			{Name: "needs-config", Usage: "hosthalla needs-config", NeedsConfig: true, Run: func(ctx context.Context, env *Env, args []string) error { return nil }},
		},
	}

	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), root, []string{"plain"}, &stdout, &stderr, deps)
	if code != ExitCodeOK {
		t.Fatalf("plain exit code = %d, stderr = %q", code, stderr.String())
	}
	if loadCount != 0 {
		t.Fatalf("loadCount after plain = %d", loadCount)
	}

	code = Execute(context.Background(), root, []string{"needs-config"}, &stdout, &stderr, deps)
	if code != ExitCodeOK {
		t.Fatalf("needs-config exit code = %d, stderr = %q", code, stderr.String())
	}
	if loadCount != 1 {
		t.Fatalf("loadCount after needs-config = %d", loadCount)
	}
}

func TestExecuteOpensDBAfterConfig(t *testing.T) {
	var steps []string
	root := &Command{
		Name:  "hosthalla",
		Usage: "hosthalla <command>",
		Children: []*Command{
			{
				Name:        "db-command",
				Usage:       "hosthalla db-command",
				NeedsConfig: true,
				NeedsDB:     true,
				Run: func(ctx context.Context, env *Env, args []string) error {
					steps = append(steps, "run")
					return nil
				},
			},
		},
	}
	deps := testDeps(nil)
	deps.LoadConfig = func(path string) (*config.AppConfig, error) {
		steps = append(steps, "config")
		cfg := config.NewDefaultAppConfig()
		return &cfg, nil
	}
	deps.OpenDB = func(ctx context.Context, cfg *config.AppConfig) (*pgxpool.Pool, error) {
		steps = append(steps, "db")
		return nil, nil
	}
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, []string{"db-command"}, &stdout, &stderr, deps)

	if code != ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !reflect.DeepEqual(steps, []string{"config", "db", "run"}) {
		t.Fatalf("steps = %#v", steps)
	}
}

func TestExecuteDependencyErrors(t *testing.T) {
	tests := []struct {
		name string
		deps Dependencies
	}{
		{
			name: "config",
			deps: Dependencies{
				LoadConfig: func(path string) (*config.AppConfig, error) {
					return nil, errors.New("boom config")
				},
			},
		},
		{
			name: "db",
			deps: Dependencies{
				LoadConfig: func(path string) (*config.AppConfig, error) {
					cfg := config.NewDefaultAppConfig()
					return &cfg, nil
				},
				OpenDB: func(ctx context.Context, cfg *config.AppConfig) (*pgxpool.Pool, error) {
					return nil, errors.New("boom db")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &Command{
				Name:  "hosthalla",
				Usage: "hosthalla <command>",
				Children: []*Command{
					{Name: "cmd", Usage: "hosthalla cmd", NeedsConfig: true, NeedsDB: tt.name == "db", Run: func(ctx context.Context, env *Env, args []string) error { return nil }},
				},
			}
			if tt.deps.NewLogger == nil {
				tt.deps.NewLogger = func(output io.Writer, level slog.Level) *slog.Logger {
					return slog.New(slog.NewTextHandler(io.Discard, nil))
				}
			}
			var stdout, stderr bytes.Buffer

			code := Execute(context.Background(), root, []string{"cmd"}, &stdout, &stderr, tt.deps)

			if code != ExitCodeError {
				t.Fatalf("exit code = %d, want %d", code, ExitCodeError)
			}
			if !strings.Contains(stderr.String(), "boom") {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestExecuteUsageErrorPrintsUsage(t *testing.T) {
	root := testRoot(func(ctx context.Context, env *Env, args []string) error {
		return UsageError{Message: "bad args", Usage: "hosthalla users list"}
	})
	var stdout, stderr bytes.Buffer

	code := Execute(context.Background(), root, []string{"users", "list"}, &stdout, &stderr, testDeps(nil))

	if code != ExitCodeUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitCodeUsage)
	}
	if !strings.Contains(stderr.String(), "bad args") || !strings.Contains(stderr.String(), "hosthalla users list") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func testRoot(run RunFunc) *Command {
	if run == nil {
		run = func(ctx context.Context, env *Env, args []string) error { return nil }
	}
	return &Command{
		Name:  "hosthalla",
		Usage: "hosthalla <command>",
		Children: []*Command{
			{
				Name:  "users",
				Usage: "hosthalla users <command>",
				Children: []*Command{
					{Name: "list", Usage: "hosthalla users list", Run: run},
				},
			},
		},
	}
}

func testDeps(loadCount *int) Dependencies {
	return Dependencies{
		LoadConfig: func(path string) (*config.AppConfig, error) {
			if loadCount != nil {
				*loadCount = *loadCount + 1
			}
			cfg := config.NewDefaultAppConfig()
			return &cfg, nil
		},
		OpenDB: func(ctx context.Context, cfg *config.AppConfig) (*pgxpool.Pool, error) {
			return nil, nil
		},
		NewLogger: func(output io.Writer, level slog.Level) *slog.Logger {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		},
	}
}
