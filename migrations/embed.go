package migrations

import "embed"

// Files contains embedded SQL migration files.
//
//go:embed postgres/*.sql sqlite/*.sql
var Files embed.FS
