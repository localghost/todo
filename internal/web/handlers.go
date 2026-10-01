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

func (s *server) toggleItem(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
func (s *server) editItem(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *server) showItem(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *server) updateItem(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
func (s *server) deleteItem(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
