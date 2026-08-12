package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	agentpostgres "github.com/yazmeyaa/hosthalla/internal/agent/postgres"
	agentsqlite "github.com/yazmeyaa/hosthalla/internal/agent/sqlite"
	authstorage "github.com/yazmeyaa/hosthalla/internal/authentication/storage"
	authpostgres "github.com/yazmeyaa/hosthalla/internal/authentication/storage/postgres"
	authsqlite "github.com/yazmeyaa/hosthalla/internal/authentication/storage/sqlite"
	"github.com/yazmeyaa/hosthalla/internal/config"
	"github.com/yazmeyaa/hosthalla/internal/host"
	hostpostgres "github.com/yazmeyaa/hosthalla/internal/host/postgres"
	hostsqlite "github.com/yazmeyaa/hosthalla/internal/host/sqlite"
	_ "modernc.org/sqlite"
)

type Store struct {
	Driver config.DatabaseDriver

	ProfileRepository                authstorage.ProfileRepository
	PasswordAuthenticationRepository authstorage.PasswordAuthenticationRepository
	SessionRepository                authstorage.SessionRepository
	APITokenRepository               authstorage.APITokenRepository
	HostRepository                   host.HostRepository
	HostManagementMethodRepository   host.HostManagementMethodRepository
	HostSystemInfoRepository         host.HostSystemInfoRepository
	HostMetricSnapshotRepository     host.HostMetricSnapshotRepository
	AgentRepository                  agent.Repository
	AgentConfigRepository            agent.AgentConfigRepository

	sqlDB *sql.DB
	close func()
}

func Open(ctx context.Context, cfg config.DatabaseConfig) (*Store, error) {
	settings, err := cfg.ConnectionSettings("", "")
	if err != nil {
		return nil, err
	}

	switch settings.Driver {
	case config.DatabaseDriverPostgres:
		pool, err := pgxpool.New(ctx, settings.DSN)
		if err != nil {
			return nil, err
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, err
		}
		return newPostgresStore(pool), nil
	case config.DatabaseDriverSQLite:
		db, err := OpenSQLite(settings.DSN)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(4)
		if err := db.PingContext(ctx); err != nil {
			db.Close()
			return nil, err
		}
		return newSQLiteStore(db), nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", settings.Driver)
	}
}

func OpenSQLite(dsn string) (*sql.DB, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse sqlite DSN: %w", err)
	}
	path := parsed.Path
	if parsed.Opaque != "" {
		path = parsed.Opaque
	}
	if parsed.Scheme == "" {
		path = strings.SplitN(dsn, "?", 2)[0]
	}
	if path != "" && path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite database directory: %w", err)
		}
	}
	return sql.Open("sqlite", dsn)
}

func (s *Store) Close() {
	if s != nil && s.close != nil {
		s.close()
	}
}

func newPostgresStore(pool *pgxpool.Pool) *Store {
	hostRepositories := hostpostgres.NewRepositories(pool)
	return &Store{
		Driver:                           config.DatabaseDriverPostgres,
		ProfileRepository:                authpostgres.NewProfileRepository(pool),
		PasswordAuthenticationRepository: authpostgres.NewPasswordAuthenticationRepository(pool),
		SessionRepository:                authpostgres.NewSessionRepository(pool),
		APITokenRepository:               authpostgres.NewAPITokenRepository(pool),
		HostRepository:                   hostRepositories.Host,
		HostManagementMethodRepository:   hostRepositories.HostManagementMethod,
		HostSystemInfoRepository:         hostRepositories.HostSystemInfo,
		HostMetricSnapshotRepository:     hostRepositories.HostMetricSnapshot,
		AgentRepository:                  agentpostgres.NewAgentRepository(pool),
		AgentConfigRepository:            agentpostgres.NewAgentConfigRepository(pool),
		close:                            pool.Close,
	}
}

func newSQLiteStore(db *sql.DB) *Store {
	hostRepositories := hostsqlite.NewRepositories(db)
	return &Store{
		Driver:                           config.DatabaseDriverSQLite,
		ProfileRepository:                authsqlite.NewProfileRepository(db),
		PasswordAuthenticationRepository: authsqlite.NewPasswordAuthenticationRepository(db),
		SessionRepository:                authsqlite.NewSessionRepository(db),
		APITokenRepository:               authsqlite.NewAPITokenRepository(db),
		HostRepository:                   hostRepositories.Host,
		HostManagementMethodRepository:   hostRepositories.HostManagementMethod,
		HostSystemInfoRepository:         hostRepositories.HostSystemInfo,
		HostMetricSnapshotRepository:     hostRepositories.HostMetricSnapshot,
		AgentRepository:                  agentsqlite.NewAgentRepository(db),
		AgentConfigRepository:            agentsqlite.NewAgentConfigRepository(db),
		sqlDB:                            db,
		close:                            func() { _ = db.Close() },
	}
}
