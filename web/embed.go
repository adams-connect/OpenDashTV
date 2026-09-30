package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns an fs.FS rooted at the compiled web/dist directory.
// This allows the single compiled Go binary to serve the complete dashboard
// and setup portal without any external HTML/JS/CSS file dependencies on disk.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return distFS
	}
	return sub
}
