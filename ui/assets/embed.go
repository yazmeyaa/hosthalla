package assets

import "embed"

// Files contains embedded frontend asset files.
//
//go:embed *.js
var Files embed.FS
