package web

import (
	"context"
	"errors"
	"net/http"

	"todo/internal/auth"
)

const sessionCookie = "todo_session"

type ctxKey int

const userKey ctxKey = 1

func setSessionCookie(w http.ResponseWriter, token string, s auth.Session) {
	c := &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
	if s.Persistent {
		c.MaxAge = int(s.ExpiresAt.Sub(s.CreatedAt).Seconds())
	}
	http.SetCookie(w, c)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// sessionToken returns the session token sent by the browser, or "".
func sessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// currentUser returns the logged-in user. Only valid inside protect.
func currentUser(r *http.Request) auth.User {
	u, _ := r.Context().Value(userKey).(auth.User)
	return u
}

func userID(r *http.Request) int64 { return currentUser(r).ID }

// protect lets only requests with a valid session through. Others go to the
// log-in page: a redirect for pages, 401 with HX-Redirect for htmx and fetch.
func (s *server) protect(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, err := s.accounts.Authenticate(r.Context(), sessionToken(r))
		if errors.Is(err, auth.ErrNoSession) {
			if isHTMX(r) {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
