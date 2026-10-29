package web

import (
	"errors"
	"net/http"
	"strconv"

	"todo/internal/todo"
)

// setLabelColor saves a label's color. The page changes its chips itself.
func (s *server) setLabelColor(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := s.svc.SetLabelColor(r.Context(), userID(r), id, r.FormValue("color"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, todo.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, todo.ErrBadLabel):
		http.Error(w, "Pick a color.", http.StatusUnprocessableEntity)
	default:
		s.serverError(w, r, err)
	}
}

// deleteLabel deletes a label: it comes off all items, the items stay. The
// answer is the section of the Labels page; a label that is already gone
// (deleted in another tab) answers 404 with the section as it is now.
func (s *server) deleteLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	status := http.StatusOK
	if err := s.svc.DeleteLabel(r.Context(), userID(r), id); errors.Is(err, todo.ErrNotFound) {
		status = http.StatusNotFound
	} else if err != nil {
		s.serverError(w, r, err)
		return
	}
	v, err := s.labelsView(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, status, part{"labels-section", v})
}

// pickerView loads the label list for item: all the user's labels, and the
// color the next new label gets (as the store gives it).
func (s *server) pickerView(r *http.Request, item todo.Item) (pickerView, error) {
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return pickerView{}, err
	}
	v := pickerView{Item: item, Full: len(item.Labels) >= todo.MaxLabelsPerItem,
		NewColor: todo.LabelColors[len(labels)%len(todo.LabelColors)]}
	for _, l := range labels {
		v.Options = append(v.Options, pickerOption{Label: l, On: item.HasLabel(l.ID)})
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
	name, color := r.FormValue("name"), r.FormValue("color")
	item, err := s.svc.AddLabel(r.Context(), userID(r), id, name, color)
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
		if c, ok := todo.CleanColor(color); ok {
			v.NewColor = c
		}
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
