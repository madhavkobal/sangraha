// Package web embeds the pre-built web dashboard static assets into the binary
// using go:embed. The dashboard is built from web/ and the output lands in
// web/dist/.
//
// During Phase 1 the web/dist directory contains only a .gitkeep placeholder.
// The full React SPA is implemented in Phase 2.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded web dashboard.
// Requests for unknown paths fall through to index.html for SPA routing.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// This can only fail if the embed path is wrong — panic at startup.
		panic("web: failed to sub dist FS: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
