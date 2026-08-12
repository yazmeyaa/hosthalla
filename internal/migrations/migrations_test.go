package migrations

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	embeddedmigrations "github.com/yazmeyaa/hosthalla/migrations"
	_ "modernc.org/sqlite"
)

func TestEmbeddedMigrationSources(t *testing.T) {
	for _, strategy := range []Strategy{Postgres, SQLite} {
		if _, err := iofs.New(embeddedmigrations.Files, string(strategy)); err != nil {
			t.Fatalf("open %s migrations: %v", strategy, err)
		}
	}
}

func TestSQLiteMigrationLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "hosthalla.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	migrator, err := NewMigrator(db, SQLite)
	if err != nil {
		t.Fatalf("NewMigrator returned error: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("Up returned error: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("second Up returned error: %v", err)
	}
	version, dirty, err := migrator.Version()
	if err != nil {
		t.Fatalf("Version returned error: %v", err)
	}
	if version != 20260803121000 || dirty {
		t.Fatalf("version = %d, dirty = %t", version, dirty)
	}

	for _, name := range []string{"profile", "host", "agent", "host_metric"} {
		var count int
		if err := db.QueryRow(`select count(*) from sqlite_master where type = 'table' and name = ?`, name).Scan(&count); err != nil {
			t.Fatalf("query table %s: %v", name, err)
		}
		if count != 1 {
			t.Fatalf("table %s was not created", name)
		}
	}
	var indexCount int
	if err := db.QueryRow(`select count(*) from sqlite_master where type = 'index' and name = 'host_metric_snapshot_host_id_timestamp_idx'`).Scan(&indexCount); err != nil {
		t.Fatalf("query index: %v", err)
	}
	if indexCount != 1 {
		t.Fatal("host metric index was not created")
	}
	if _, err := db.Exec(`insert into host (id, name, ip, created_at, updated_at) values ('host-1', 'host', '127.0.0.1', current_timestamp, current_timestamp)`); err != nil {
		t.Fatalf("insert host before rollback: %v", err)
	}
	if _, err := db.Exec(`insert into agent (id, host_id, version, created_at, last_seen_at) values ('agent-1', 'host-1', 'test', current_timestamp, current_timestamp)`); err != nil {
		t.Fatalf("insert agent before rollback: %v", err)
	}
	if _, err := db.Exec(`update host set monitoring_agent_id = 'agent-1' where id = 'host-1'`); err != nil {
		t.Fatalf("link host and agent before rollback: %v", err)
	}

	if err := migrator.Down(); err != nil {
		t.Fatalf("Down returned error: %v", err)
	}
	var hostTableCount int
	if err := db.QueryRow(`select count(*) from sqlite_master where type = 'table' and name = 'host'`).Scan(&hostTableCount); err != nil {
		t.Fatalf("query rolled back schema: %v", err)
	}
	if hostTableCount != 0 {
		t.Fatal("host table still exists after rollback")
	}
}
