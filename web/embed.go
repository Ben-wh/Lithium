// Package web embeds the built SvelteKit frontend into the Go binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var build embed.FS

// FS returns the built frontend rooted at its output directory.
func FS() fs.FS {
	sub, err := fs.Sub(build, "build")
	if err != nil {
		panic(err)
	}
	return sub
}
