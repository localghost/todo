package web

import (
	"errors"
	"net/http"
	"strconv"

	"todo/internal/todo"
)

// duePicker shows the due date pop-up that the calendar icon opens.
func (s *server) duePicker(w http.ResponseWriter, r *http.Request) {
	item, ok := s.findItem(w, r)
	if !ok {
		return
	}
	date, clock := dueInputs(item, s.zoneFor(r))
	s.render(w, r, http.StatusOK, part{"due-picker", duePickerView{Item: item, DueDate: date, DueTime: clock, Repeat: item.Repeat}})
}

// setItemDue sets the due date of an item from the pop-up, or removes it
// when both fields are empty. The answer is the row, or the whole list if
// the item moves; on a bad value it is the pop-up with a message.
func (s *server) setItemDue(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	date, clock, repeat := r.FormValue("due_date"), r.FormValue("due_time"), r.FormValue("due_repeat")
	// Read the time in the zone the pop-up was shown in, so an unchanged
	// time stays the same moment even if the browser's zone changed since.
	loc, ok := s.parseZone(r.FormValue("due_tz"))
	if !ok {
		loc = s.zoneFor(r)
	}
	old := s.oldItem(r, id)
	item, err := s.svc.SetDueRepeat(r.Context(), userID(r), id, date, clock, repeat, loc)
	var dueErr *todo.DueError
	switch {
	case errors.As(err, &dueErr):
		cur, getErr := s.svc.Get(r.Context(), userID(r), id)
		if errors.Is(getErr, todo.ErrNotFound) {
			s.oobOnly(w, r, http.StatusNotFound)
			return
		}
		if getErr != nil {
			s.serverError(w, r, getErr)
			return
		}
		w.Header().Set("HX-Retarget", "#due-picker-"+strconv.FormatInt(id, 10))
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity,
			part{"due-picker", duePickerView{Item: cur, DueDate: date, DueTime: clock, Repeat: repeat, Error: dueErr.Msg}})
	case errors.Is(err, todo.ErrNotFound):
		s.oobOnly(w, r, http.StatusNotFound)
	case err != nil:
		s.serverError(w, r, err)
	default:
		if placeChanged(old, item, s.nowFor(r), currentUser(r).SortDue) {
			s.renderList(w, r, currentUser(r).HideDone)
			return
		}
		// The permission bar depends on the due dates, so it comes along.
		lv, err := s.listView(r, currentUser(r).HideDone)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		lv.OOB = true
		s.render(w, r, http.StatusOK, part{"item", item}, part{"oob", lv})
	}
}
