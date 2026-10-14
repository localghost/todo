package web

import (
	"errors"
	"net/http"
	"strconv"

	"todo/internal/todo"
)

// labelRow is one label on the labels page.
type labelRow struct {
	todo.Label
	NameValue  string // the name field (what the user typed, after an error)
	ColorValue string
	Confirm    bool   // ask whether to delete the label
	Saved      bool   // the last save changed this label
	Error      string // why the last save failed
}

// labelsView is the data for the labels page.
type labelsView struct {
	Rows   []labelRow
	Colors []string
}

// labelsView loads the user's labels. confirm and saved name a label id
// for the delete question and the "Saved." line (0: none).
func (s *server) labelsView(r *http.Request, confirm, saved int64) (labelsView, error) {
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return labelsView{}, err
	}
	v := labelsView{Colors: todo.LabelColors}
	for _, l := range labels {
		v.Rows = append(v.Rows, labelRow{Label: l, NameValue: l.Name, ColorValue: l.Color,
			Confirm: l.ID == confirm, Saved: l.ID == saved})
	}
	return v, nil
}

// labelsPage shows the labels page. ?delete=<id> asks whether to delete that
// label, ?saved=<id> says that its last change was saved.
func (s *server) labelsPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	confirm, _ := strconv.ParseInt(q.Get("delete"), 10, 64)
	saved, _ := strconv.ParseInt(q.Get("saved"), 10, 64)
	v, err := s.labelsView(r, confirm, saved)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"labels", v})
}

// saveLabel renames and recolors a label, then goes back to the labels page.
func (s *server) saveLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.Redirect(w, r, "/labels", http.StatusSeeOther)
		return
	}
	name, color := r.PostFormValue("name"), r.PostFormValue("color")
	_, err := s.svc.UpdateLabel(r.Context(), userID(r), id, name, color)
	var msg string
	var le *todo.LabelError
	switch {
	case err == nil:
		http.Redirect(w, r, "/labels?saved="+strconv.FormatInt(id, 10)+"#label-"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	case errors.Is(err, todo.ErrNotFound):
		http.Redirect(w, r, "/labels", http.StatusSeeOther)
		return
	case errors.Is(err, todo.ErrLabelExists):
		msg = "You already have a label “" + name + "”."
	case errors.As(err, &le):
		msg = le.Msg
	default:
		s.serverError(w, r, err)
		return
	}
	v, err := s.labelsView(r, 0, 0)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	for i := range v.Rows {
		if v.Rows[i].ID == id {
			v.Rows[i].NameValue, v.Rows[i].Error = name, msg
			if color != "" {
				v.Rows[i].ColorValue = color
			}
		}
	}
	s.render(w, r, http.StatusUnprocessableEntity, part{"labels", v})
}

// deleteLabel deletes a label (its items stay), then shows the labels page.
func (s *server) deleteLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if ok {
		err := s.svc.DeleteLabel(r.Context(), userID(r), id)
		if err != nil && !errors.Is(err, todo.ErrNotFound) {
			s.serverError(w, r, err)
			return
		}
	}
	http.Redirect(w, r, "/labels", http.StatusSeeOther)
}
