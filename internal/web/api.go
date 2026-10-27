package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"todo/internal/auth"
	"todo/internal/todo"
)

// The JSON API for the Android app (spec 2026-10-21-android-app-design.md).
// It calls the same services as the web pages and adds no rules of its own.
// API routes read only the bearer token, never the session cookie, so a web
// page cannot call them with the user's cookie and they need no CSRF defense.

func (s *server) apiRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/login", s.apiLogin)
	mux.Handle("POST /api/v1/logout", s.apiProtect(s.apiLogout))
	mux.Handle("GET /api/v1/me", s.apiProtect(s.apiMe))
	mux.Handle("PUT /api/v1/settings", s.apiProtect(s.apiSettings))
	mux.Handle("GET /api/v1/items", s.apiProtect(s.apiItems))
	mux.Handle("POST /api/v1/items", s.apiProtect(s.apiAddItem))
	mux.Handle("POST /api/v1/items/done/delete", s.apiProtect(s.apiClearDone))
	mux.Handle("GET /api/v1/items/{id}", s.apiProtect(s.apiGetItem))
	mux.Handle("PATCH /api/v1/items/{id}", s.apiProtect(s.apiUpdateItem))
	mux.Handle("DELETE /api/v1/items/{id}", s.apiProtect(s.apiDeleteItem))
	mux.Handle("PUT /api/v1/items/{id}/due", s.apiProtect(s.apiSetDue))
	mux.Handle("POST /api/v1/items/{id}/postpone", s.apiProtect(s.apiPostpone))
	mux.Handle("POST /api/v1/items/{id}/move", s.apiProtect(s.apiMove))
	mux.Handle("POST /api/v1/items/{id}/labels", s.apiProtect(s.apiAddLabel))
	mux.Handle("DELETE /api/v1/items/{id}/labels/{label}", s.apiProtect(s.apiRemoveLabel))
	mux.Handle("GET /api/v1/labels", s.apiProtect(s.apiLabels))
	mux.Handle("POST /api/v1/labels", s.apiProtect(s.apiCreateLabel))
	mux.Handle("PUT /api/v1/labels/{id}/color", s.apiProtect(s.apiSetLabelColor))
	mux.Handle("DELETE /api/v1/labels/{id}", s.apiProtect(s.apiDeleteLabel))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		apiError(w, http.StatusNotFound, "not_found", "Unknown address.")
	})
}

// apiLabel is a label in API answers. Items is set only in the label list.
type apiLabel struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Items *int   `json:"items,omitempty"`
}

// apiItem is an item in API answers. Times are UTC.
type apiItem struct {
	ID        int64      `json:"id"`
	Text      string     `json:"text"`
	Done      bool       `json:"done"`
	DueAt     *time.Time `json:"due_at"`
	DueAllDay bool       `json:"due_all_day"`
	Overdue   bool       `json:"overdue"`
	Labels    []apiLabel `json:"labels"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// apiList is the answer of GET /api/v1/items.
type apiList struct {
	Items    []apiItem `json:"items"`
	Open     int       `json:"open"`
	Done     int       `json:"done"`
	HideDone bool      `json:"hide_done"`
	Label    *apiLabel `json:"label"` // the filter, or null
}

type apiUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	HideDone bool   `json:"hide_done"`
}

func toAPILabel(l todo.Label) apiLabel {
	return apiLabel{ID: l.ID, Name: l.Name, Color: l.Color}
}

func toAPIItem(it todo.Item, now time.Time) apiItem {
	out := apiItem{ID: it.ID, Text: it.Text, Done: it.Done, DueAllDay: it.DueAllDay, Overdue: it.Overdue(now),
		Labels: []apiLabel{}, CreatedAt: it.CreatedAt.UTC(), UpdatedAt: it.UpdatedAt.UTC()}
	if it.DueAt != nil {
		d := it.DueAt.UTC()
		out.DueAt = &d
	}
	for _, l := range it.Labels {
		out.Labels = append(out.Labels, toAPILabel(l))
	}
	return out
}

func writeJSON(w http.ResponseWriter, r *http.Request, s *server, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("write API response", "path", r.URL.Path, "err", err)
	}
}

// apiError answers with {"error": code, "message": msg}. The app shows msg.
func apiError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": msg})
}

func (s *server) apiServerError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	apiError(w, http.StatusInternalServerError, "server_error", "Something went wrong. Please try again.")
}

// readJSON decodes the request body into v. On a bad body it answers 400
// (413 if too large) and returns false.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	err := json.NewDecoder(r.Body).Decode(v)
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		apiError(w, http.StatusRequestEntityTooLarge, "too_large", "Request too large.")
		return false
	case err != nil:
		apiError(w, http.StatusBadRequest, "bad_request", "The request is not valid JSON.")
		return false
	}
	return true
}

// bearerToken returns the token of "Authorization: Bearer <token>", or "".
func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// apiProtect lets only requests with a valid bearer token through. Others
// get 401 {"error": "logged_out"}, and the app shows its log-in screen.
func (s *server) apiProtect(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, err := s.accounts.AuthenticateApp(r.Context(), bearerToken(r))
		if errors.Is(err, auth.ErrNoSession) {
			apiError(w, http.StatusUnauthorized, "logged_out", "Please log in again.")
			return
		}
		if err != nil {
			s.apiServerError(w, r, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

func (s *server) apiLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	token, sess, err := s.checkLogin(r, in.Username, in.Password, true)
	switch {
	case errors.Is(err, errLoginLocked):
		w.Header().Set("Retry-After", "900")
		apiError(w, http.StatusTooManyRequests, "locked", loginLocked)
	case errors.Is(err, auth.ErrBadLogin):
		apiError(w, http.StatusUnauthorized, "bad_login", "Wrong username or password.")
	case errors.Is(err, auth.ErrBusy):
		apiError(w, http.StatusServiceUnavailable, "busy", serverBusy)
	case err != nil:
		s.apiServerError(w, r, err)
	default:
		u, _, err := s.accounts.Authenticate(r.Context(), token)
		if err != nil {
			s.apiServerError(w, r, err)
			return
		}
		writeJSON(w, r, s, http.StatusOK, map[string]any{"token": token, "expires_at": sess.ExpiresAt.UTC(),
			"user": apiUser{ID: u.ID, Username: u.Username, HideDone: u.HideDone}})
	}
}

func (s *server) apiLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.LogOut(r.Context(), bearerToken(r)); err != nil {
		s.apiServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) apiMe(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	writeJSON(w, r, s, http.StatusOK, apiUser{ID: u.ID, Username: u.Username, HideDone: u.HideDone})
}

func (s *server) apiSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		HideDone *bool `json:"hide_done"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	u := currentUser(r)
	if in.HideDone != nil {
		if err := s.accounts.SetHideDone(r.Context(), u.ID, *in.HideDone); err != nil {
			s.apiServerError(w, r, err)
			return
		}
		u.HideDone = *in.HideDone
	}
	writeJSON(w, r, s, http.StatusOK, apiUser{ID: u.ID, Username: u.Username, HideDone: u.HideDone})
}

// apiItems answers the list as the web shows it: overdue items first, done
// items left out if the user hides them, and only ?label= items if given.
func (s *server) apiItems(w http.ResponseWriter, r *http.Request) {
	lv, err := s.listView(r, currentUser(r).HideDone)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	now := s.nowFor(r)
	out := apiList{Items: []apiItem{}, Open: lv.Open, Done: lv.Done, HideDone: lv.HideDone}
	for _, it := range lv.Items {
		out.Items = append(out.Items, toAPIItem(it, now))
	}
	if lv.Filter != nil {
		l := toAPILabel(*lv.Filter)
		out.Label = &l
	}
	writeJSON(w, r, s, http.StatusOK, out)
}

func (s *server) apiAddItem(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text   string   `json:"text"`
		Labels []string `json:"labels"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	// Label names cannot hold a comma (ParseLabels splits on it).
	item, err := s.svc.AddItem(r.Context(), userID(r), todo.Input{Text: in.Text, Labels: strings.Join(in.Labels, ",")}, s.zoneFor(r))
	s.apiItemAnswer(w, r, http.StatusCreated, item, err)
}

func (s *server) apiGetItem(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	item, err := s.svc.Get(r.Context(), userID(r), id)
	s.apiItemAnswer(w, r, http.StatusOK, item, err)
}

// apiUpdateItem changes the text, the done state, or both.
func (s *server) apiUpdateItem(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	var in struct {
		Text *string `json:"text"`
		Done *bool   `json:"done"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	item, err := s.svc.Get(r.Context(), userID(r), id)
	if err == nil && in.Text != nil {
		item, err = s.svc.EditText(r.Context(), userID(r), id, *in.Text)
	}
	if err == nil && in.Done != nil && *in.Done != item.Done {
		item, err = s.svc.SetDone(r.Context(), userID(r), id, *in.Done)
	}
	s.apiItemAnswer(w, r, http.StatusOK, item, err)
}

func (s *server) apiDeleteItem(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	s.apiNoContent(w, r, s.svc.Delete(r.Context(), userID(r), id))
}

// apiSetDue sets the due date from {"date", "time"} in the phone's zone.
// An empty time is all day; both empty remove the due date.
func (s *server) apiSetDue(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	var in struct {
		Date string `json:"date"`
		Time string `json:"time"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	item, err := s.svc.SetDue(r.Context(), userID(r), id, in.Date, in.Time, s.zoneFor(r))
	s.apiItemAnswer(w, r, http.StatusOK, item, err)
}

func (s *server) apiPostpone(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	var in struct {
		Minutes int `json:"minutes"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	item, err := s.svc.Postpone(r.Context(), userID(r), id, in.Minutes)
	s.apiItemAnswer(w, r, http.StatusOK, item, err)
}

// apiMove puts an item right after item "after" (0: first).
func (s *server) apiMove(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	var in struct {
		After int64 `json:"after"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.After < 0 {
		apiError(w, http.StatusNotFound, "not_found", "This item no longer exists.")
		return
	}
	s.apiNoContent(w, r, s.svc.Move(r.Context(), userID(r), id, in.After, s.nowFor(r)))
}

// apiAddLabel puts the label "name" on an item, making it if needed.
// "color" (optional) is the color of a new label.
func (s *server) apiAddLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	var in struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	item, err := s.svc.AddLabel(r.Context(), userID(r), id, in.Name, in.Color)
	s.apiItemAnswer(w, r, http.StatusOK, item, err)
}

func (s *server) apiRemoveLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	labelID, err := strconv.ParseInt(r.PathValue("label"), 10, 64)
	if err != nil || labelID <= 0 {
		apiError(w, http.StatusNotFound, "not_found", "This label no longer exists.")
		return
	}
	item, err := s.svc.RemoveLabel(r.Context(), userID(r), id, labelID)
	s.apiItemAnswer(w, r, http.StatusOK, item, err)
}

// apiClearDone deletes the done items (with ?label= only the ones with it).
func (s *server) apiClearDone(w http.ResponseWriter, r *http.Request) {
	filter, err := s.labelFilter(r)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	var labelID int64
	if filter != nil {
		labelID = filter.ID
	}
	n, err := s.svc.ClearDone(r.Context(), userID(r), labelID)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	writeJSON(w, r, s, http.StatusOK, map[string]int{"deleted": n})
}

func (s *server) apiLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	out := make([]apiLabel, 0, len(labels))
	for _, l := range labels {
		a := toAPILabel(l)
		a.Items = &l.Items
		out = append(out, a)
	}
	writeJSON(w, r, s, http.StatusOK, out)
}

// apiCreateLabel makes the label "name" with no items, or gives the label of
// that name if it exists. "color" (optional) is the color of a new label.
func (s *server) apiCreateLabel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	l, err := s.svc.CreateLabel(r.Context(), userID(r), in.Name, in.Color)
	if err != nil {
		s.apiChangeError(w, r, err)
		return
	}
	writeJSON(w, r, s, http.StatusOK, toAPILabel(l))
}

func (s *server) apiSetLabelColor(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	var in struct {
		Color string `json:"color"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	s.apiLabelChange(w, r, s.svc.SetLabelColor(r.Context(), userID(r), id, in.Color))
}

func (s *server) apiDeleteLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := apiID(w, r)
	if !ok {
		return
	}
	s.apiLabelChange(w, r, s.svc.DeleteLabel(r.Context(), userID(r), id))
}

// apiLabelChange answers a change of a label: 204, or the error.
func (s *server) apiLabelChange(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, todo.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "This label no longer exists.")
		return
	}
	s.apiNoContent(w, r, err)
}

// apiID reads the {id} path value; for a bad ID it answers 404.
func apiID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := parseID(r)
	if !ok {
		apiError(w, http.StatusNotFound, "not_found", "This item no longer exists.")
	}
	return id, ok
}

// apiItemAnswer answers with the item, or with the error of a change.
func (s *server) apiItemAnswer(w http.ResponseWriter, r *http.Request, status int, item todo.Item, err error) {
	if err != nil {
		s.apiChangeError(w, r, err)
		return
	}
	writeJSON(w, r, s, status, toAPIItem(item, s.nowFor(r)))
}

// apiNoContent answers 204, or the error of a change.
func (s *server) apiNoContent(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		s.apiChangeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// apiChangeError answers a refused change with the same text the web shows.
func (s *server) apiChangeError(w http.ResponseWriter, r *http.Request, err error) {
	var dueErr *todo.DueError
	switch {
	case errors.Is(err, todo.ErrNotFound):
		apiError(w, http.StatusNotFound, "not_found", "This item no longer exists.")
	case textMessage(err, "Please type some text first.") != "":
		apiError(w, http.StatusUnprocessableEntity, "bad_text", textMessage(err, "Please type some text first."))
	case labelsMessage(err) != "":
		apiError(w, http.StatusUnprocessableEntity, "bad_label", labelsMessage(err))
	case errors.Is(err, todo.ErrBadLabel):
		apiError(w, http.StatusUnprocessableEntity, "bad_label", "Pick a color.")
	case errors.As(err, &dueErr):
		apiError(w, http.StatusUnprocessableEntity, "bad_due", dueErr.Msg)
	case errors.Is(err, todo.ErrBadPostpone):
		apiError(w, http.StatusBadRequest, "bad_postpone", "Unknown postpone amount.")
	case errors.Is(err, todo.ErrCannotPostpone):
		apiError(w, http.StatusConflict, "cannot_postpone", "Only open items with a due date can be postponed.")
	case errors.Is(err, todo.ErrCannotMove):
		apiError(w, http.StatusConflict, "cannot_move", "Overdue items stay at the top.")
	default:
		s.apiServerError(w, r, err)
	}
}
