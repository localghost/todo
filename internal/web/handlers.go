package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"todo/internal/todo"
)

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	lv, err := s.listView(r.Context(), userID(r), currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Add("Vary", "HX-Request")
	// htmx asks for the list fragment when the hide/show button is clicked.
	// A history restore after "Back" needs the full page.
	if isHTMX(r) && r.Header.Get("HX-History-Restore-Request") != "true" {
		s.render(w, r, http.StatusOK, part{"list", lv})
		return
	}
	s.render(w, r, http.StatusOK, part{"page", pageView{Form: formView{Focus: true}, List: lv, Username: currentUser(r).Username}})
}

func (s *server) addItem(w http.ResponseWriter, r *http.Request) {
	text, date, clock := r.FormValue("text"), r.FormValue("due_date"), r.FormValue("due_time")
	item, err := s.svc.AddIn(r.Context(), userID(r), text, date, clock, s.zoneFor(r))
	var dueErr *todo.DueError
	if errors.Is(err, todo.ErrEmptyText) || errors.As(err, &dueErr) {
		view := formView{Text: text, DueDate: date, DueTime: clock, Focus: true, Error: errors.Is(err, todo.ErrEmptyText)}
		if dueErr != nil {
			view.DueError = dueErr.Msg
		}
		w.Header().Set("HX-Retarget", "#add-form")
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity, part{"add-form", view})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv, err := s.listView(r.Context(), userID(r), currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv.OOB = true
	// The add form is not part of the answer: app.js clears the input, so
	// text typed while this request ran is not lost.
	s.render(w, r, http.StatusOK, part{"item", item}, part{"oob", lv})
}

func (s *server) toggleItem(w http.ResponseWriter, r *http.Request) {
	hideDone := currentUser(r).HideDone
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	item, err := s.svc.Toggle(r.Context(), userID(r), id)
	if errors.Is(err, todo.ErrNotFound) {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv, err := s.listView(r.Context(), userID(r), hideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv.OOB = true
	parts := []part{}
	// In hide mode a done row disappears: an empty main body removes it.
	if !(hideDone && item.Done) {
		parts = append(parts, part{"item", item})
	}
	s.render(w, r, http.StatusOK, append(parts, part{"oob", lv})...)
}

func (s *server) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	err := s.svc.Delete(r.Context(), userID(r), id)
	if errors.Is(err, todo.ErrNotFound) {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.oobOnly(w, r, http.StatusOK)
}

// oobOnly answers with an empty main body (htmx removes the target row)
// plus the out-of-band toolbar and empty state.
func (s *server) oobOnly(w http.ResponseWriter, r *http.Request, status int) {
	lv, err := s.listView(r.Context(), userID(r), currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv.OOB = true
	s.render(w, r, status, part{"oob", lv})
}
func (s *server) editItem(w http.ResponseWriter, r *http.Request) {
	item, ok := s.findItem(w, r)
	if !ok {
		return
	}
	date, clock := dueInputs(item, s.zoneFor(r))
	s.render(w, r, http.StatusOK, part{"item-edit", editView{Item: item, Text: item.Text, DueDate: date, DueTime: clock}})
}

func (s *server) showItem(w http.ResponseWriter, r *http.Request) {
	item, ok := s.findItem(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, part{"item", item})
}

func (s *server) updateItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	text, date, clock := r.FormValue("text"), r.FormValue("due_date"), r.FormValue("due_time")
	// Read the time in the zone the edit row was shown in, so an unchanged
	// time stays the same moment even if the browser's zone changed since.
	loc, ok := s.parseZone(r.FormValue("due_tz"))
	if !ok {
		loc = s.zoneFor(r)
	}
	item, err := s.svc.EditIn(r.Context(), userID(r), id, text, date, clock, loc)
	var dueErr *todo.DueError
	switch {
	case errors.Is(err, todo.ErrEmptyText) || errors.As(err, &dueErr):
		old, getErr := s.svc.Get(r.Context(), userID(r), id)
		if errors.Is(getErr, todo.ErrNotFound) {
			s.oobOnly(w, r, http.StatusNotFound)
			return
		}
		if getErr != nil {
			s.serverError(w, r, getErr)
			return
		}
		view := editView{Item: old, Text: text, DueDate: date, DueTime: clock, Error: errors.Is(err, todo.ErrEmptyText)}
		if dueErr != nil {
			view.DueError = dueErr.Msg
		}
		s.render(w, r, http.StatusUnprocessableEntity, part{"item-edit", view})
	case errors.Is(err, todo.ErrNotFound):
		s.oobOnly(w, r, http.StatusNotFound)
	case err != nil:
		s.serverError(w, r, err)
	default:
		// The due date may have changed, so the permission bar comes along.
		lv, err := s.listView(r.Context(), userID(r), currentUser(r).HideDone)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		lv.OOB = true
		s.render(w, r, http.StatusOK, part{"item", item}, part{"oob", lv})
	}
}

// findItem loads the item named by {id}. If it is missing, findItem writes
// the 404 response and returns false.
func (s *server) findItem(w http.ResponseWriter, r *http.Request) (todo.Item, bool) {
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return todo.Item{}, false
	}
	item, err := s.svc.Get(r.Context(), userID(r), id)
	if errors.Is(err, todo.ErrNotFound) {
		s.oobOnly(w, r, http.StatusNotFound)
		return todo.Item{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return todo.Item{}, false
	}
	return item, true
}

// notification is one due item for the page to show as a browser notification.
type notification struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
	Due   string `json:"due"`
}

// claimNotifications returns the items that are due and not notified yet,
// and marks them, so each item notifies only once.
func (s *server) claimNotifications(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	items, err := s.svc.ClaimDue(r.Context(), userID(r), now)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	out := make([]notification, 0, len(items))
	for _, it := range items {
		title, due := notifyText(it, now.In(s.zoneFor(r)))
		out = append(out, notification{ID: it.ID, Title: title, Text: it.Text, Due: due})
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		s.log.Error("write claim response", "err", err)
	}
}

// postponeItem moves an item's due time by ?minutes= (5, 10, 15, or 30).
func (s *server) postponeItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	minutes, err := strconv.Atoi(r.URL.Query().Get("minutes"))
	if err != nil {
		http.Error(w, "Unknown postpone amount.", http.StatusBadRequest)
		return
	}
	item, err := s.svc.Postpone(r.Context(), userID(r), id, minutes)
	switch {
	case errors.Is(err, todo.ErrBadPostpone):
		http.Error(w, "Unknown postpone amount.", http.StatusBadRequest)
	case errors.Is(err, todo.ErrNotFound):
		s.oobOnly(w, r, http.StatusNotFound)
	case errors.Is(err, todo.ErrCannotPostpone):
		old, getErr := s.svc.Get(r.Context(), userID(r), id)
		if getErr != nil {
			s.serverError(w, r, getErr)
			return
		}
		s.render(w, r, http.StatusUnprocessableEntity, part{"item", old})
	case err != nil:
		s.serverError(w, r, err)
	default:
		lv, err := s.listView(r.Context(), userID(r), currentUser(r).HideDone)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		lv.OOB = true
		s.render(w, r, http.StatusOK, part{"item", item}, part{"oob", lv})
	}
}

// setHideDone saves whether the list hides done items, then shows the list.
func (s *server) setHideDone(w http.ResponseWriter, r *http.Request) {
	hide := r.FormValue("hide_done") == "1"
	if err := s.accounts.SetHideDone(r.Context(), userID(r), hide); err != nil {
		s.serverError(w, r, err)
		return
	}
	if !isHTMX(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	lv, err := s.listView(r.Context(), userID(r), hide)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"list", lv})
}
