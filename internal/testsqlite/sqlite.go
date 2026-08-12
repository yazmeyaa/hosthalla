package testsqlite

import (
	"database/sql"
	"path/filepath"
	"testing"

	appmigrations "github.com/yazmeyaa/hosthalla/internal/migrations"
	_ "modernc.org/sqlite"
)

func Open(t testing.TB) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosthalla.sqlite")
	db, err := sql.Open("sqlite", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	migrator, err := appmigrations.NewMigrator(db, appmigrations.SQLite)
	if err != nil {
		t.Fatalf("create sqlite migrator: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("apply sqlite migrations: %v", err)
	}
	return db
}
