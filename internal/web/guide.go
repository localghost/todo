package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// WithGuide serves the user guide from fsys: user-guide.md at /guide and the
// PNG files in guide-images/ at /guide-images/. Without it, both answer 404.
func WithGuide(fsys fs.FS) Option { return func(s *server) { s.guideFS = fsys } }

// guideView is the user guide, rendered once at start (canvas page 2,
// "user guide at /guide").
type guideView struct {
	Intro    template.HTML  // the title and the text before the first section
	Sections []guideSection // the level-2 headings, for the contents list
	Body     template.HTML  // the sections
}

type guideSection struct {
	ID    string
	Title string
}

// renderGuide turns the Markdown guide into HTML. Raw HTML in the Markdown is
// not rendered (goldmark's default), so the guide cannot add scripts.
func renderGuide(md []byte) (*guideView, error) {
	gm := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithParserOptions(parser.WithAutoHeadingID()))
	doc := gm.Parser().Parse(text.NewReader(md))
	v := &guideView{}
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok && h.Level == 2 {
			id, _ := h.AttributeString("id")
			idBytes, _ := id.([]byte)
			v.Sections = append(v.Sections, guideSection{ID: string(idBytes), Title: string(h.Lines().Value(md))})
		}
	}
	var buf bytes.Buffer
	if err := gm.Renderer().Render(&buf, md, doc); err != nil {
		return nil, err
	}
	html := buf.String()
	cut := strings.Index(html, "<h2")
	if cut < 0 {
		cut = len(html)
	}
	v.Intro, v.Body = template.HTML(html[:cut]), template.HTML(html[cut:])
	return v, nil
}

// guidePage shows the user guide. It needs no session.
func (s *server) guidePage(w http.ResponseWriter, r *http.Request) {
	if s.guide == nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, part{"guide", s.guide})
}

// guideImages serves the screenshots of the guide: PNG files only, no folder
// listing.
func (s *server) guideImages() http.Handler {
	if s.guideFS == nil {
		return http.NotFoundHandler()
	}
	images, err := fs.Sub(s.guideFS, "guide-images")
	if err != nil {
		return http.NotFoundHandler()
	}
	files := http.StripPrefix("/guide-images/", http.FileServerFS(images))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Ext(r.URL.Path) != ".png" {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
