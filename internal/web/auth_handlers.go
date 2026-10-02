package web

import (
	"errors"
	"net/http"
	"strings"

	"todo/internal/auth"
	"todo/internal/guard"
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
	FormToken     string
	PowBits       int
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

const loginLocked = "Too many attempts. Please try again in 15 minutes."

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	ip := s.clientIP(r)
	userKey := strings.ToLower(strings.TrimSpace(username))
	locked := func() {
		s.render(w, r, http.StatusTooManyRequests, part{"login", loginView{Username: username, Error: loginLocked}})
	}
	// Reserve both slots before the slow password hash; a wrong password keeps them.
	ipTicket, ok := s.loginIP.Reserve(ip)
	if !ok {
		s.log.Info("login blocked", "ip", ip, "reason", "ip limit")
		locked()
		return
	}
	// A name that breaks the rules cannot exist: it gets no counter, so junk
	// names cannot fill the limiter's memory.
	countUser := auth.ValidateUsername(userKey) == nil
	var userTicket guard.Ticket
	if countUser {
		if userTicket, ok = s.loginUser.Reserve(userKey); !ok {
			s.loginIP.Release(ipTicket)
			s.log.Info("login blocked", "ip", ip, "reason", "username limit")
			locked()
			return
		}
	}
	token, sess, err := s.accounts.LogIn(r.Context(), username, r.PostFormValue("password"), r.PostFormValue("keep") == "1")
	if errors.Is(err, auth.ErrBadLogin) {
		s.render(w, r, http.StatusUnprocessableEntity, part{"login", loginView{Username: username, Error: "Wrong username or password."}})
		return
	}
	s.loginIP.Release(ipTicket)
	if countUser {
		s.loginUser.Release(userTicket)
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.accounts.LogOut(r.Context(), sessionToken(r)); err != nil {
		s.log.Error("end previous session", "err", err)
	}
	setSessionCookie(w, token, sess)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

const signupFailed = "Sign-up failed. Please wait a moment and try again."

// newSignupView returns a sign-up view with a fresh form token.
func (s *server) newSignupView(username string) signupView {
	return signupView{Username: username, FormToken: s.tokens.New(), PowBits: s.powBits}
}

func (s *server) signupPage(w http.ResponseWriter, r *http.Request) {
	if s.alreadyLoggedIn(w, r) {
		return
	}
	s.render(w, r, http.StatusOK, part{"signup", s.newSignupView("")})
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	username := r.PostFormValue("username")
	ip := s.clientIP(r)
	failed := func() {
		view := s.newSignupView(username)
		view.Error = signupFailed
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", view})
	}
	if _, ok := s.signupTry.Reserve(ip); !ok { // every attempt counts; never released
		s.log.Info("sign-up rejected", "ip", ip, "reason", "attempt limit")
		failed()
		return
	}
	if r.PostFormValue("website") != "" {
		s.log.Info("sign-up rejected", "ip", ip, "reason", "honeypot")
		failed()
		return
	}
	if err := s.tokens.Check(r.PostFormValue("form_token"), r.PostFormValue("pow_nonce"), s.powBits); err != nil {
		s.log.Info("sign-up rejected", "ip", ip, "reason", err)
		failed()
		return
	}
	newTicket, ok := s.signupNew.Reserve(ip)
	if !ok {
		s.log.Info("sign-up rejected", "ip", ip, "reason", "account limit")
		failed()
		return
	}
	u, err := s.accounts.SignUp(r.Context(), username, r.PostFormValue("password"))
	if err != nil {
		s.signupNew.Release(newTicket) // no account was created
	}
	var rule *auth.RuleError
	switch {
	case errors.Is(err, auth.ErrUsernameTaken):
		view := s.newSignupView(username)
		view.UsernameError = "This username is taken."
		s.render(w, r, http.StatusUnprocessableEntity, part{"signup", view})
		return
	case errors.As(err, &rule):
		view := s.newSignupView(username)
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
	if err := s.accounts.LogOut(r.Context(), sessionToken(r)); err != nil {
		s.log.Error("end previous session", "err", err)
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

type accountView struct {
	Username      string
	MemberSince   string
	PasswordError string
	PasswordOK    bool
	DeleteError   string
}

func (s *server) accountView(r *http.Request) accountView {
	u := currentUser(r)
	return accountView{Username: u.Username, MemberSince: u.CreatedAt.In(s.now().Location()).Format("2 Jan 2006")}
}

func (s *server) accountPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, part{"account", s.accountView(r)})
}

func (s *server) changePassword(w http.ResponseWriter, r *http.Request) {
	view := s.accountView(r)
	err := s.accounts.ChangePassword(r.Context(), userID(r), sessionToken(r),
		r.PostFormValue("current_password"), r.PostFormValue("new_password"))
	var rule *auth.RuleError
	switch {
	case errors.Is(err, auth.ErrWrongPassword):
		view.PasswordError = "The current password is wrong."
	case errors.As(err, &rule):
		view.PasswordError = rule.Msg
	case err != nil:
		s.serverError(w, r, err)
		return
	default:
		view.PasswordOK = true
		s.render(w, r, http.StatusOK, part{"account", view})
		return
	}
	s.render(w, r, http.StatusUnprocessableEntity, part{"account", view})
}

func (s *server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	err := s.accounts.DeleteAccount(r.Context(), userID(r), r.PostFormValue("confirm_username"))
	if errors.Is(err, auth.ErrConfirmMismatch) {
		view := s.accountView(r)
		view.DeleteError = "The username does not match."
		s.render(w, r, http.StatusUnprocessableEntity, part{"account", view})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/login?deleted=1", http.StatusSeeOther)
}
