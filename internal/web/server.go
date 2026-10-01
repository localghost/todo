// Package web serves the todo list as HTML pages and htmx fragments.
package web

import (
	"bytes"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"todo/internal/todo"
)

// defaultUserID is the only user until login exists.
const defaultUserID int64 = 1

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type server struct {
	svc  *todo.Service
	tmpl *template.Template
	log  *slog.Logger
}

// New returns the HTTP handler for the app.
func New(svc *todo.Service, log *slog.Logger) (http.Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s := &server{svc: svc, tmpl: tmpl, log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.Handle("GET /static/", noDirListing(http.FileServerFS(staticFS)))
	mux.HandleFunc("POST /items", s.addItem)
	mux.HandleFunc("POST /items/{id}/toggle", s.toggleItem)
	mux.HandleFunc("GET /items/{id}/edit", s.editItem)
	mux.HandleFunc("GET /items/{id}", s.showItem)
	mux.HandleFunc("PUT /items/{id}", s.updateItem)
	mux.HandleFunc("DELETE /items/{id}", s.deleteItem)
	return sameOriginOnly(mux), nil
}

// noDirListing answers 404 for folder paths, so the file server never lists files.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sameOriginOnly rejects changing requests that another website could send
// (CSRF). A cross-site HTML form cannot set the HX-Request header, and
// browsers mark cross-site requests in Sec-Fetch-Site.
func sameOriginOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			site := r.Header.Get("Sec-Fetch-Site")
			if !isHTMX(r) || (site != "" && site != "same-origin") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// part is one template to render into a response.
type part struct {
	name string
	data any
}

// render executes all parts into one buffer, then writes status and body.
// If a template fails, the client gets a 500 and no half-written HTML.
func (s *server) render(w http.ResponseWriter, r *http.Request, status int, parts ...part) {
	var buf bytes.Buffer
	for _, p := range parts {
		if err := s.tmpl.ExecuteTemplate(&buf, p.name, p.data); err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (s *server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
}

// hideDoneFrom reads the hide state from the URL query or the form body.
func hideDoneFrom(r *http.Request) bool {
	return r.FormValue("hide_done") == "1"
}

// parseID reads the {id} path value. It returns false for a bad ID.
func parseID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
