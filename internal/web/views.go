package web

import (
	"context"

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

func (s *server) listView(ctx context.Context, userID int64, hideDone bool) (listView, error) {
	items, err := s.svc.List(ctx, userID, hideDone)
	if err != nil {
		return listView{}, err
	}
	open, done, err := s.svc.Counts(ctx, userID)
	if err != nil {
		return listView{}, err
	}
	lv := listView{Items: items, Open: open, Done: done, Total: open + done, HideDone: hideDone}
	for _, it := range items {
		if !it.Done && it.DueAt != nil {
			lv.HasDue = true
			break
		}
	}
	return lv, nil
}
