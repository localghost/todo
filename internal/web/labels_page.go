package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"todo/internal/todo"
)

// The Labels page (canvas page "16 · Labels", board "Labels page again"):
// create a label, change its color, delete it, and see how many items use
// it. labels.js filters the lines as the user types, opens the color wheel
// and asks before a delete.

// labelsView is the data for the Labels page and its section.
type labelsView struct {
	Username string
	Labels   []labelLine
	NewColor string // the color of the next new label
	Value    string // the typed name, after an error
	Error    string
}

// labelLine is one label on the page.
type labelLine struct {
	todo.Label
	Count string // "3 items", "1 item", "no items"
}

func itemCount(n int) string {
	switch n {
	case 0:
		return "no items"
	case 1:
		return "1 item"
	}
	return strconv.Itoa(n) + " items"
}

func (s *server) labelsView(r *http.Request) (labelsView, error) {
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return labelsView{}, err
	}
	v := labelsView{Username: currentUser(r).Username, NewColor: todo.LabelColors[len(labels)%len(todo.LabelColors)]}
	for _, l := range labels {
		v.Labels = append(v.Labels, labelLine{Label: l, Count: itemCount(l.Items)})
	}
	return v, nil
}

// labelsPage shows the Labels page.
func (s *server) labelsPage(w http.ResponseWriter, r *http.Request) {
	v, err := s.labelsView(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"labels-page", v})
}

// createLabel makes the label in the form field "name" with "color". The
// answer is the section of the page; on a bad name, or a name that a label
// has already (ignoring case), it keeps the name and shows why.
func (s *server) createLabel(w http.ResponseWriter, r *http.Request) {
	name, color := r.FormValue("name"), r.FormValue("color")
	msg, err := s.existingLabelMessage(r, name)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if msg == "" {
		_, err = s.svc.CreateLabel(r.Context(), userID(r), name, color)
		msg = labelsMessage(err)
		if err != nil && msg == "" {
			s.serverError(w, r, err)
			return
		}
	}
	v, verr := s.labelsView(r)
	if verr != nil {
		s.serverError(w, r, verr)
		return
	}
	status := http.StatusOK
	if msg != "" {
		status = http.StatusUnprocessableEntity
		v.Value, v.Error = name, msg
		if c, ok := todo.CleanColor(color); ok {
			v.NewColor = c
		}
	}
	if !isHTMX(r) {
		// Without JavaScript the form posts the page itself.
		s.render(w, r, status, part{"labels-page", v})
		return
	}
	s.render(w, r, status, part{"labels-section", v})
}

// existingLabelMessage says so if the user has a label with this name
// already (ignoring case and extra spaces), else "".
func (s *server) existingLabelMessage(r *http.Request, name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "", nil
	}
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return "", err
	}
	for _, l := range labels {
		if strings.EqualFold(l.Name, name) {
			return fmt.Sprintf("You have a label “%s” already.", l.Name), nil
		}
	}
	return "", nil
}
