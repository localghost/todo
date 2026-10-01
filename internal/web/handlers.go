package web

import (
	"errors"
	"net/http"

	"todo/internal/todo"
)

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	lv, err := s.listView(r.Context(), hideDoneFrom(r))
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
	s.render(w, r, http.StatusOK, part{"page", pageView{Form: formView{Focus: true}, List: lv}})
}

func (s *server) addItem(w http.ResponseWriter, r *http.Request) {
	text, date, clock := r.FormValue("text"), r.FormValue("due_date"), r.FormValue("due_time")
	item, err := s.svc.Add(r.Context(), defaultUserID, text, date, clock)
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
	lv, err := s.listView(r.Context(), hideDoneFrom(r))
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
	hideDone := hideDoneFrom(r)
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	item, err := s.svc.Toggle(r.Context(), defaultUserID, id)
	if errors.Is(err, todo.ErrNotFound) {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv, err := s.listView(r.Context(), hideDone)
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
	err := s.svc.Delete(r.Context(), defaultUserID, id)
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
	lv, err := s.listView(r.Context(), hideDoneFrom(r))
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
	date, clock := dueInputs(item, s.now().Location())
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
	item, err := s.svc.Edit(r.Context(), defaultUserID, id, text, date, clock)
	var dueErr *todo.DueError
	switch {
	case errors.Is(err, todo.ErrEmptyText) || errors.As(err, &dueErr):
		old, getErr := s.svc.Get(r.Context(), defaultUserID, id)
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
		s.render(w, r, http.StatusOK, part{"item", item})
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
	item, err := s.svc.Get(r.Context(), defaultUserID, id)
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
