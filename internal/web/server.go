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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	svc            *todo.Service
	accounts       *auth.Service
	base           *template.Template // parsed once, never executed; cloned per zone
	views          sync.Map           // zone name → *template.Template bound to that zone
	zones          sync.Map           // todo_tz cookie value → *time.Location
	viewCount      atomic.Int32       // entries in views, at most maxZoneSets
	log            *slog.Logger
	now            func() time.Time
	powBits        int
	trustProxy     bool
	clientIPHeader string
	xffWarn        sync.Once
	signingKey     []byte
	tokens         *guard.Tokens
	signupTry      *guard.Limiter // sign-up attempts per IP
	signupNew      *guard.Limiter // new accounts per IP
	loginUser      *guard.Limiter // wrong passwords per lowercase username
	loginIP        *guard.Limiter // failed logins per IP
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

// WithClientIPHeader makes the app take the client IP from the header name, which
// the proxy sets (for example Fly-Client-IP on Fly.io). It wins over WithTrustProxy.
func WithClientIPHeader(name string) Option { return func(s *server) { s.clientIPHeader = name } }

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
	tmpl, err := template.New("").Funcs(s.zoneFuncs(time.UTC)).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s.base = tmpl

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
		tmpl, err := s.templatesFor(s.zoneFor(r))
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if err := tmpl.ExecuteTemplate(&buf, p.name, p.data); err != nil {
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
	if s.clientIPHeader != "" {
		v := strings.TrimSpace(r.Header.Get(s.clientIPHeader))
		if key, ok := ipKey(v); ok {
			return key
		}
		s.log.Warn("cannot read the client address", "header", s.clientIPHeader, "value", cut(v, 64))
	} else if vals := r.Header.Values("X-Forwarded-For"); len(vals) > 0 {
		if !s.trustProxy {
			s.xffWarn.Do(func() {
				s.log.Warn("requests have X-Forwarded-For but -trust-proxy is off; if this app runs behind a reverse proxy, start it with -trust-proxy; if not, ignore this (clients can send the header themselves)")
			})
		} else {
			parts := strings.Split(vals[len(vals)-1], ",")
			last := strings.TrimSpace(parts[len(parts)-1])
			if ap, err := netip.ParseAddrPort(last); err == nil {
				last = ap.Addr().String()
			}
			last = strings.TrimSuffix(strings.TrimPrefix(last, "["), "]")
			if key, ok := ipKey(last); ok {
				return key
			}
			s.log.Warn("cannot read the client address from X-Forwarded-For", "value", cut(last, 64))
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

// cut returns at most n bytes of s, so a client cannot fill the log.
func cut(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
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

// zoneCookie names the browser's time zone; static/tz.js sets it.
const zoneCookie = "todo_tz"

var zoneName = regexp.MustCompile(`^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*$`)

// zoneFor returns the browser's time zone from the todo_tz cookie, or the
// server's zone if the cookie is missing or names no known zone.
func (s *server) zoneFor(r *http.Request) *time.Location {
	if c, err := r.Cookie(zoneCookie); err == nil {
		if loc, ok := s.parseZone(c.Value); ok {
			return loc
		}
	}
	return s.now().Location()
}

// parseZone returns the zone named name, if it is a valid IANA name.
func (s *server) parseZone(name string) (*time.Location, bool) {
	if len(name) > 64 || name == "Local" || !zoneName.MatchString(name) {
		return nil, false
	}
	if loc, ok := s.zones.Load(name); ok {
		return loc.(*time.Location), true
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, false
	}
	s.zones.Store(name, loc)
	return loc, true
}

// zoneFuncs returns the template functions that show times in loc.
func (s *server) zoneFuncs(loc *time.Location) template.FuncMap {
	return template.FuncMap{
		"added":           func(t time.Time) string { return addedLabel(t, s.now().In(loc)) },
		"due":             func(it todo.Item) dueView { return dueLabel(it, s.now().In(loc)) },
		"zone":            func() string { return loc.String() },
		"postponeMinutes": func() []int { return todo.PostponeMinutes },
	}
}

// templatesFor returns the templates bound to loc, cloned from the
// never-executed base set. The first maxZoneSets zones stay cached; any other
// zone gets a fresh set per request, so one client cannot fill the memory.
func (s *server) templatesFor(loc *time.Location) (*template.Template, error) {
	if t, ok := s.views.Load(loc.String()); ok {
		return t.(*template.Template), nil
	}
	t, err := s.base.Clone()
	if err != nil {
		return nil, err
	}
	t.Funcs(s.zoneFuncs(loc))
	for {
		n := s.viewCount.Load()
		if n >= maxZoneSets {
			return t, nil
		}
		if s.viewCount.CompareAndSwap(n, n+1) {
			break
		}
	}
	actual, loaded := s.views.LoadOrStore(loc.String(), t)
	if loaded {
		s.viewCount.Add(-1)
	}
	return actual.(*template.Template), nil
}

// maxZoneSets limits the cached template sets (one per zone, about 74 KiB each).
const maxZoneSets = 16
