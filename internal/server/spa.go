package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves the built frontend, falling back to index.html so
// client-side routes like /login work on refresh.
func spaHandler(frontend fs.FS) http.Handler {
	files := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" {
			if st, err := fs.Stat(frontend, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "_app/immutable/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(frontend, "index.html")
		if err != nil {
			http.Error(w, "Frontend not built. Run `npm run build` in web/, or use the Vite dev server.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
