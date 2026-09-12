package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

func Handler() http.Handler {
	// Get the dist directory from the embedded FS
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Fallback: serve a simple message if assets aren't embedded
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Web UI not built. Run 'make web' first.", http.StatusNotFound)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requestedPath == "." {
			requestedPath = "index.html"
		}
		if _, err := fs.Stat(sub, requestedPath); err != nil {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			fileServer.ServeHTTP(w, clone)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
