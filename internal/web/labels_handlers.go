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

// pickerView loads the label list for item: the user's labels that the
// item does not have yet.
func (s *server) pickerView(r *http.Request, item todo.Item) (pickerView, error) {
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return pickerView{}, err
	}
	v := pickerView{Item: item, Taken: todo.LabelNames(item.Labels), Full: len(item.Labels) >= todo.MaxLabelsPerItem}
	for _, l := range labels {
		if !item.HasLabel(l.ID) {
			v.Options = append(v.Options, l)
		}
	}
	return v, nil
}

// labelPicker shows the label list that + opens on an item.
func (s *server) labelPicker(w http.ResponseWriter, r *http.Request) {
	item, ok := s.findItem(w, r)
	if !ok {
		return
	}
	v, err := s.pickerView(r, item)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"label-picker", v})
}

// addItemLabel puts the label in the form field "name" on an item, making
// the label if the user has none of that name. The answer is the row; on a
// bad name it is the label list with a message.
func (s *server) addItemLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	name := r.FormValue("name")
	item, err := s.svc.AddLabel(r.Context(), userID(r), id, name)
	if msg := labelsMessage(err); msg != "" {
		old, getErr := s.svc.Get(r.Context(), userID(r), id)
		if errors.Is(getErr, todo.ErrNotFound) {
			s.oobOnly(w, r, http.StatusNotFound)
			return
		}
		if getErr != nil {
			s.serverError(w, r, getErr)
			return
		}
		v, verr := s.pickerView(r, old)
		if verr != nil {
			s.serverError(w, r, verr)
			return
		}
		v.Value, v.Error = name, msg
		w.Header().Set("HX-Retarget", "#picker-"+strconv.FormatInt(id, 10))
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity, part{"label-picker", v})
		return
	}
	s.renderLabeled(w, r, item, err)
}

// removeItemLabel takes a label off an item. The label itself stays.
func (s *server) removeItemLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	labelID, err := strconv.ParseInt(r.PathValue("label"), 10, 64)
	if !ok || err != nil {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	item, err := s.svc.RemoveLabel(r.Context(), userID(r), id, labelID)
	s.renderLabeled(w, r, item, err)
}

// renderLabeled answers a change of an item's labels: the row and the
// toolbar. A row that lost the label of the list's filter leaves the list.
func (s *server) renderLabeled(w http.ResponseWriter, r *http.Request, item todo.Item, err error) {
	if errors.Is(err, todo.ErrNotFound) {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv, err := s.listView(r, currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	lv.OOB = true
	parts := []part{}
	if lv.Filter == nil || item.HasLabel(lv.Filter.ID) {
		parts = append(parts, part{"item", item})
	}
	s.render(w, r, http.StatusOK, append(parts, part{"oob", lv})...)
}
