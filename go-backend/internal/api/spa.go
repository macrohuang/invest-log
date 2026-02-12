package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// WithSPA wraps API handler with SPA static serving.
func WithSPA(apiHandler http.Handler, webDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(webDir))
	indexPath := filepath.Join(webDir, "index.html")
	nonCacheFileServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSPACacheControl(w)
		fileServer.ServeHTTP(w, r)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		cleanPath := path.Clean("/" + r.URL.Path)
		cleanPath = strings.TrimPrefix(cleanPath, "/")
		if cleanPath == "." || cleanPath == "" {
			serveIndex(w, r, indexPath)
			return
		}

		fullPath := filepath.Join(webDir, cleanPath)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			nonCacheFileServer.ServeHTTP(w, r)
			return
		}

		serveIndex(w, r, indexPath)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, indexPath string) {
	if _, err := os.Stat(indexPath); err == nil {
		setSPACacheControl(w)
		http.ServeFile(w, r, indexPath)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("index.html not found"))
}

func setSPACacheControl(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
