//go:build frontend

package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

// FrontendDirectory - name of frontend directory.
const FrontendDirectory = "dist"

//go:generate ./build-frontend dist
//go:embed dist
var staticFS embed.FS

func init() {
	subfs, err := fs.Sub(staticFS, FrontendDirectory)
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServerFS(subfs))
	version = fmt.Sprintf("%s (frontend)", version)
}
