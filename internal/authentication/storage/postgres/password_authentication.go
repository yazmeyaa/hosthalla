package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

const (
	passwordAuthenticationSelectColumns          = "password_hash, created_at, updated_at"
	passwordAuthenticationSelectColumnsWithAlias = "pa.password_hash, pa.created_at, pa.updated_at"

	insertPasswordAuthenticationQuery        = "insert into password_authentication (profile_id, password_hash) values ($1, $2) returning " + passwordAuthenticationSelectColumns
	getPasswordAuthenticationByIDQuery       = "select " + passwordAuthenticationSelectColumns + " from password_authentication where id = $1"
	getPasswordAuthenticationByUsernameQuery = `
		select ` + passwordAuthenticationSelectColumnsWithAlias + `
		from password_authentication pa
		inner join profile p on p.id = pa.profile_id
		where p.username = $1
		order by pa.created_at desc
		limit 1`
)

func scanPasswordAuthentication(row pgx.Row) (authentication.PasswordAuthentication, error) {
	var auth authentication.PasswordAuthentication
	if err := row.Scan(
		&auth.PasswordHash,
		&auth.CreatedAt,
		&auth.UpdatedAt,
	); err != nil {
		return authentication.PasswordAuthentication{}, err
	}
	return auth, nil
}

type PasswordAuthenticationRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

func (p *PasswordAuthenticationRepositoryPostgresImpl) CreatePasswordAuthentication(ctx context.Context, data storage.CreatePasswordAuthenticationDTO) (authentication.PasswordAuthentication, error) {
	row := p.pool.QueryRow(ctx, insertPasswordAuthenticationQuery, data.ProfileID, data.PasswordHash)
	return scanPasswordAuthentication(row)
}

func (p *PasswordAuthenticationRepositoryPostgresImpl) GetPasswordAuthenticationByID(ctx context.Context, id string) (authentication.PasswordAuthentication, error) {
	row := p.pool.QueryRow(ctx, getPasswordAuthenticationByIDQuery, id)
	return scanPasswordAuthentication(row)
}

func (p *PasswordAuthenticationRepositoryPostgresImpl) GetPasswordAuthenticationByUsername(ctx context.Context, username string) (authentication.PasswordAuthentication, error) {
	row := p.pool.QueryRow(ctx, getPasswordAuthenticationByUsernameQuery, username)
	return scanPasswordAuthentication(row)
}

func NewPasswordAuthenticationRepository(pool *pgxpool.Pool) *PasswordAuthenticationRepositoryPostgresImpl {
	return &PasswordAuthenticationRepositoryPostgresImpl{pool: pool}
}

var _ storage.PasswordAuthenticationRepository = &PasswordAuthenticationRepositoryPostgresImpl{}
