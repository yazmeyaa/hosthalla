package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestLoadConfigsFromDir(t *testing.T) {
	dir := t.TempDir()
	idA, idB := uuid.New(), uuid.New()
	writeConfig(t, filepath.Join(dir, "z.yml"), idA, "https", "z.example")
	writeConfig(t, filepath.Join(dir, "a.yaml"), idB, "https", "a.example")
	writeConfig(t, filepath.Join(dir, "ignored.txt"), uuid.New(), "https", "ignored.example")
	if err := os.Mkdir(filepath.Join(dir, "nested.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}

	configs, err := LoadConfigsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 || configs[0].Name != "a" || configs[1].Name != "z" {
		t.Fatalf("loaded configs = %#v", configs)
	}
	for _, config := range configs {
		if !filepath.IsAbs(config.Path) || config.Config == nil {
			t.Fatalf("invalid loaded config: %#v", config)
		}
	}
}

func TestLoadConfigsFromDirReportsAllErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("agent_id: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "missing.yml"), []byte("agent_id: not-a-uuid\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigsFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "bad.yaml") || !strings.Contains(err.Error(), "missing.yml") {
		t.Fatalf("error = %v", err)
	}
	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) || len(joined.Unwrap()) != 2 {
		t.Fatalf("expected two joined errors, got %v", err)
	}
}

func TestLoadConfigsFromDirRejectsDuplicates(t *testing.T) {
	dir := t.TempDir()
	id := uuid.New()
	writeConfig(t, filepath.Join(dir, "first.yaml"), id, "HTTPS", "Example.COM")
	writeConfig(t, filepath.Join(dir, "second.yml"), id, "https", "example.com")

	_, err := LoadConfigsFromDir(dir)
	if err == nil || !strings.Contains(err.Error(), "first.yaml") || !strings.Contains(err.Error(), "second.yml") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigsFromDirRejectsEmptyDirectory(t *testing.T) {
	_, err := LoadConfigsFromDir(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "contains no") {
		t.Fatalf("error = %v", err)
	}
}

func TestSaveNewConfigToPathDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.yaml")
	original := []byte("existing")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := NewAgentConfig()
	cfg.AgentID = uuid.New()
	if err := SaveNewConfigToPath(path, cfg); err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("SaveNewConfigToPath() error = %v, want os.ErrExist", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("config contents = %q, want %q", got, original)
	}
}

func writeConfig(t *testing.T, path string, id uuid.UUID, scheme, host string) {
	t.Helper()
	raw := "agent_id: " + id.String() + "\nconnection:\n  scheme: " + scheme + "\n  host: " + host + "\n  api_key: key\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
}
