package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

const (
	profileSelectColumns = "id, username, created_at, updated_at"

	insertProfileQuery        = "insert into profile (username) values ($1) returning " + profileSelectColumns
	getProfileByIDQuery       = "select " + profileSelectColumns + " from profile where id = $1"
	getProfileByUsernameQuery = "select " + profileSelectColumns + " from profile where username = $1"
	updateProfileQuery        = "update profile set username = $2, updated_at = now() where id = $1 returning updated_at"
	deleteProfileQuery        = "delete from profile where id = $1"
)

func scanProfile(row pgx.Row) (authentication.Profile, error) {
	var profile authentication.Profile
	if err := row.Scan(
		&profile.ID,
		&profile.Username,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		return authentication.Profile{}, err
	}
	return profile, nil
}

type ProfileRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

// CreateProfile implements storage.ProfileRepository.
func (p *ProfileRepositoryPostgresImpl) CreateProfile(ctx context.Context, data storage.CreateProfileDTO) (authentication.Profile, error) {
	row := p.pool.QueryRow(ctx, insertProfileQuery, data.Username)
	return scanProfile(row)
}

// GetProfileByID implements storage.ProfileRepository.
func (p *ProfileRepositoryPostgresImpl) GetProfileByID(ctx context.Context, id string) (authentication.Profile, error) {
	row := p.pool.QueryRow(ctx, getProfileByIDQuery, id)
	return scanProfile(row)
}

// GetProfileByUsername implements storage.ProfileRepository.
func (p *ProfileRepositoryPostgresImpl) GetProfileByUsername(ctx context.Context, username string) (authentication.Profile, error) {
	row := p.pool.QueryRow(ctx, getProfileByUsernameQuery, username)
	return scanProfile(row)
}

// UpdateProfile implements storage.ProfileRepository.
func (p *ProfileRepositoryPostgresImpl) UpdateProfile(ctx context.Context, profile *authentication.Profile) error {
	row := p.pool.QueryRow(ctx, updateProfileQuery, profile.ID, profile.Username)
	if err := row.Scan(&profile.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("profile not found: %s", profile.ID)
		}
		return err
	}
	return nil
}

// DeleteProfile implements storage.ProfileRepository.
func (p *ProfileRepositoryPostgresImpl) DeleteProfile(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, deleteProfileQuery, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("profile not found: %s", id)
	}
	return nil
}

func NewProfileRepository(pool *pgxpool.Pool) *ProfileRepositoryPostgresImpl {
	return &ProfileRepositoryPostgresImpl{pool: pool}
}

var _ storage.ProfileRepository = &ProfileRepositoryPostgresImpl{}
