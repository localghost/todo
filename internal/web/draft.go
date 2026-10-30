package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"todo/internal/todo"
)

// The add form's due date and labels (canvas page "10 · Due date", board
// "calendar and + under the add field", option B). The user picks them
// before the item exists, so the page keeps them in hidden fields of
// #add-extras, and the server draws that part again after each change. The
// pop-ups are the same as on a row, for the item with id 0.

// draftView is the data for #add-extras.
type draftView struct {
	DueDate string // as the date field wants it, or ""
	DueTime string // HH:MM, or "" for all day
	DueText string // "Tomorrow, 10:00", or "" without a due date
	// DueRepeat is the repeat rule of the due date ("": never).
	DueRepeat string
	Labels    []todo.Label
	OOB       bool // sent next to the new item, to start again
}

// LabelIDs is the hidden field: the ids of the picked labels, with commas.
func (d draftView) LabelIDs() string {
	ids := make([]string, len(d.Labels))
	for i, l := range d.Labels {
		ids[i] = strconv.FormatInt(l.ID, 10)
	}
	return strings.Join(ids, ",")
}

// DueURL opens the due date pop-up with the picked date in its fields.
func (d draftView) DueURL() string {
	v := url.Values{"due_date": {d.DueDate}, "due_time": {d.DueTime}}
	if d.DueRepeat != "" {
		v.Set("due_repeat", d.DueRepeat)
	}
	return "/new/due?" + v.Encode()
}

// LabelsURL opens the label list with the picked labels ticked.
func (d draftView) LabelsURL() string {
	return "/new/labels?" + url.Values{"labels": {d.LabelIDs()}}.Encode()
}

// labelNames is the labels as todo.Input wants them.
func (d draftView) labelNames() string {
	return todo.LabelNames(d.Labels)
}

// draftItem is the item that the pop-ups show for the add form.
func (d draftView) draftItem() todo.Item {
	return todo.Item{Text: "the new item", Labels: d.Labels}
}

// newDraft starts the add form's choices: no due date, and the list's
// filter label, so a new item shows in the filtered list.
func (s *server) newDraft(r *http.Request) (draftView, error) {
	filter, err := s.labelFilter(r)
	if filter == nil || err != nil {
		return draftView{}, err
	}
	return draftView{Labels: []todo.Label{*filter}}, nil
}

// readDraft reads the add form's choices from the request. Labels that are
// gone (deleted in another tab) drop out. A bad due date gives a
// todo.DueError, with the rest of the draft.
func (s *server) readDraft(r *http.Request) (draftView, error) {
	d := draftView{DueDate: strings.TrimSpace(r.FormValue("due_date"))}
	all, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return d, err
	}
	for _, v := range strings.Split(r.FormValue("labels"), ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil || hasLabel(d.Labels, id) || len(d.Labels) >= todo.MaxLabelsPerItem {
			continue
		}
		for _, l := range all {
			if l.ID == id {
				d.Labels = append(d.Labels, l)
			}
		}
	}
	return d, s.setDraftDue(r, &d, d.DueDate, r.FormValue("due_time"), r.FormValue("due_repeat"))
}

// setDraftDue checks a due date and puts it in d, in the form the fields
// show it. On a bad value d keeps no due date.
func (s *server) setDraftDue(r *http.Request, d *draftView, date, clock, repeat string) error {
	d.DueDate, d.DueTime, d.DueText, d.DueRepeat = "", "", "", ""
	loc := s.zoneFor(r)
	at, allDay, err := todo.ParseDue(date, clock, loc)
	if err != nil {
		return err
	}
	if err := todo.CheckRepeat(repeat, at); err != nil || at == nil {
		return err
	}
	d.DueRepeat = repeat
	it := todo.Item{DueAt: at, DueAllDay: allDay}
	d.DueDate, d.DueTime = dueInputs(it, loc)
	d.DueText = upperFirst(dueWhen(at.In(loc), allDay, s.nowFor(r)))
	return nil
}

func hasLabel(labels []todo.Label, id int64) bool {
	for _, l := range labels {
		if l.ID == id {
			return true
		}
	}
	return false
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// draftDuePicker shows the due date pop-up of the add form.
func (s *server) draftDuePicker(w http.ResponseWriter, r *http.Request) {
	d, err := s.readDraft(r)
	if err != nil && !errors.Is(err, todo.ErrBadDue) {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"due-picker", duePickerView{Item: d.draftItem(), Draft: true, DueDate: d.DueDate, DueTime: d.DueTime, Repeat: d.DueRepeat}})
}

// setDraftDueDate saves the due date of the pop-up in the add form, or
// removes it (both fields empty, or "clear"). The answer is #add-extras; on
// a bad value it is the pop-up with a message.
func (s *server) setDraftDueDate(w http.ResponseWriter, r *http.Request) {
	d, err := s.readDraft(r)
	if err != nil && !errors.Is(err, todo.ErrBadDue) {
		s.serverError(w, r, err)
		return
	}
	date, clock, repeat := r.FormValue("due_date"), r.FormValue("due_time"), r.FormValue("due_repeat")
	if r.FormValue("clear") != "" {
		date, clock, repeat = "", "", ""
	}
	var dueErr *todo.DueError
	if err := s.setDraftDue(r, &d, date, clock, repeat); errors.As(err, &dueErr) {
		w.Header().Set("HX-Retarget", "#due-picker-0")
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity,
			part{"due-picker", duePickerView{Item: d.draftItem(), Draft: true, DueDate: date, DueTime: clock, Repeat: repeat, Error: dueErr.Msg}})
		return
	}
	s.render(w, r, http.StatusOK, part{"add-extras", d})
}

// draftPickerView loads the label list of the add form.
func (s *server) draftPickerView(r *http.Request, d draftView) (pickerView, error) {
	v, err := s.pickerView(r, d.draftItem())
	v.Draft = true
	return v, err
}

// draftLabelPicker shows the label list that "Label" opens in the add form.
func (s *server) draftLabelPicker(w http.ResponseWriter, r *http.Request) {
	d, err := s.readDraft(r)
	if err != nil && !errors.Is(err, todo.ErrBadDue) {
		s.serverError(w, r, err)
		return
	}
	v, err := s.draftPickerView(r, d)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"label-picker", v})
}

// changeDraftLabels picks the label in the form field "name" for the add
// form, making the label (with "color") if the user has none of that name,
// or takes off the label "remove". The answer is #add-extras; on a bad
// name it is the label list with a message.
func (s *server) changeDraftLabels(w http.ResponseWriter, r *http.Request) {
	d, err := s.readDraft(r)
	if err != nil && !errors.Is(err, todo.ErrBadDue) {
		s.serverError(w, r, err)
		return
	}
	if v := r.FormValue("remove"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		kept := d.Labels[:0]
		for _, l := range d.Labels {
			if l.ID != id {
				kept = append(kept, l)
			}
		}
		d.Labels = kept
		s.render(w, r, http.StatusOK, part{"add-extras", d})
		return
	}
	name, color := r.FormValue("name"), r.FormValue("color")
	var l todo.Label
	if len(d.Labels) >= todo.MaxLabelsPerItem {
		err = &todo.LabelError{Msg: "An item can have at most 5 labels."}
	} else {
		l, err = s.svc.CreateLabel(r.Context(), userID(r), name, color)
	}
	if msg := labelsMessage(err); msg != "" {
		v, verr := s.draftPickerView(r, d)
		if verr != nil {
			s.serverError(w, r, verr)
			return
		}
		v.Value, v.Error = name, msg
		if c, ok := todo.CleanColor(color); ok {
			v.NewColor = c
		}
		w.Header().Set("HX-Retarget", "#picker-0")
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity, part{"label-picker", v})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if !hasLabel(d.Labels, l.ID) {
		d.Labels = append(d.Labels, l)
	}
	s.render(w, r, http.StatusOK, part{"add-extras", d})
}
