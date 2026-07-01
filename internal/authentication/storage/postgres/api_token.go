package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

const (
	apiTokenSelectColumns = "id, profile_id, name, prefix, hash, scopes, last_used_at, created_at, expires_at, revoked_at"

	insertAPITokenQuery           = "insert into api_token (profile_id, name, prefix, hash, scopes, expires_at) values ($1, $2, $3, $4, $5, $6) returning " + apiTokenSelectColumns
	getAPITokenByIDQuery          = "select " + apiTokenSelectColumns + " from api_token where id = $1"
	getAPITokenByHashQuery        = "select " + apiTokenSelectColumns + " from api_token where hash = $1"
	listAPITokensQuery            = "select " + apiTokenSelectColumns + " from api_token order by created_at desc"
	listAPITokensByProfileIDQuery = "select " + apiTokenSelectColumns + " from api_token where profile_id = $1 order by created_at desc"
	revokeAPITokenQuery           = "update api_token set revoked_at = now() where id = $1 and revoked_at is null"
	updateLastUsedAtQuery         = "update api_token set last_used_at = $2 where id = $1"
)

func scanAPIToken(row pgx.Row) (authentication.APIToken, error) {
	var token authentication.APIToken
	if err := row.Scan(
		&token.ID,
		&token.ProfileID,
		&token.Name,
		&token.Prefix,
		&token.Hash,
		&token.Scopes,
		&token.LastUsedAt,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.RevokedAt,
	); err != nil {
		return authentication.APIToken{}, err
	}
	return token, nil
}

type APITokenRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

func (r *APITokenRepositoryPostgresImpl) CreateAPIToken(ctx context.Context, data storage.CreateAPITokenDTO) (authentication.APIToken, error) {
	row := r.pool.QueryRow(ctx, insertAPITokenQuery, data.ProfileID, data.Name, data.Prefix, data.Hash, data.Scopes, data.ExpiresAt)
	return scanAPIToken(row)
}

func (r *APITokenRepositoryPostgresImpl) GetAPITokenByID(ctx context.Context, id string) (authentication.APIToken, error) {
	row := r.pool.QueryRow(ctx, getAPITokenByIDQuery, id)
	return scanAPIToken(row)
}

func (r *APITokenRepositoryPostgresImpl) GetAPITokenByHash(ctx context.Context, hash string) (authentication.APIToken, error) {
	row := r.pool.QueryRow(ctx, getAPITokenByHashQuery, hash)
	return scanAPIToken(row)
}

func (r *APITokenRepositoryPostgresImpl) ListAPITokens(ctx context.Context) ([]authentication.APIToken, error) {
	rows, err := r.pool.Query(ctx, listAPITokensQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]authentication.APIToken, 0)
	for rows.Next() {
		token, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, token)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *APITokenRepositoryPostgresImpl) ListAPITokensByProfileID(ctx context.Context, profileID string) ([]authentication.APIToken, error) {
	rows, err := r.pool.Query(ctx, listAPITokensByProfileIDQuery, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]authentication.APIToken, 0)
	for rows.Next() {
		token, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, token)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *APITokenRepositoryPostgresImpl) RevokeAPIToken(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, revokeAPITokenQuery, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("api token not found or already revoked: %s", id)
	}
	return nil
}

func (r *APITokenRepositoryPostgresImpl) UpdateLastUsedAt(ctx context.Context, id string, lastUsedAt time.Time) error {
	tag, err := r.pool.Exec(ctx, updateLastUsedAtQuery, id, lastUsedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("api token not found: %s", id)
	}
	return nil
}

func NewAPITokenRepository(pool *pgxpool.Pool) *APITokenRepositoryPostgresImpl {
	return &APITokenRepositoryPostgresImpl{pool: pool}
}

var _ storage.APITokenRepository = &APITokenRepositoryPostgresImpl{}
