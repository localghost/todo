package web

import (
	"net/http"

	"todo/internal/todo"
)

// listView is the data for the list section, the toolbar, and the empty state.
type listView struct {
	Items    []todo.Item
	Open     int
	Done     int
	Total    int
	HideDone bool
	OOB      bool // render toolbar and empty state as htmx out-of-band swaps
	HasDue   bool // an open item has a due date
	// NextOverdue is the Unix time in ms when the next open item becomes
	// overdue (0: none); app.js refreshes the list then.
	NextOverdue int64
}

// formView is the data for the add form.
type formView struct {
	Text     string
	DueDate  string
	DueTime  string
	Error    bool   // empty text
	DueError string // message for a bad due date
	Focus    bool
}

// editView is the data for a row in edit mode.
type editView struct {
	Item     todo.Item
	Text     string
	DueDate  string
	DueTime  string
	Error    bool   // empty text
	DueError string // message for a bad due date
}

// pageView is the data for the full page.
type pageView struct {
	Form     formView
	List     listView
	Username string
}

// listView loads the list of the request's user: overdue items first, most
// overdue first, in the browser's zone.
func (s *server) listView(r *http.Request, hideDone bool) (listView, error) {
	ctx, userID, now := r.Context(), userID(r), s.nowFor(r)
	items, err := s.svc.List(ctx, userID, hideDone)
	if err != nil {
		return listView{}, err
	}
	open, done, err := s.svc.Counts(ctx, userID)
	if err != nil {
		return listView{}, err
	}
	items = todo.OverdueFirst(items, now)
	lv := listView{Items: items, Open: open, Done: done, Total: open + done, HideDone: hideDone}
	if next, ok := todo.NextOverdue(items, now); ok {
		lv.NextOverdue = next.UnixMilli()
	}
	for _, it := range items {
		if !it.Done && it.DueAt != nil {
			lv.HasDue = true
			break
		}
	}
	return lv, nil
}
