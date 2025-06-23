package server

import "embed"

//go:embed templates/*.html
var tmplFS embed.FS
