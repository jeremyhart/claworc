package middleware

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

type SPAHandler struct {
	fs        http.FileSystem
	indexHTML []byte
}

func NewSPAHandler(fsys fs.FS) *SPAHandler {
	index, _ := fs.ReadFile(fsys, "index.html")
	return &SPAHandler{
		fs:        http.FS(fsys),
		indexHTML: index,
	}
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/openclaw/") || r.URL.Path == "/health" {
		http.NotFound(w, r)
		return
	}

	// Try to serve the actual file
	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	if f, err := h.fs.Open(reqPath); err == nil {
		defer f.Close()
		if stat, err := f.Stat(); err == nil && !stat.IsDir() {
			http.FileServer(h.fs).ServeHTTP(w, r)
			return
		}
	}

	// A request that looks like a static asset (has a file extension, e.g.
	// /assets/index-abc123.js) but wasn't found must NOT fall back to the SPA
	// shell. Returning index.html (text/html) for a missing .js makes the
	// browser reject the module script on its MIME check, and the app silently
	// fails to boot with a blank screen — the typical symptom after a redeploy
	// when a stale, cached index.html still references old asset hashes. A real
	// 404 surfaces the problem instead of hiding it behind HTML.
	if path.Ext(reqPath) != "" {
		http.NotFound(w, r)
		return
	}

	// Fall back to index.html for client-side routing. The shell must never be
	// cached: it pins the hashed asset filenames, so a stale copy points the
	// browser at assets that no longer exist after a deploy. Hashed assets are
	// safe to cache (their name changes on every build); index.html is not.
	if h.indexHTML != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Write(h.indexHTML)
		return
	}

	http.NotFound(w, r)
}
