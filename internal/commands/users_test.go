package commands

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	cliapp "github.com/yazmeyaa/hosthalla/internal/cli"
	"github.com/yazmeyaa/hosthalla/internal/config"
)

type fakeUserCreator struct {
	got auth_service.CreateUserDTO
}

func (c *fakeUserCreator) CreateUser(ctx context.Context, data auth_service.CreateUserDTO) (authentication.Profile, error) {
	c.got = data
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	return authentication.Profile{
		ID:        "user-1",
		Username:  data.Username,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func TestUsersCreateUsesPreparedDBAndService(t *testing.T) {
	oldNewUserCreator := newUserCreator
	defer func() {
		newUserCreator = oldNewUserCreator
	}()

	creator := &fakeUserCreator{}
	newUserCreator = func(pool *pgxpool.Pool) userCreator {
		return creator
	}

	cfg := config.NewDefaultAppConfig()
	var openedDB bool
	deps := cliapp.Dependencies{
		LoadConfig: func(path string) (*config.AppConfig, error) {
			return &cfg, nil
		},
		OpenDB: func(ctx context.Context, cfg *config.AppConfig) (*pgxpool.Pool, error) {
			openedDB = true
			return nil, nil
		},
		NewLogger: func(output io.Writer, level slog.Level) *slog.Logger {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		},
	}

	root := NewRoot(RootParams{})
	var stdout, stderr bytes.Buffer
	code := cliapp.Execute(context.Background(), root, []string{"users", "create", "alice", "correct horse battery staple"}, &stdout, &stderr, deps)

	if code != cliapp.ExitCodeOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !openedDB {
		t.Fatal("DB was not opened")
	}
	if creator.got.Username != "alice" || creator.got.Password != "correct horse battery staple" {
		t.Fatalf("CreateUser DTO = %#v", creator.got)
	}
	if !strings.Contains(stdout.String(), "User created: alice") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
