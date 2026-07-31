package assets

import "embed"

// Files contains embedded frontend asset files.
//
//go:embed *.js fonts/*.ttf static/*.png
var Files embed.FS
