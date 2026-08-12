package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yazmeyaa/hosthalla/internal/config"
)

func TestOpenSQLiteStore(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "nested", "hosthalla.sqlite")
	store, err := Open(context.Background(), config.DatabaseConfig{
		Driver: "sqlite",
		Path:   databasePath,
	})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if store.Driver != config.DatabaseDriverSQLite || store.ProfileRepository == nil || store.HostRepository == nil || store.AgentRepository == nil {
		t.Fatalf("sqlite repositories were not initialized: %#v", store)
	}
	if got := store.sqlDB.Stats().MaxOpenConnections; got != 4 {
		t.Fatalf("max open connections = %d, want 4", got)
	}

	var foreignKeys int
	if err := store.sqlDB.QueryRow(`pragma foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	var journalMode string
	if err := store.sqlDB.QueryRow(`pragma journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	if _, err := os.Stat(databasePath); err != nil {
		t.Fatalf("sqlite database was not created: %v", err)
	}
}
