package web

import (
	"errors"
	"net/http"

	"todo/internal/auth"
)

type loginView struct {
	Username string
	Error    string
	Notice   string
}

type signupView struct {
	Username      string
	Error         string
	UsernameError string
	PasswordError string
}

// alreadyLoggedIn sends a logged-in browser from the log-in and sign-up pages to the list.
func (s *server) alreadyLoggedIn(w http.ResponseWriter, r *http.Request) bool {
	if _, _, err := s.accounts.Authenticate(r.Context(), sessionToken(r)); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return true
	}
	return false
}

func (s *server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.alreadyLoggedIn(w, r) {
		return
	}
	view := loginView{}
	if r.URL.Query().Get("deleted") == "1" {
		view.Notice = "Your account was deleted."
	}
	s.render(w, r, http.StatusOK, part{"login", view})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	token, sess, err := s.accounts.LogIn(r.Context(), username, r.PostFormValue("password"), r.PostFormValue("keep") == "1")
	if errors.Is(err, auth.ErrBadLogin) {
		s.render(w, r, http.StatusUnprocessableEntity, part{"login", loginView{Username: username, Error: "Wrong username or password."}})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	setSessionCookie(w, token, sess)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) signupPage(w http.ResponseWriter, r *http.Request) {
	if s.alreadyLoggedIn(w, r) {
		return
	}
	s.render(w, r, http.StatusOK, part{"signup", signupView{}})
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	u, err := s.accounts.SignUp(r.Context(), username, r.PostFormValue("password"))
	var rule *auth.RuleError
	switch {
	case errors.Is(err, auth.ErrUsernameTaken):
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", signupView{Username: username, UsernameError: "This username is taken."}})
		return
	case errors.As(err, &rule):
		view := signupView{Username: username}
		if rule.Field == "username" {
			view.UsernameError = rule.Msg
		} else {
			view.PasswordError = rule.Msg
		}
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", view})
		return
	case err != nil:
		s.serverError(w, r, err)
		return
	}
	token, sess, err := s.accounts.StartSession(r.Context(), u.ID, false)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	setSessionCookie(w, token, sess)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.LogOut(r.Context(), sessionToken(r)); err != nil {
		s.serverError(w, r, err)
		return
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
