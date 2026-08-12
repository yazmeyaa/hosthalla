package commands

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
	appdatabase "github.com/yazmeyaa/hosthalla/internal/database"
	appmigrations "github.com/yazmeyaa/hosthalla/internal/migrations"
)

type fakeMigrator struct {
	upCalled *bool
}

func TestSQLiteCLIWorkflow(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.NewDefaultAppConfig()
	cfg.Database.Path = filepath.Join(filepath.Dir(configPath), "hosthalla.sqlite")
	if err := cfg.SaveToPath(configPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	root := NewRoot(RootParams{})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--config", configPath, "db", "migrate"}, want: "Database migrations applied successfully"},
		{args: []string{"--config", configPath, "users", "create", "alice", "correct horse battery staple"}, want: "User created: alice"},
		{args: []string{"--config", configPath, "users", "list"}, want: "alice"},
	} {
		var stdout, stderr bytes.Buffer
		code := cliapp.Execute(context.Background(), root, test.args, &stdout, &stderr, cliapp.DefaultDependencies())
		if code != cliapp.ExitCodeOK {
			t.Fatalf("%v: exit code = %d, stderr = %q", test.args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), test.want) {
			t.Fatalf("%v: stdout = %q, want %q", test.args, stdout.String(), test.want)
		}
	}
}

func (m fakeMigrator) Up() error {
	*m.upCalled = true
	return nil
}

func (m fakeMigrator) Down() error {
	return nil
}

func (m fakeMigrator) Version() (uint, bool, error) {
	return 0, false, nil
}

func TestDBMigrateUsesConfigAndMigrator(t *testing.T) {
	oldOpenSQL := openSQL
	oldNewMigrator := newMigrator
	defer func() {
		openSQL = oldOpenSQL
		newMigrator = oldNewMigrator
	}()

	var opened bool
	var migrated bool
	openSQL = func(driverName string, dataSourceName string) (*sql.DB, error) {
		opened = true
		if driverName != "sqlite" {
			t.Fatalf("driver = %q, want sqlite", driverName)
		}
		if dataSourceName != "file:override.sqlite" {
			t.Fatalf("DSN = %q, want override", dataSourceName)
		}
		return sql.Open("sqlite", ":memory:")
	}
	newMigrator = func(db *sql.DB, strategy appmigrations.Strategy) (migrator, error) {
		if strategy != appmigrations.SQLite {
			t.Fatalf("strategy = %q, want sqlite", strategy)
		}
		return fakeMigrator{upCalled: &migrated}, nil
	}

	cfg := config.NewDefaultAppConfig()
	cfg.Database = config.DatabaseConfig{Driver: "postgres", Host: "localhost", Port: 5432, User: "hosthalla", Database: "hosthalla"}
	deps := cliapp.Dependencies{
		LoadConfig: func(path string) (*config.AppConfig, error) {
			return &cfg, nil
		},
		OpenDB: func(ctx context.Context, cfg *config.AppConfig) (*appdatabase.Store, error) {
			t.Fatal("db migrate should not open application store")
			return nil, nil
		},
		NewLogger: func(output io.Writer, level slog.Level) *slog.Logger {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		},
	}

	root := NewRoot(RootParams{})
	var stdout, stderr bytes.Buffer
	code := cliapp.Execute(context.Background(), root, []string{"db", "migrate", "--driver", "sqlite", "--dsn", "file:override.sqlite"}, &stdout, &stderr, deps)

	if code != cliapp.ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !opened {
		t.Fatal("database was not opened")
	}
	if !migrated {
		t.Fatal("migration was not run")
	}
	if !strings.Contains(stdout.String(), "Database migrations applied successfully") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
