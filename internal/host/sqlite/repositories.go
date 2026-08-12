package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type Repositories struct {
	Host                 host.HostRepository
	HostManagementMethod host.HostManagementMethodRepository
	HostSystemInfo       host.HostSystemInfoRepository
	HostMetricSnapshot   host.HostMetricSnapshotRepository
}

func NewRepositories(db *sql.DB) Repositories {
	return Repositories{
		Host:                 NewHostRepository(db),
		HostManagementMethod: NewHostManagementMethodRepository(db),
		HostSystemInfo:       NewHostSystemInfoRepository(db),
		HostMetricSnapshot:   NewHostMetricSnapshotRepository(db),
	}
}

type scanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func placeholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func uuidStrings(values []uuid.UUID) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value.String()
	}
	return result
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

func parseUUID(raw, field string) (uuid.UUID, error) {
	value, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse %s: %w", field, err)
	}
	return value, nil
}
