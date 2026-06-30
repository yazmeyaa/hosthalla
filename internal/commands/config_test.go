package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
)

func TestConfigGenerateAndShow(t *testing.T) {
	root := NewRoot(RootParams{})
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	var stdout, stderr bytes.Buffer

	code := cliapp.Execute(context.Background(), root, []string{"config", "generate", "--path", configPath}, &stdout, &stderr, cliapp.Dependencies{})
	if code != cliapp.ExitCodeOK {
		t.Fatalf("generate exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Config generated") {
		t.Fatalf("generate stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = cliapp.Execute(context.Background(), root, []string{"config", "show", "--path", configPath}, &stdout, &stderr, cliapp.Dependencies{})
	if code != cliapp.ExitCodeOK {
		t.Fatalf("show exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "database:") || !strings.Contains(stdout.String(), "secret_encryption_key:") {
		t.Fatalf("show stdout = %q", stdout.String())
	}
}

func TestConfigValidate(t *testing.T) {
	root := NewRoot(RootParams{})
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	var stdout, stderr bytes.Buffer

	code := cliapp.Execute(context.Background(), root, []string{"config", "generate", "--path", configPath}, &stdout, &stderr, cliapp.Dependencies{})
	if code != cliapp.ExitCodeOK {
		t.Fatalf("generate exit code = %d, stderr = %q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = cliapp.Execute(context.Background(), root, []string{"config", "validate", "--path", configPath}, &stdout, &stderr, cliapp.Dependencies{})
	if code != cliapp.ExitCodeOK {
		t.Fatalf("validate exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "is valid") {
		t.Fatalf("validate stdout = %q", stdout.String())
	}
}
