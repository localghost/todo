// Package web serves the todo list as HTML pages and htmx fragments.
package web

import (
	"bytes"
	"crypto/rand"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"todo/internal/auth"
	"todo/internal/guard"
	"todo/internal/todo"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type server struct {
	svc        *todo.Service
	accounts   *auth.Service
	tmpl       *template.Template
	log        *slog.Logger
	now        func() time.Time
	powBits    int
	trustProxy bool
	signingKey []byte
	tokens     *guard.Tokens
	signupTry  *guard.Limiter // sign-up attempts per IP
	signupNew  *guard.Limiter // new accounts per IP
	loginUser  *guard.Limiter // wrong passwords per lowercase username
	loginIP    *guard.Limiter // failed logins per IP
}

// Option changes how the app is set up.
type Option func(*server)

// WithClock replaces the clock used for labels and the guards (for tests).
func WithClock(now func() time.Time) Option { return func(s *server) { s.now = now } }

// WithPowBits sets the sign-up proof-of-work difficulty.
func WithPowBits(bits int) Option { return func(s *server) { s.powBits = bits } }

// WithSigningKey sets the key for sign-up form tokens. Without it, a random
// key is used, and open sign-up forms stop working after a restart.
func WithSigningKey(key []byte) Option { return func(s *server) { s.signingKey = key } }

// WithTrustProxy makes the app take the client IP from X-Forwarded-For.
func WithTrustProxy(trust bool) Option { return func(s *server) { s.trustProxy = trust } }

// New returns the HTTP handler for the app.
func New(svc *todo.Service, accounts *auth.Service, log *slog.Logger, opts ...Option) (http.Handler, error) {
	s := &server{svc: svc, accounts: accounts, log: log, now: time.Now, powBits: guard.DefaultPowBits}
	for _, o := range opts {
		o(s)
	}
	if s.signingKey == nil {
		s.signingKey = make([]byte, 32)
		rand.Read(s.signingKey)
	}
	s.tokens = guard.NewTokens(s.signingKey, s.now)
	s.signupTry = guard.NewLimiter(30, time.Hour, s.now)
	s.signupNew = guard.NewLimiter(5, time.Hour, s.now)
	s.loginUser = guard.NewLimiter(5, 15*time.Minute, s.now)
	s.loginIP = guard.NewLimiter(20, 15*time.Minute, s.now)
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

// clientIP is the key used for the limits. With -trust-proxy it is the last
// address in X-Forwarded-For, which the proxy adds. An IPv6 address counts
// as its /64 network, because one client usually controls a whole /64.
func (s *server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if vals := r.Header.Values("X-Forwarded-For"); len(vals) > 0 {
			parts := strings.Split(vals[len(vals)-1], ",")
			if key, ok := ipKey(strings.TrimSpace(parts[len(parts)-1])); ok {
				return key
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if key, ok := ipKey(host); ok {
		return key
	}
	return host
}

// ipKey returns the limiter key of an address: IPv4 as is, IPv6 as its /64.
func ipKey(s string) (string, bool) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return "", false
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String(), true
	}
	prefix, err := addr.Prefix(64)
	if err != nil {
		return "", false
	}
	return prefix.String(), true
}
