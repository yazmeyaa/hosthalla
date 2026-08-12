package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
	authsqlite "github.com/yazmeyaa/hosthalla/internal/authentication/storage/sqlite"
	"github.com/yazmeyaa/hosthalla/internal/testsqlite"
)

func TestAuthenticationRepositories(t *testing.T) {
	ctx := context.Background()
	db := testsqlite.Open(t)
	profiles := authsqlite.NewProfileRepository(db)
	passwords := authsqlite.NewPasswordAuthenticationRepository(db)
	sessions := authsqlite.NewSessionRepository(db)
	tokens := authsqlite.NewAPITokenRepository(db)

	profile, err := profiles.CreateProfile(ctx, storage.CreateProfileDTO{Username: "alice"})
	must(t, err)
	if profile.ID == "" || profile.CreatedAt.IsZero() || profile.UpdatedAt.IsZero() {
		t.Fatalf("incomplete profile: %+v", profile)
	}
	byID, err := profiles.GetProfileByID(ctx, profile.ID)
	must(t, err)
	byUsername, err := profiles.GetProfileByUsername(ctx, "alice")
	must(t, err)
	if byID.ID != profile.ID || byUsername.ID != profile.ID {
		t.Fatal("profile lookup returned another profile")
	}
	listed, err := profiles.ListProfiles(ctx)
	must(t, err)
	if len(listed) != 1 {
		t.Fatalf("profiles count = %d", len(listed))
	}
	profile.Username = "alice-updated"
	must(t, profiles.UpdateProfile(ctx, &profile))
	if profile.UpdatedAt.Before(profile.CreatedAt) {
		t.Fatal("profile updated_at was not refreshed")
	}
	if _, err := profiles.CreateProfile(ctx, storage.CreateProfileDTO{Username: profile.Username}); err == nil {
		t.Fatal("expected unique username error")
	}

	password, err := passwords.CreatePasswordAuthentication(ctx, storage.CreatePasswordAuthenticationDTO{ProfileID: profile.ID, PasswordHash: "hash-1"})
	must(t, err)
	if password.PasswordHash != "hash-1" {
		t.Fatalf("password hash = %q", password.PasswordHash)
	}
	byUsernamePassword, err := passwords.GetPasswordAuthenticationByUsername(ctx, profile.Username)
	must(t, err)
	if byUsernamePassword.PasswordHash != password.PasswordHash {
		t.Fatal("password lookup returned another record")
	}
	var passwordID string
	must(t, db.QueryRowContext(ctx, `select id from password_authentication where profile_id = ?`, profile.ID).Scan(&passwordID))
	byPasswordID, err := passwords.GetPasswordAuthenticationByID(ctx, passwordID)
	must(t, err)
	if byPasswordID.PasswordHash != password.PasswordHash {
		t.Fatal("password id lookup returned another record")
	}

	session, err := sessions.CreateSession(ctx, storage.CreateSessionDTO{ProfileID: profile.ID})
	must(t, err)
	bySessionID, err := sessions.GetSessionByID(ctx, session.ID)
	must(t, err)
	latestSession, err := sessions.GetSessionByProfileID(ctx, profile.ID)
	must(t, err)
	if bySessionID.ID != session.ID || latestSession.ID != session.ID {
		t.Fatal("session lookup returned another record")
	}
	must(t, sessions.DeleteSession(ctx, session.ID))
	if _, err := sessions.GetSessionByID(ctx, session.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted session error = %v", err)
	}

	expiresAt := time.Now().UTC().Add(time.Hour)
	token, err := tokens.CreateAPIToken(ctx, storage.CreateAPITokenDTO{
		ProfileID: profile.ID, Name: "agent", Prefix: "12345678", Hash: "token-hash",
		Scopes: []string{"hosts:register", "metrics:write"}, ExpiresAt: &expiresAt,
	})
	must(t, err)
	byTokenID, err := tokens.GetAPITokenByID(ctx, token.ID)
	must(t, err)
	byTokenHash, err := tokens.GetAPITokenByHash(ctx, token.Hash)
	must(t, err)
	if byTokenID.ID != token.ID || byTokenHash.ID != token.ID || len(byTokenID.Scopes) != 2 {
		t.Fatalf("unexpected token lookup: %+v", byTokenID)
	}
	allTokens, err := tokens.ListAPITokens(ctx)
	must(t, err)
	profileTokens, err := tokens.ListAPITokensByProfileID(ctx, profile.ID)
	must(t, err)
	if len(allTokens) != 1 || len(profileTokens) != 1 {
		t.Fatalf("token list counts = %d, %d", len(allTokens), len(profileTokens))
	}
	lastUsedAt := time.Now().UTC()
	must(t, tokens.UpdateLastUsedAt(ctx, token.ID, lastUsedAt))
	must(t, tokens.RevokeAPIToken(ctx, token.ID))
	revoked, err := tokens.GetAPITokenByID(ctx, token.ID)
	must(t, err)
	if revoked.LastUsedAt == nil || revoked.RevokedAt == nil {
		t.Fatalf("token timestamps were not updated: %+v", revoked)
	}
	if err := tokens.RevokeAPIToken(ctx, token.ID); err == nil {
		t.Fatal("expected second revoke to fail")
	}

	if _, err := profiles.GetProfileByID(ctx, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing profile error = %v", err)
	}
	if _, err := passwords.CreatePasswordAuthentication(ctx, storage.CreatePasswordAuthenticationDTO{ProfileID: "missing", PasswordHash: "hash"}); err == nil {
		t.Fatal("expected foreign key error")
	}
	must(t, profiles.DeleteProfile(ctx, profile.ID))
	for _, table := range []string{"password_authentication", "api_token"} {
		var count int
		must(t, db.QueryRowContext(ctx, `select count(*) from `+table).Scan(&count))
		if count != 0 {
			t.Fatalf("%s cascade count = %d", table, count)
		}
	}
	if err := profiles.DeleteProfile(ctx, profile.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second profile delete error = %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
