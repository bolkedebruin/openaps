// Package web embeds the built Lit single-page app so the entire UI
// ships inside the ecu-web binary — nothing to deploy alongside it on
// the offline ECU. The dist/ tree is produced by `bun run build`
// (see web/build.ts); do not edit dist/ by hand.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built SPA rooted at the dist/ directory.
func Assets() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
