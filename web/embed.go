// Package web embeds the built frontend into the binary (design §3.2).
// web/dist is gitignored; CI and builds create a placeholder index.html
// when the real frontend build is absent.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// DistFS returns the embedded frontend rooted at dist/.
func DistFS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
