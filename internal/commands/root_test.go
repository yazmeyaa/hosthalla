package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
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
