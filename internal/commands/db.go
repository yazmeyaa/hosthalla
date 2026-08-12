package commands

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"

	_ "github.com/jackc/pgx/v5/stdlib"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
	appdatabase "github.com/yazmeyaa/hosthalla/internal/database"
	appmigrations "github.com/yazmeyaa/hosthalla/internal/migrations"
)

type migrator interface {
	Up() error
	Down() error
	Version() (uint, bool, error)
}

var openSQL = func(driverName, dsn string) (*sql.DB, error) {
	if driverName == "sqlite" {
		return appdatabase.OpenSQLite(dsn)
	}
	return sql.Open(driverName, dsn)
}
var newMigrator = func(db *sql.DB, strategy appmigrations.Strategy) (migrator, error) {
	return appmigrations.NewMigrator(db, strategy)
}

const dbFlagsUsage = "[--driver <sqlite|postgres>] [--dsn <connection-string>]"

func newDBCommand() *cliapp.Command {
	return &cliapp.Command{
		Name:  "db",
		Usage: "hosthalla [--config <file>] db <command>",
		Short: "Manage database migrations.",
		Children: []*cliapp.Command{
			newDBMigrateCommand("hosthalla [--config <file>] db migrate " + dbFlagsUsage),
			{
				Name:        "status",
				Usage:       "hosthalla [--config <file>] db status " + dbFlagsUsage,
				Short:       "Print migration status.",
				NeedsConfig: true,
				Run:         runDBStatus,
			},
			{
				Name:        "rollback",
				Usage:       "hosthalla [--config <file>] db rollback " + dbFlagsUsage,
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
	migrator, db, err := openMigrator(env, args, "hosthalla [--config <file>] db migrate "+dbFlagsUsage)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrator.Up(); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	fmt.Fprintln(env.Stdout, "Database migrations applied successfully")
	return nil
}

func runDBStatus(ctx context.Context, env *cliapp.Env, args []string) error {
	migrator, db, err := openMigrator(env, args, "hosthalla [--config <file>] db status "+dbFlagsUsage)
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
	migrator, db, err := openMigrator(env, args, "hosthalla [--config <file>] db rollback "+dbFlagsUsage)
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

func openMigrator(env *cliapp.Env, args []string, usage string) (migrator, *sql.DB, error) {
	flags := flag.NewFlagSet("database migrations", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	driverOverride := flags.String("driver", "", "database driver override")
	dsnOverride := flags.String("dsn", "", "database connection string override")
	if err := flags.Parse(args); err != nil {
		return nil, nil, cliapp.UsageError{Message: err.Error(), Usage: usage}
	}
	if flags.NArg() != 0 {
		return nil, nil, cliapp.UsageError{Message: "database command does not accept positional arguments", Usage: usage}
	}

	settings, err := env.Config.Database.ConnectionSettings(*driverOverride, *dsnOverride)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve database connection: %w", err)
	}

	var (
		sqlDriver string
		strategy  appmigrations.Strategy
	)
	switch settings.Driver {
	case config.DatabaseDriverPostgres:
		sqlDriver = "pgx"
		strategy = appmigrations.Postgres
	case config.DatabaseDriverSQLite:
		sqlDriver = "sqlite"
		strategy = appmigrations.SQLite
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", settings.Driver)
	}

	db, err := openSQL(sqlDriver, settings.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open database connection: %w", err)
	}
	migrator, err := newMigrator(db, strategy)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("initialize migrator: %w", err)
	}
	return migrator, db, nil
}
