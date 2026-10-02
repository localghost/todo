package web

import (
	"html/template"
	"strings"
	"testing"
	"time"
)

// One client can send every zone name; the cache of template sets must stay small.
func TestZoneTemplateCacheIsBounded(t *testing.T) {
	s := &server{now: time.Now}
	base, err := template.New("").Funcs(s.zoneFuncs(time.UTC)).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	template.Must(base.New("zone-probe").Parse("{{zone}}"))
	s.base = base
	for i := -12; i <= 14; i++ {
		loc := time.FixedZone("Z"+time.Duration(i).String(), i*3600)
		tmpl, err := s.templatesFor(loc)
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := tmpl.ExecuteTemplate(&out, "zone-probe", nil); err != nil {
			t.Fatal(err)
		}
		if out.String() != loc.String() {
			t.Fatalf("zone %s rendered as %q", loc, out.String())
		}
	}
	n := 0
	s.views.Range(func(_, _ any) bool { n++; return true })
	if n > maxZoneSets {
		t.Fatalf("%d cached template sets, want at most %d", n, maxZoneSets)
	}
}
