// Package web holds the GUI frontend (Deltarune-style, zero build), embedded
// into the GUI binary and served by internal/server.
package web

import "embed"

//go:embed index.html app.js style.css fonts assets config-features.json
var FS embed.FS
