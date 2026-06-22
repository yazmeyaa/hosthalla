package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/host/storage"
)

const hostNoteSelectColumns = "id, host_id, title, body, created_at, updated_at"

func scanHostNote(row pgx.Row) (host.HostNote, error) {
	var result host.HostNote
	if err := row.Scan(
		&result.ID,
		&result.HostID,
		&result.Title,
		&result.Body,
		&result.CreatedAt,
		&result.UpdatedAt,
	); err != nil {
		return host.HostNote{}, err
	}
	return result, nil
}

type HostNoteRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

// CreateHostNote implements storage.HostNoteRepository.
func (h HostNoteRepositoryPostgresImpl) CreateHostNote(ctx context.Context, hostID host.HostID, data storage.CreateHostNoteDTO) (host.HostNote, error) {
	const insertHostNoteQuery = "insert into host_note (host_id, title, body) values ($1, $2, $3) returning id, host_id, title, body, created_at, updated_at"
	row := h.pool.QueryRow(ctx, insertHostNoteQuery, uuid.UUID(hostID), data.Title, data.Body)
	return scanHostNote(row)
}

// DeleteHostNote implements storage.HostNoteRepository.
func (h HostNoteRepositoryPostgresImpl) DeleteHostNote(ctx context.Context, hostNoteID host.HostNoteID) error {
	const deleteHostNoteQuery = "delete from host_note where id = $1"
	tag, err := h.pool.Exec(ctx, deleteHostNoteQuery, uuid.UUID(hostNoteID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("host note not found: %s", uuid.UUID(hostNoteID))
	}
	return nil
}

// GetHostNodeByID implements storage.HostNoteRepository.
func (h HostNoteRepositoryPostgresImpl) GetHostNodeByID(ctx context.Context, hostNoteID host.HostNoteID) (host.HostNote, error) {
	query := "select " + hostNoteSelectColumns + " from host_note where id = $1"
	row := h.pool.QueryRow(ctx, query, uuid.UUID(hostNoteID))
	return scanHostNote(row)
}

// ListHostNotes implements storage.HostNoteRepository.
func (h HostNoteRepositoryPostgresImpl) ListHostNotes(ctx context.Context, hostID host.HostID) ([]host.HostNote, error) {
	query := "select " + hostNoteSelectColumns + " from host_note where host_id = $1 order by created_at desc"
	rows, err := h.pool.Query(ctx, query, uuid.UUID(hostID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []host.HostNote
	for rows.Next() {
		note, err := scanHostNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notes, nil
}

// UpdateHostNote implements storage.HostNoteRepository.
func (h HostNoteRepositoryPostgresImpl) UpdateHostNote(ctx context.Context, hostNote *host.HostNote) error {
	const updateHostNoteQuery = "update host_note set title = $2, body = $3, updated_at = now() where id = $1 returning updated_at"
	row := h.pool.QueryRow(ctx, updateHostNoteQuery, hostNote.ID, hostNote.Title, hostNote.Body)
	if err := row.Scan(&hostNote.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("host note not found: %s", hostNote.ID)
		}
		return err
	}
	return nil
}

func NewHostNoteRepository(pool *pgxpool.Pool) HostNoteRepositoryPostgresImpl {
	return HostNoteRepositoryPostgresImpl{pool}
}

var _ storage.HostNoteRepository = HostNoteRepositoryPostgresImpl{}
