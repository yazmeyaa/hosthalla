package migrations

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	embedded_migrations "github.com/yazmeyaa/hosthalla/migrations"
)

type Migrator struct {
	migrate *migrate.Migrate
}

type Strategy string

const (
	Postgres Strategy = "postgres"
	SQLite   Strategy = "sqlite"
)

func NewMigrator(db *sql.DB, strategy Strategy) (*Migrator, error) {
	var (
		driver       database.Driver
		databaseName string
		err          error
	)
	switch strategy {
	case Postgres:
		driver, err = postgres.WithInstance(db, &postgres.Config{})
		databaseName = "postgres"
	case SQLite:
		driver, err = sqlite.WithInstance(db, &sqlite.Config{})
		databaseName = "sqlite"
	default:
		return nil, fmt.Errorf("unsupported migration strategy %q", strategy)
	}
	if err != nil {
		return nil, fmt.Errorf("create migration db driver: %w", err)
	}

	sourceDriver, err := iofs.New(embedded_migrations.Files, string(strategy))
	if err != nil {
		return nil, fmt.Errorf("create migration source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, databaseName, driver)
	if err != nil {
		return nil, fmt.Errorf("create migrator: %w", err)
	}

	return &Migrator{migrate: m}, nil
}

func (m *Migrator) Up() error {
	err := m.migrate.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run up migrations: %w", err)
	}

	return nil
}

func (m *Migrator) Down() error {
	err := m.migrate.Steps(-1)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run down migration step: %w", err)
	}

	return nil
}

func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("read migration version: %w", err)
	}

	return version, dirty, nil
}
