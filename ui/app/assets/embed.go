package assets

import "embed"

// Files contains embedded frontend asset files.
//
//go:embed *.js *.svg *.txt fonts/*.ttf static/*.png
var Files embed.FS
