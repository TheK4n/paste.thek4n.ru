//go:build frontend

package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

// frontendDirectory name of frontend directory.
const frontendDirectory = "dist"

//go:generate ./build-frontend dist
//go:embed dist/frontend/assets
var assetsFS embed.FS

//go:embed dist/index.html
var indexFS embed.FS

func init() {
	assetsFS, err := fs.Sub(assetsFS, fmt.Sprintf("%s/%s/%s", frontendDirectory, "frontend", "assets"))
	if err != nil {
		panic(err)
	}
	indexFS, err := fs.Sub(indexFS, frontendDirectory)
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /frontend/assets/", http.StripPrefix("/frontend/assets", http.FileServerFS(assetsFS)))
	mux.Handle("GET /{$}", http.FileServerFS(indexFS))
}
