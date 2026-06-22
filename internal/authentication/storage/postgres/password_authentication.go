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

func scanPasswordAuthentication(row pgx.Row) (authentication.PasswordAuthentincation, error) {
	var auth authentication.PasswordAuthentincation
	if err := row.Scan(
		&auth.PasswordHash,
		&auth.CreatedAt,
		&auth.UpdatedAt,
	); err != nil {
		return authentication.PasswordAuthentincation{}, err
	}
	return auth, nil
}

type PasswordAuthenticationRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

// CreatePasswordAuthentication implements storage.PasswordAuthenticationRepository.
func (p *PasswordAuthenticationRepositoryPostgresImpl) CreatePasswordAuthentication(ctx context.Context, data storage.CreatePasswordAuthenticationDTO) (authentication.PasswordAuthentincation, error) {
	row := p.pool.QueryRow(ctx, insertPasswordAuthenticationQuery, data.ProfileID, data.PasswordHash)
	return scanPasswordAuthentication(row)
}

// GetPasswordAuthenticationByID implements storage.PasswordAuthenticationRepository.
func (p *PasswordAuthenticationRepositoryPostgresImpl) GetPasswordAuthenticationByID(ctx context.Context, id string) (authentication.PasswordAuthentincation, error) {
	row := p.pool.QueryRow(ctx, getPasswordAuthenticationByIDQuery, id)
	return scanPasswordAuthentication(row)
}

// GetPasswordAuthenticationByUsername implements storage.PasswordAuthenticationRepository.
func (p *PasswordAuthenticationRepositoryPostgresImpl) GetPasswordAuthenticationByUsername(ctx context.Context, username string) (authentication.PasswordAuthentincation, error) {
	row := p.pool.QueryRow(ctx, getPasswordAuthenticationByUsernameQuery, username)
	return scanPasswordAuthentication(row)
}

func NewPasswordAuthenticationRepository(pool *pgxpool.Pool) *PasswordAuthenticationRepositoryPostgresImpl {
	return &PasswordAuthenticationRepositoryPostgresImpl{pool: pool}
}

var _ storage.PasswordAuthenticationRepository = &PasswordAuthenticationRepositoryPostgresImpl{}
