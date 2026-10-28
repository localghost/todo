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
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// WithGuide serves the user guide from fsys: user-guide.md at /guide and the
// PNG files in guide-images/ at /guide-images/. If fsys has
// privacy-policy.md, it serves that at /privacy. Without it, all answer 404.
func WithGuide(fsys fs.FS) Option { return func(s *server) { s.guideFS = fsys } }

// guideView is the user guide or the privacy policy, rendered once at start
// (canvas page 2, "user guide at /guide" and "privacy policy at /privacy").
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
	gm := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(figureRenderer{}, 100))))
	doc := gm.Parser().Parse(text.NewReader(md))
	v := &guideView{}
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if p, ok := n.(*ast.Paragraph); ok && p.ChildCount() == 1 {
			if img, ok := p.FirstChild().(*ast.Image); ok && len(img.Title) > 0 {
				f := &figure{Caption: img.Title}
				img.Title = nil
				p.RemoveChild(p, img)
				f.AppendChild(f, img)
				doc.ReplaceChild(doc, p, f)
				n = f
			}
		}
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

// figure is a screenshot with its caption: an image alone in a paragraph,
// with a title (canvas page 2, "centered screenshots", option B). The title
// is the caption, above the image.
type figure struct {
	ast.BaseBlock
	Caption []byte
}

var kindFigure = ast.NewNodeKind("Figure")

func (f *figure) Kind() ast.NodeKind { return kindFigure }

func (f *figure) Dump(source []byte, level int) { ast.DumpHelper(f, source, level, nil, nil) }

type figureRenderer struct{}

func (figureRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindFigure, func(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString("<figure>\n<figcaption><b>Screenshot:</b> ")
			_, _ = w.Write(util.EscapeHTML(n.(*figure).Caption))
			_, _ = w.WriteString("</figcaption>\n")
		} else {
			_, _ = w.WriteString("\n</figure>\n")
		}
		return ast.WalkContinue, nil
	})
}

// guidePage shows the user guide. It needs no session.
func (s *server) guidePage(w http.ResponseWriter, r *http.Request) {
	if s.guide == nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, part{"guide", docPage{"User guide", s.guide, s.privacy != nil}})
}

// docPage is the data of the guide template: a page from docs/.
type docPage struct {
	Title       string
	Doc         *guideView
	PrivacyLink bool // a link to /privacy in the top line
}

// privacyPage shows the privacy policy (canvas page 2, "privacy policy at
// /privacy", option A). It needs no session.
func (s *server) privacyPage(w http.ResponseWriter, r *http.Request) {
	if s.privacy == nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, part{"guide", docPage{"Privacy policy", s.privacy, false}})
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
