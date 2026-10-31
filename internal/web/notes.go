package web

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"todo/internal/todo"
)

// Notes on an item (canvas page "21 · Notes", option N2): the notes button
// on a row opens a pop-up that shows the notes, with "Edit", or the field.

// notesPickerView is the data for the notes pop-up of an item.
type notesPickerView struct {
	Item  todo.Item
	Notes string // the field (what the user typed, after an error)
	Edit  bool   // show the field, not the notes
	Error string // why the last save failed
}

// notesPicker shows the notes pop-up. An item without notes opens it with
// the field.
func (s *server) notesPicker(w http.ResponseWriter, r *http.Request) {
	item, ok := s.findItem(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, part{"notes-picker", notesPickerView{Item: item, Notes: item.Notes, Edit: item.Notes == ""}})
}

// setItemNotes saves the notes from the pop-up. The answer is the row; on
// notes that are too long it is the pop-up with a message.
func (s *server) setItemNotes(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.oobOnly(w, r, http.StatusNotFound)
		return
	}
	notes := r.FormValue("notes")
	item, err := s.svc.SetNotes(r.Context(), userID(r), id, notes)
	switch {
	case errors.Is(err, todo.ErrNotesTooLong):
		cur, getErr := s.svc.Get(r.Context(), userID(r), id)
		if errors.Is(getErr, todo.ErrNotFound) {
			s.oobOnly(w, r, http.StatusNotFound)
			return
		}
		if getErr != nil {
			s.serverError(w, r, getErr)
			return
		}
		w.Header().Set("HX-Retarget", "#notes-picker-"+strconv.FormatInt(id, 10))
		w.Header().Set("HX-Reswap", "outerHTML")
		s.render(w, r, http.StatusUnprocessableEntity,
			part{"notes-picker", notesPickerView{Item: cur, Notes: notes, Edit: true, Error: notesMessage(err)}})
	case errors.Is(err, todo.ErrNotFound):
		s.oobOnly(w, r, http.StatusNotFound)
	case err != nil:
		s.serverError(w, r, err)
	default:
		s.render(w, r, http.StatusOK, part{"item", item})
	}
}

// notesMessage returns the message for notes that the service refused, or
// "" if err is another error.
func notesMessage(err error) string {
	if errors.Is(err, todo.ErrNotesTooLong) {
		return fmt.Sprintf("The notes are too long: at most %d characters.", todo.MaxNotesChars)
	}
	return ""
}

// linkRE finds the http and https links in notes.
var linkRE = regexp.MustCompile(`https?://[^\s<>"]+`)

// linkify escapes notes for HTML and makes each http or https link in them
// a link that opens in a new tab. Punctuation at the end of a link (a
// period, or a bracket that closes outside it) stays text.
func linkify(notes string) template.HTML {
	var b strings.Builder
	last := 0
	for _, m := range linkRE.FindAllStringIndex(notes, -1) {
		link := trimLink(notes[m[0]:m[1]])
		if link == "http://" || link == "https://" {
			continue // only punctuation after the scheme
		}
		b.WriteString(template.HTMLEscapeString(notes[last:m[0]]))
		esc := template.HTMLEscapeString(link)
		fmt.Fprintf(&b, `<a href="%s" target="_blank" rel="noopener noreferrer nofollow">%s</a>`, esc, esc)
		last = m[0] + len(link)
	}
	b.WriteString(template.HTMLEscapeString(notes[last:]))
	return template.HTML(b.String())
}

// trimLink removes the punctuation at the end of a link: ".,;:!?'" and a
// closing bracket that has no opening one in the link.
func trimLink(link string) string {
	for link != "" {
		c := link[len(link)-1]
		switch {
		case strings.IndexByte(".,;:!?'", c) >= 0:
		case c == ')' && strings.Count(link, "(") < strings.Count(link, ")"):
		case c == ']' && strings.Count(link, "[") < strings.Count(link, "]"):
		default:
			return link
		}
		link = link[:len(link)-1]
	}
	return link
}
