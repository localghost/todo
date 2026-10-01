package web

import "net/http"

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

func (s *server) addItem(w http.ResponseWriter, r *http.Request)    { http.NotFound(w, r) }
func (s *server) toggleItem(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
func (s *server) editItem(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *server) showItem(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *server) updateItem(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
func (s *server) deleteItem(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
