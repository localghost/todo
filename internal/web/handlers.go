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
	text := r.FormValue("text")
	item, err := s.svc.Add(r.Context(), defaultUserID, text)
	if errors.Is(err, todo.ErrEmptyText) {
		w.Header().Set("HX-Retarget", "#add-form")
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity,
			part{"add-form", formView{Text: text, Error: true, Focus: true}})
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
	s.render(w, r, http.StatusOK,
		part{"item", item},
		part{"add-form", formView{Focus: true, OOB: true}},
		part{"oob", lv})
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
func (s *server) editItem(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *server) showItem(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *server) updateItem(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
