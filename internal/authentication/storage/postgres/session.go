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
	sessionSelectColumns = "id, profile_id, created_at, updated_at"

	insertSessionQuery         = "insert into session (profile_id) values ($1) returning " + sessionSelectColumns
	getSessionByIDQuery        = "select " + sessionSelectColumns + " from session where id = $1"
	getSessionByProfileIDQuery = "select " + sessionSelectColumns + " from session where profile_id = $1 order by created_at desc limit 1"
	deleteSessionQuery         = "delete from session where id = $1"
)

func scanSession(row pgx.Row) (authentication.Session, error) {
	var session authentication.Session
	if err := row.Scan(
		&session.ID,
		&session.ProfileID,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return authentication.Session{}, err
	}
	return session, nil
}

type SessionRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

// CreateSession implements storage.SessionRepository.
func (s *SessionRepositoryPostgresImpl) CreateSession(ctx context.Context, data storage.CreateSessionDTO) (authentication.Session, error) {
	row := s.pool.QueryRow(ctx, insertSessionQuery, data.ProfileID)
	return scanSession(row)
}

// GetSessionByID implements storage.SessionRepository.
func (s *SessionRepositoryPostgresImpl) GetSessionByID(ctx context.Context, id string) (authentication.Session, error) {
	row := s.pool.QueryRow(ctx, getSessionByIDQuery, id)
	return scanSession(row)
}

// GetSessionByProfileID implements storage.SessionRepository.
func (s *SessionRepositoryPostgresImpl) GetSessionByProfileID(ctx context.Context, profileID string) (authentication.Session, error) {
	row := s.pool.QueryRow(ctx, getSessionByProfileIDQuery, profileID)
	return scanSession(row)
}

func (s *SessionRepositoryPostgresImpl) DeleteSession(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, deleteSessionQuery, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("session not found: %s", id)
	}
	return nil
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepositoryPostgresImpl {
	return &SessionRepositoryPostgresImpl{pool: pool}
}

var _ storage.SessionRepository = &SessionRepositoryPostgresImpl{}
