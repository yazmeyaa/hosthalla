package commands

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
)

type fakeMigrator struct {
	upCalled *bool
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
		if !strings.Contains(dataSourceName, "hosthalla") {
			t.Fatalf("unexpected connection string: %q", dataSourceName)
		}
		return sql.Open("pgx", dataSourceName)
	}
	newMigrator = func(db *sql.DB) (migrator, error) {
		return fakeMigrator{upCalled: &migrated}, nil
	}

	cfg := config.NewDefaultAppConfig()
	deps := cliapp.Dependencies{
		LoadConfig: func(path string) (*config.AppConfig, error) {
			return &cfg, nil
		},
		OpenDB: func(ctx context.Context, cfg *config.AppConfig) (*pgxpool.Pool, error) {
			t.Fatal("db migrate should not open pgxpool")
			return nil, nil
		},
		NewLogger: func(output io.Writer, level slog.Level) *slog.Logger {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		},
	}

	root := NewRoot(RootParams{})
	var stdout, stderr bytes.Buffer
	code := cliapp.Execute(context.Background(), root, []string{"db", "migrate"}, &stdout, &stderr, deps)

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
