package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPublicWebOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		want    string
		wantErr bool
	}{
		{name: "normalizes trailing slash", origin: " https://hosthalla.example.com/ ", want: "https://hosthalla.example.com"},
		{name: "rejects path", origin: "https://hosthalla.example.com/app", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := NewDefaultAppConfig()
			cfg.WebOrigin = test.origin
			cfg.ApplyDefaults()

			origin, err := cfg.PublicWebOrigin()
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("PublicWebOrigin returned error: %v", err)
			}
			if origin != test.want {
				t.Fatalf("origin = %q", origin)
			}
		})
	}
}

func TestDefaultDatabaseIsSQLite(t *testing.T) {
	cfg := NewDefaultAppConfig()
	settings, err := cfg.Database.ConnectionSettings("", "")
	if err != nil {
		t.Fatalf("ConnectionSettings returned error: %v", err)
	}
	if settings.Driver != DatabaseDriverSQLite {
		t.Fatalf("driver = %q, want sqlite", settings.Driver)
	}
	for _, parameter := range []string{"_busy_timeout=5000", "_foreign_keys=on", "_journal_mode=WAL", "_synchronous=NORMAL"} {
		if !strings.Contains(settings.DSN, parameter) {
			t.Fatalf("DSN %q does not contain %q", settings.DSN, parameter)
		}
	}
}

func TestLegacyDatabaseConfigDefaultsToPostgres(t *testing.T) {
	fsys := fstest.MapFS{
		configFileName: {Data: []byte(`database:
  host: localhost
  port: 5432
  user: hosthalla
  password: hosthalla
  database: hosthalla
`)},
	}
	var cfg AppConfig
	if err := cfg.LoadFromFS(fsys); err != nil {
		t.Fatalf("LoadFromFS returned error: %v", err)
	}
	settings, err := cfg.Database.ConnectionSettings("", "")
	if err != nil {
		t.Fatalf("ConnectionSettings returned error: %v", err)
	}
	if settings.Driver != DatabaseDriverPostgres || !strings.HasPrefix(settings.DSN, "postgres://") {
		t.Fatalf("settings = %#v, want postgres", settings)
	}
}

func TestDatabaseConnectionValidation(t *testing.T) {
	tests := []DatabaseConfig{
		{Driver: "mysql"},
		{Driver: "sqlite"},
	}
	for _, database := range tests {
		if _, err := database.ConnectionSettings("", ""); err == nil {
			t.Fatalf("ConnectionSettings(%#v) returned nil error", database)
		}
	}
}

func TestGenerateDefaultConfigUsesSystemSQLitePath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "custom", configFileName)
	if err := GenerateDefaultConfig(configPath, false); err != nil {
		t.Fatalf("GenerateDefaultConfig returned error: %v", err)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	if !strings.Contains(string(content), "driver: sqlite") || !strings.Contains(string(content), "path: "+DefaultSQLitePath) {
		t.Fatalf("generated config does not contain sqlite defaults:\n%s", content)
	}
}
