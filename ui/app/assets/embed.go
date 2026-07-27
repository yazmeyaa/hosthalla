package assets

import "embed"

// Files contains embedded frontend asset files.
//
//go:embed *.js fonts/*.ttf
var Files embed.FS
