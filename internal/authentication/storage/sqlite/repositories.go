package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

type scanner interface {
	Scan(dest ...any) error
}

type ProfileRepositorySQLiteImpl struct{ db *sql.DB }

func NewProfileRepository(db *sql.DB) *ProfileRepositorySQLiteImpl {
	return &ProfileRepositorySQLiteImpl{db: db}
}

func (r *ProfileRepositorySQLiteImpl) CreateProfile(ctx context.Context, data storage.CreateProfileDTO) (authentication.Profile, error) {
	now := time.Now().UTC()
	value := authentication.Profile{ID: uuid.NewString(), Username: data.Username, CreatedAt: now, UpdatedAt: now}
	_, err := r.db.ExecContext(ctx, `insert into profile (id, username, created_at, updated_at) values (?, ?, ?, ?)`, value.ID, value.Username, value.CreatedAt, value.UpdatedAt)
	return value, err
}

func (r *ProfileRepositorySQLiteImpl) ListProfiles(ctx context.Context) ([]authentication.Profile, error) {
	rows, err := r.db.QueryContext(ctx, `select id, username, created_at, updated_at from profile order by created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]authentication.Profile, 0)
	for rows.Next() {
		value, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *ProfileRepositorySQLiteImpl) GetProfileByID(ctx context.Context, id string) (authentication.Profile, error) {
	return scanProfile(r.db.QueryRowContext(ctx, `select id, username, created_at, updated_at from profile where id = ?`, id))
}

func (r *ProfileRepositorySQLiteImpl) GetProfileByUsername(ctx context.Context, username string) (authentication.Profile, error) {
	return scanProfile(r.db.QueryRowContext(ctx, `select id, username, created_at, updated_at from profile where username = ?`, username))
}

func (r *ProfileRepositorySQLiteImpl) UpdateProfile(ctx context.Context, value *authentication.Profile) error {
	updatedAt := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `update profile set username = ?, updated_at = ? where id = ?`, value.Username, updatedAt, value.ID)
	if err != nil {
		return err
	}
	if err := requireAffected(result, fmt.Sprintf("profile not found: %s", value.ID)); err != nil {
		return err
	}
	value.UpdatedAt = updatedAt
	return nil
}

func (r *ProfileRepositorySQLiteImpl) DeleteProfile(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `delete from profile where id = ?`, id)
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("profile not found: %s", id))
}

func scanProfile(row scanner) (authentication.Profile, error) {
	var value authentication.Profile
	err := row.Scan(&value.ID, &value.Username, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

type PasswordAuthenticationRepositorySQLiteImpl struct{ db *sql.DB }

func NewPasswordAuthenticationRepository(db *sql.DB) *PasswordAuthenticationRepositorySQLiteImpl {
	return &PasswordAuthenticationRepositorySQLiteImpl{db: db}
}

func (r *PasswordAuthenticationRepositorySQLiteImpl) CreatePasswordAuthentication(ctx context.Context, data storage.CreatePasswordAuthenticationDTO) (authentication.PasswordAuthentication, error) {
	now := time.Now().UTC()
	value := authentication.PasswordAuthentication{PasswordHash: data.PasswordHash, CreatedAt: now, UpdatedAt: now}
	_, err := r.db.ExecContext(ctx, `insert into password_authentication (id, profile_id, password_hash, created_at, updated_at) values (?, ?, ?, ?, ?)`, uuid.NewString(), data.ProfileID, data.PasswordHash, now, now)
	return value, err
}

func (r *PasswordAuthenticationRepositorySQLiteImpl) GetPasswordAuthenticationByID(ctx context.Context, id string) (authentication.PasswordAuthentication, error) {
	return scanPasswordAuthentication(r.db.QueryRowContext(ctx, `select password_hash, created_at, updated_at from password_authentication where id = ?`, id))
}

func (r *PasswordAuthenticationRepositorySQLiteImpl) GetPasswordAuthenticationByUsername(ctx context.Context, username string) (authentication.PasswordAuthentication, error) {
	return scanPasswordAuthentication(r.db.QueryRowContext(ctx, `
select pa.password_hash, pa.created_at, pa.updated_at
from password_authentication pa
join profile p on p.id = pa.profile_id
where p.username = ?
order by pa.created_at desc
limit 1`, username))
}

func scanPasswordAuthentication(row scanner) (authentication.PasswordAuthentication, error) {
	var value authentication.PasswordAuthentication
	err := row.Scan(&value.PasswordHash, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

type SessionRepositorySQLiteImpl struct{ db *sql.DB }

func NewSessionRepository(db *sql.DB) *SessionRepositorySQLiteImpl {
	return &SessionRepositorySQLiteImpl{db: db}
}

func (r *SessionRepositorySQLiteImpl) CreateSession(ctx context.Context, data storage.CreateSessionDTO) (authentication.Session, error) {
	now := time.Now().UTC()
	value := authentication.Session{ID: uuid.NewString(), ProfileID: data.ProfileID, CreatedAt: now, UpdatedAt: now}
	_, err := r.db.ExecContext(ctx, `insert into session (id, profile_id, created_at, updated_at) values (?, ?, ?, ?)`, value.ID, value.ProfileID, now, now)
	return value, err
}

func (r *SessionRepositorySQLiteImpl) GetSessionByID(ctx context.Context, id string) (authentication.Session, error) {
	return scanSession(r.db.QueryRowContext(ctx, `select id, profile_id, created_at, updated_at from session where id = ?`, id))
}

func (r *SessionRepositorySQLiteImpl) GetSessionByProfileID(ctx context.Context, profileID string) (authentication.Session, error) {
	return scanSession(r.db.QueryRowContext(ctx, `select id, profile_id, created_at, updated_at from session where profile_id = ? order by created_at desc limit 1`, profileID))
}

func (r *SessionRepositorySQLiteImpl) DeleteSession(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `delete from session where id = ?`, id)
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("session not found: %s", id))
}

func scanSession(row scanner) (authentication.Session, error) {
	var value authentication.Session
	err := row.Scan(&value.ID, &value.ProfileID, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

type APITokenRepositorySQLiteImpl struct{ db *sql.DB }

func NewAPITokenRepository(db *sql.DB) *APITokenRepositorySQLiteImpl {
	return &APITokenRepositorySQLiteImpl{db: db}
}

func (r *APITokenRepositorySQLiteImpl) CreateAPIToken(ctx context.Context, data storage.CreateAPITokenDTO) (authentication.APIToken, error) {
	scopes, err := json.Marshal(data.Scopes)
	if err != nil {
		return authentication.APIToken{}, err
	}
	value := authentication.APIToken{
		ID: uuid.NewString(), ProfileID: data.ProfileID, Name: data.Name, Prefix: data.Prefix,
		Hash: data.Hash, Scopes: append([]string(nil), data.Scopes...), CreatedAt: time.Now().UTC(), ExpiresAt: data.ExpiresAt,
	}
	_, err = r.db.ExecContext(ctx, `
insert into api_token (id, profile_id, name, prefix, hash, scopes, created_at, expires_at)
values (?, ?, ?, ?, ?, ?, ?, ?)`, value.ID, value.ProfileID, value.Name, value.Prefix, value.Hash, string(scopes), value.CreatedAt, value.ExpiresAt)
	return value, err
}

func (r *APITokenRepositorySQLiteImpl) GetAPITokenByID(ctx context.Context, id string) (authentication.APIToken, error) {
	return scanAPIToken(r.db.QueryRowContext(ctx, apiTokenSelect+` where id = ?`, id))
}

func (r *APITokenRepositorySQLiteImpl) GetAPITokenByHash(ctx context.Context, hash string) (authentication.APIToken, error) {
	return scanAPIToken(r.db.QueryRowContext(ctx, apiTokenSelect+` where hash = ?`, hash))
}

func (r *APITokenRepositorySQLiteImpl) ListAPITokens(ctx context.Context) ([]authentication.APIToken, error) {
	return r.list(ctx, apiTokenSelect+` order by created_at desc`)
}

func (r *APITokenRepositorySQLiteImpl) ListAPITokensByProfileID(ctx context.Context, profileID string) ([]authentication.APIToken, error) {
	return r.list(ctx, apiTokenSelect+` where profile_id = ? order by created_at desc`, profileID)
}

func (r *APITokenRepositorySQLiteImpl) list(ctx context.Context, query string, args ...any) ([]authentication.APIToken, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]authentication.APIToken, 0)
	for rows.Next() {
		value, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *APITokenRepositorySQLiteImpl) RevokeAPIToken(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `update api_token set revoked_at = ? where id = ? and revoked_at is null`, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("api token not found or already revoked: %s", id))
}

func (r *APITokenRepositorySQLiteImpl) UpdateLastUsedAt(ctx context.Context, id string, lastUsedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `update api_token set last_used_at = ? where id = ?`, lastUsedAt, id)
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("api token not found: %s", id))
}

const apiTokenSelect = `select id, profile_id, name, prefix, hash, scopes, last_used_at, created_at, expires_at, revoked_at from api_token`

func scanAPIToken(row scanner) (authentication.APIToken, error) {
	var value authentication.APIToken
	var scopes string
	if err := row.Scan(&value.ID, &value.ProfileID, &value.Name, &value.Prefix, &value.Hash, &scopes, &value.LastUsedAt, &value.CreatedAt, &value.ExpiresAt, &value.RevokedAt); err != nil {
		return authentication.APIToken{}, err
	}
	if err := json.Unmarshal([]byte(scopes), &value.Scopes); err != nil {
		return authentication.APIToken{}, fmt.Errorf("decode api token scopes: %w", err)
	}
	return value, nil
}

func requireAffected(result sql.Result, message string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%s: %w", message, sql.ErrNoRows)
	}
	return nil
}

var (
	_ storage.ProfileRepository                = (*ProfileRepositorySQLiteImpl)(nil)
	_ storage.PasswordAuthenticationRepository = (*PasswordAuthenticationRepositorySQLiteImpl)(nil)
	_ storage.SessionRepository                = (*SessionRepositorySQLiteImpl)(nil)
	_ storage.APITokenRepository               = (*APITokenRepositorySQLiteImpl)(nil)
)
