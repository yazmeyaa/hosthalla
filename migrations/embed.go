package migrations

import "embed"

// Files contains embedded SQL migration files.
//
//go:embed *.sql
var Files embed.FS
