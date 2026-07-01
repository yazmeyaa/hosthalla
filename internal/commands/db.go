package commands

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	app_migrations "github.com/yazmeyaa/hosthalla/internal/migrations"
)

type migrator interface {
	Up() error
	Down() error
	Version() (uint, bool, error)
}

var openSQL = sql.Open
var newMigrator = func(db *sql.DB) (migrator, error) {
	return app_migrations.NewMigrator(db)
}

func newDBCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "db",
		Usage: "hosthalla [--config <file>] db <command>",
		Short: "Manage database migrations.",
		Children: []*cliapp.Command{
			newDBMigrateCommand("hosthalla [--config <file>] db migrate"),
			{
				Name:        "status",
				Usage:       "hosthalla [--config <file>] db status",
				Short:       "Print migration status.",
				NeedsConfig: true,
				Run:         runDBStatus,
			},
			{
				Name:        "rollback",
				Usage:       "hosthalla [--config <file>] db rollback",
				Short:       "Roll back one migration.",
				NeedsConfig: true,
				Run:         runDBRollback,
			},
		},
	}
}

func newDBMigrateCommand(usage string) *cliapp.Command {
	return &cliapp.Command{
		Name:        "migrate",
		Usage:       usage,
		Short:       "Apply pending migrations.",
		NeedsConfig: true,
		Run:         runDBMigrate,
	}
}

func runDBMigrate(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 0 {
		return cliapp.UsageError{Message: "db migrate does not accept arguments", Usage: "hosthalla [--config <file>] db migrate"}
	}

	db, err := openSQL("pgx", env.Config.Database.ConnectionString())
	if err != nil {
		return fmt.Errorf("open database connection: %w", err)
	}
	defer db.Close()

	migrator, err := newMigrator(db)
	if err != nil {
		return fmt.Errorf("initialize migrator: %w", err)
	}

	if err := migrator.Up(); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	fmt.Fprintln(env.Stdout, "Database migrations applied successfully")
	return nil
}

func runDBStatus(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 0 {
		return cliapp.UsageError{Message: "db status does not accept arguments", Usage: "hosthalla [--config <file>] db status"}
	}

	migrator, db, err := openMigrator(env)
	if err != nil {
		return err
	}
	defer db.Close()

	version, dirty, err := migrator.Version()
	if err != nil {
		return err
	}
	if env.JSON {
		return writeJSON(env.Stdout, map[string]any{"version": version, "dirty": dirty})
	}
	fmt.Fprintf(env.Stdout, "Migration version: %d\nDirty: %t\n", version, dirty)
	return nil
}

func runDBRollback(ctx context.Context, env *cliapp.Env, args []string) error {
	if len(args) != 0 {
		return cliapp.UsageError{Message: "db rollback does not accept arguments", Usage: "hosthalla [--config <file>] db rollback"}
	}

	migrator, db, err := openMigrator(env)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrator.Down(); err != nil {
		return fmt.Errorf("roll back migration: %w", err)
	}
	fmt.Fprintln(env.Stdout, "Database rolled back by one migration")
	return nil
}

func openMigrator(env *cliapp.Env) (migrator, *sql.DB, error) {
	db, err := openSQL("pgx", env.Config.Database.ConnectionString())
	if err != nil {
		return nil, nil, fmt.Errorf("open database connection: %w", err)
	}

	migrator, err := newMigrator(db)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("initialize migrator: %w", err)
	}
	return migrator, db, nil
}
