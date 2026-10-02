// Package web serves the todo list as HTML pages and htmx fragments.
package web

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"todo/internal/auth"
	"todo/internal/todo"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type server struct {
	svc      *todo.Service
	accounts *auth.Service
	tmpl     *template.Template
	log      *slog.Logger
	now      func() time.Time
}

// New returns the HTTP handler for the app.
func New(svc *todo.Service, accounts *auth.Service, log *slog.Logger) (http.Handler, error) {
	s := &server{svc: svc, accounts: accounts, log: log, now: time.Now}
	funcs := template.FuncMap{
		"added":           func(t time.Time) string { return addedLabel(t, s.now()) },
		"due":             func(it todo.Item) dueView { return dueLabel(it, s.now()) },
		"postponeMinutes": func() []int { return todo.PostponeMinutes },
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s.tmpl = tmpl

	mux := http.NewServeMux()
	mux.Handle("GET /static/", noDirListing(http.FileServerFS(staticFS)))
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /signup", s.signupPage)
	mux.HandleFunc("POST /signup", s.signup)
	mux.Handle("POST /logout", s.protect(s.logout))
	mux.Handle("GET /account", s.protect(s.accountPage))
	mux.Handle("POST /account/password", s.protect(s.changePassword))
	mux.Handle("POST /account/delete", s.protect(s.deleteAccount))
	mux.Handle("GET /{$}", s.protect(s.index))
	mux.Handle("POST /items", s.protect(s.addItem))
	mux.Handle("POST /items/{id}/toggle", s.protect(s.toggleItem))
	mux.Handle("POST /items/{id}/postpone", s.protect(s.postponeItem))
	mux.Handle("GET /items/{id}/edit", s.protect(s.editItem))
	mux.Handle("GET /items/{id}", s.protect(s.showItem))
	mux.Handle("PUT /items/{id}", s.protect(s.updateItem))
	mux.Handle("DELETE /items/{id}", s.protect(s.deleteItem))
	mux.Handle("POST /notifications/claim", s.protect(s.claimNotifications))
	// Rejects changing requests that a browser marks as coming from another site.
	return securityHeaders(http.NewCrossOriginProtection().Handler(limitBody(mux))), nil
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

const contentSecurityPolicy = "default-src 'self'; style-src 'self' https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"

// securityHeaders adds the CSP and other protective headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			// Private data: the browser must not keep it (for example for Back after logout).
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// maxBody is the largest request body the app accepts.
const maxBody = 64 << 10

// limitBody rejects request bodies over maxBody before the form is parsed.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		if err := r.ParseForm(); err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "Request too large.", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "Bad request.", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
