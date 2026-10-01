package web

import "embed"

// Assets contains the embedded web UI files.
//
//go:embed *.html *.js *.css
var Assets embed.FS
