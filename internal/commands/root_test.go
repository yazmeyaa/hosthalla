package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/version"
)

func TestRootDoesNotExposeLegacyAliases(t *testing.T) {
	root := NewRoot(RootParams{})
	var stdout, stderr bytes.Buffer

	code := cliapp.Execute(context.Background(), root, nil, &stdout, &stderr, cliapp.Dependencies{})
	if code != cliapp.ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	help := stdout.String()
	for _, legacy := range []string{"  create-user", "  database"} {
		if strings.Contains(help, legacy) {
			t.Fatalf("root help contains legacy command %q:\n%s", legacy, help)
		}
	}

	for _, args := range [][]string{{"create-user"}, {"database", "up"}} {
		stdout.Reset()
		stderr.Reset()

		code := cliapp.Execute(context.Background(), root, args, &stdout, &stderr, cliapp.Dependencies{})
		if code != cliapp.ExitCodeUsage {
			t.Fatalf("args %v exit code = %d, want %d", args, code, cliapp.ExitCodeUsage)
		}
		if !strings.Contains(stderr.String(), "unknown command") {
			t.Fatalf("args %v stderr = %q", args, stderr.String())
		}
	}
}

func TestVersionCommandPrintsTagOnly(t *testing.T) {
	originalVersion := version.Version
	originalCommit := version.Commit
	t.Cleanup(func() {
		version.Version = originalVersion
		version.Commit = originalCommit
	})
	version.Version = "v1.2.3"
	version.Commit = "abc1234"

	root := NewRoot(RootParams{})
	var stdout, stderr bytes.Buffer

	code := cliapp.Execute(context.Background(), root, []string{"version"}, &stdout, &stderr, cliapp.Dependencies{})
	if code != cliapp.ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "v1.2.3\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
