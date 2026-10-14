package web

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"

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
	Filter      *todo.Label // the list shows only items with this label
}

// formView is the data for the add form.
type formView struct {
	Text        string
	DueDate     string
	DueTime     string
	Labels      string
	LabelsStart string // the Labels field's value after an add: the filter's label
	Error       bool   // empty text
	DueError    string // message for a bad due date
	LabelsError string // message for bad labels
	Focus       bool
	Suggest     []todo.Label
}

// editView is the data for a row in edit mode.
type editView struct {
	Item        todo.Item
	Text        string
	DueDate     string
	DueTime     string
	Labels      string
	Error       bool   // empty text
	DueError    string // message for a bad due date
	LabelsError string // message for bad labels
	Suggest     []todo.Label
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
	filter, err := s.labelFilter(r)
	if err != nil {
		return listView{}, err
	}
	var labelID int64
	if filter != nil {
		labelID = filter.ID
	}
	items, err := s.svc.ListLabel(ctx, userID, hideDone, labelID)
	if err != nil {
		return listView{}, err
	}
	open, done, err := s.svc.CountsLabel(ctx, userID, labelID)
	if err != nil {
		return listView{}, err
	}
	items = todo.OverdueFirst(items, now)
	lv := listView{Items: items, Open: open, Done: done, Total: open + done, HideDone: hideDone, Filter: filter}
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

// labelFilter returns the label in ?label= (nil: no filter). An htmx request
// from the page carries the page's address in HX-Current-URL, so a row or
// toolbar answer is filtered like the page. An unknown label is no filter.
func (s *server) labelFilter(r *http.Request) (*todo.Label, error) {
	v := r.URL.Query().Get("label")
	if v == "" && isHTMX(r) {
		if u, err := url.Parse(r.Header.Get("HX-Current-URL")); err == nil && u.Path == "/" {
			v = u.Query().Get("label")
		}
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if v == "" || err != nil || id <= 0 {
		return nil, nil
	}
	l, err := s.svc.Label(r.Context(), userID(r), id)
	if errors.Is(err, todo.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// maxSuggest is the number of label buttons under a Labels field.
const maxSuggest = 8

// suggestions returns the user's most used labels, for the buttons under a
// Labels field.
func (s *server) suggestions(r *http.Request) ([]todo.Label, error) {
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(labels, func(a, b todo.Label) int { return b.Items - a.Items })
	if len(labels) > maxSuggest {
		labels = labels[:maxSuggest]
	}
	return labels, nil
}

// newForm returns the add form for the request: the Labels field starts with
// the filter's label, so a new item shows in the filtered list.
func (s *server) newForm(r *http.Request, filter *todo.Label) (formView, error) {
	suggest, err := s.suggestions(r)
	if err != nil {
		return formView{}, err
	}
	f := formView{Focus: true, Suggest: suggest}
	if filter != nil {
		f.Labels, f.LabelsStart = filter.Name, filter.Name
	}
	return f, nil
}

// labelsMessage returns the message for a label error from the service, or "".
func labelsMessage(err error) string {
	var le *todo.LabelError
	switch {
	case errors.As(err, &le):
		return le.Msg
	case errors.Is(err, todo.ErrTooManyLabels):
		return "You have 50 labels. Delete one on the Labels page first."
	}
	return ""
}
