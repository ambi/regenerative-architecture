package ui

import (
	"embed"
	"io/fs"
)

// Assets contains the production Vite bundle embedded into the Go binary.
//
//go:embed dist/assets/app.css dist/assets/app.js
var assets embed.FS

func ReadAsset(name string) ([]byte, error) {
	return fs.ReadFile(assets, "dist/assets/"+name)
}
