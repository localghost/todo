package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"todo/internal/todo"
)

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	lv, err := s.listView(r, currentUser(r).HideDone)
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
	draft, err := s.newDraft(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	page := pageView{Form: formView{Focus: true}, Draft: draft, List: lv, Username: currentUser(r).Username}
	s.render(w, r, http.StatusOK, part{"page", page})
}

func (s *server) addItem(w http.ResponseWriter, r *http.Request) {
	// The due date and the labels come from #add-extras (draft.go), which
	// shows the list's filter label at the start.
	draft, err := s.readDraft(r)
	var dueErr *todo.DueError
	msg := ""
	if errors.As(err, &dueErr) {
		msg = dueErr.Msg
	} else if err != nil {
		s.serverError(w, r, err)
		return
	}
	in := todo.Input{Text: r.FormValue("text"), DueDate: draft.DueDate, DueTime: draft.DueTime, Labels: draft.labelNames()}
	var item todo.Item
	if msg == "" {
		item, err = s.svc.AddItem(r.Context(), userID(r), in, s.zoneFor(r))
		msg = textMessage(err, "Please type some text first.")
	}
	if msg != "" {
		w.Header().Set("HX-Retarget", "#add-form")
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity, part{"add-form", formView{Text: in.Text, Error: msg, Focus: true}})
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	fresh, err := s.newDraft(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	fresh.OOB = true
	lv, err := s.listView(r, currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	// In due date order an item with a due date goes to its date, and an item
	// without the filter label is not in the filtered list: the whole list
	// comes back. Else the new row goes at the end, overdue or not.
	if (lv.SortDue && item.DueAt != nil) || (lv.Filter != nil && !item.HasLabel(lv.Filter.ID)) {
		w.Header().Set("HX-Retarget", "#list-section")
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusOK, part{"list", lv}, part{"add-extras", fresh})
		return
	}
	lv.OOB = true
	// The add form is not part of the answer: app.js clears the input, so
	// text typed while this request ran is not lost.
	s.render(w, r, http.StatusOK, part{"item", item}, part{"oob", lv}, part{"add-extras", fresh})
}

func (s *server) toggleItem(w http.ResponseWriter, r *http.Request) {
	hideDone := currentUser(r).HideDone
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	old := s.oldItem(r, id)
	// The checkbox sends the wanted state (code review 4); a request with no
	// value, from a page loaded before this change, flips the item.
	var item todo.Item
	var err error
	switch r.PostFormValue("done") {
	case "1", "0":
		item, err = s.svc.SetDone(r.Context(), userID(r), id, r.PostFormValue("done") == "1")
	default:
		item, err = s.svc.Toggle(r.Context(), userID(r), id)
	}
	if errors.Is(err, todo.ErrNotFound) {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if placeChanged(old, item, s.nowFor(r), currentUser(r).SortDue) {
		s.renderList(w, r, hideDone) // in due date order: into or out of the overdue group
		return
	}
	lv, err := s.listView(r, hideDone)
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

// clearDoneConfirm answers with the question of "Clear done": how many done
// items go, and the label when the page is filtered. No done items: an
// empty answer.
func (s *server) clearDoneConfirm(w http.ResponseWriter, r *http.Request) {
	lv, err := s.listView(r, currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"clear-confirm", lv})
}

// clearDone deletes the done items (with a label filter only the ones with
// that label) and answers with the whole list.
func (s *server) clearDone(w http.ResponseWriter, r *http.Request) {
	filter, err := s.labelFilter(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	var labelID int64
	if filter != nil {
		labelID = filter.ID
	}
	// The question sends the number it showed. If another tab changed it,
	// delete nothing and ask again with the new number (code review 7).
	if shown := r.PostFormValue("count"); shown != "" && isHTMX(r) {
		lv, err := s.listView(r, currentUser(r).HideDone)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if strconv.Itoa(lv.Done) != shown {
			lv.Ask, lv.Changed = true, true
			s.render(w, r, http.StatusOK, part{"list", lv})
			return
		}
	}
	if _, err := s.svc.ClearDone(r.Context(), userID(r), labelID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if !isHTMX(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	lv, err := s.listView(r, currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"list", lv})
}

// oobOnly answers with an empty main body (htmx removes the target row)
// plus the out-of-band toolbar and empty state.
func (s *server) oobOnly(w http.ResponseWriter, r *http.Request, status int) {
	lv, err := s.listView(r, currentUser(r).HideDone)
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
	s.render(w, r, http.StatusOK, part{"item-edit", editView{Item: item, Text: item.Text}})
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
	text := r.FormValue("text")
	item, err := s.svc.EditText(r.Context(), userID(r), id, text)
	msg := textMessage(err, "Text cannot be empty.")
	switch {
	case msg != "":
		old, getErr := s.svc.Get(r.Context(), userID(r), id)
		if errors.Is(getErr, todo.ErrNotFound) {
			s.oobOnly(w, r, http.StatusNotFound)
			return
		}
		if getErr != nil {
			s.serverError(w, r, getErr)
			return
		}
		s.render(w, r, http.StatusUnprocessableEntity, part{"item-edit", editView{Item: old, Text: text, Error: msg}})
	case errors.Is(err, todo.ErrNotFound):
		s.oobOnly(w, r, http.StatusNotFound)
	case err != nil:
		s.serverError(w, r, err)
	default:
		// The text does not change the place, the counts, or the permission bar.
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
	old := s.oldItem(r, id)
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
		if placeChanged(old, item, s.nowFor(r), currentUser(r).SortDue) {
			s.renderList(w, r, currentUser(r).HideDone)
			return
		}
		lv, err := s.listView(r, currentUser(r).HideDone)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		lv.OOB = true
		s.render(w, r, http.StatusOK, part{"item", item}, part{"oob", lv})
	}
}

// moveItem puts an item right after the item in the form field "after"
// (empty: first). The page has already moved the row, so success is 204.
// On an error the page gets the whole list, to show the true order.
func (s *server) moveItem(w http.ResponseWriter, r *http.Request) {
	hideDone := currentUser(r).HideDone
	id, ok := parseID(r)
	after, okAfter := parseAfter(r.FormValue("after"))
	if !ok || !okAfter {
		s.renderListStatus(w, r, hideDone, http.StatusNotFound)
		return
	}
	// In due date order the page has no grips; a move from an old page
	// gets the true list.
	if currentUser(r).SortDue {
		s.renderListStatus(w, r, hideDone, http.StatusUnprocessableEntity)
		return
	}
	err := s.svc.Move(r.Context(), userID(r), id, after)
	switch {
	case errors.Is(err, todo.ErrNotFound):
		s.renderListStatus(w, r, hideDone, http.StatusNotFound)
	case err != nil:
		s.serverError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// parseAfter reads the form field "after": empty means 0, the first place.
func parseAfter(v string) (int64, bool) {
	if v == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(v, 10, 64)
	return id, err == nil && id > 0
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
	lv, err := s.listView(r, hide)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"list", lv})
}

// setSortDue saves whether the list is in due date order, then shows the list.
func (s *server) setSortDue(w http.ResponseWriter, r *http.Request) {
	sortDue := r.FormValue("sort_due") == "1"
	if err := s.accounts.SetSortDue(r.Context(), userID(r), sortDue); err != nil {
		s.serverError(w, r, err)
		return
	}
	if !isHTMX(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	lv, err := s.listViewOrder(r, currentUser(r).HideDone, sortDue)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"list", lv})
}

// oldItem returns item id before a change (a zero item if it is missing).
func (s *server) oldItem(r *http.Request, id int64) todo.Item {
	it, _ := s.svc.Get(r.Context(), userID(r), id)
	return it
}

// placeChanged reports whether a change moves the item in the list. By hand
// nothing moves it. In due date order (sortDue) a change of the due date moves
// it, and so does done or not done, which takes it into or out of the overdue
// group.
func placeChanged(old, item todo.Item, now time.Time, sortDue bool) bool {
	if !sortDue {
		return false
	}
	return old.Overdue(now) != item.Overdue(now) || !sameDue(old, item)
}

// sameDue reports whether a and b have the same due date.
func sameDue(a, b todo.Item) bool {
	if a.DueAt == nil || b.DueAt == nil {
		return a.DueAt == b.DueAt
	}
	return a.DueAt.Equal(*b.DueAt) && a.DueAllDay == b.DueAllDay
}

// renderList answers with the whole list section instead of one row, for a
// change that moves an item to another place.
func (s *server) renderList(w http.ResponseWriter, r *http.Request, hideDone bool) {
	s.renderListStatus(w, r, hideDone, http.StatusOK)
}

// renderListStatus is renderList with another status code.
func (s *server) renderListStatus(w http.ResponseWriter, r *http.Request, hideDone bool, status int) {
	lv, err := s.listView(r, hideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("HX-Retarget", "#list-section")
	w.Header().Set("HX-Reswap", "outerHTML")
	s.render(w, r, status, part{"list", lv})
}

// textMessage returns the message for an item text that the service refused,
// or "" if err is another error. empty is the message for an empty text.
func textMessage(err error, empty string) string {
	switch {
	case errors.Is(err, todo.ErrEmptyText):
		return empty
	case errors.Is(err, todo.ErrTextTooLong):
		return fmt.Sprintf("The text is too long: at most %d characters.", todo.MaxTextChars)
	case errors.Is(err, todo.ErrTooManyItems):
		return fmt.Sprintf("You have %s items. Delete some first.", groupDigits(todo.MaxItemsPerUser))
	}
	return ""
}

// groupDigits writes n with a comma between each group of three digits.
func groupDigits(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
