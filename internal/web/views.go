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
}

// formView is the data for the add form.
type formView struct {
	Text  string
	Error bool
	Focus bool
}

// editView is the data for a row in edit mode.
type editView struct {
	Item  todo.Item
	Text  string
	Error bool
}

// pageView is the data for the full page.
type pageView struct {
	Form formView
	List listView
}

func (s *server) listView(ctx context.Context, hideDone bool) (listView, error) {
	items, err := s.svc.List(ctx, defaultUserID, hideDone)
	if err != nil {
		return listView{}, err
	}
	open, done, err := s.svc.Counts(ctx, defaultUserID)
	if err != nil {
		return listView{}, err
	}
	return listView{Items: items, Open: open, Done: done, Total: open + done, HideDone: hideDone}, nil
}
