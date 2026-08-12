package repository

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
)

// NormalizeError keeps repository consumers independent from database drivers.
func NormalizeError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}
