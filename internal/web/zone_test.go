package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func tz(name string) map[string]string { return map[string]string{"Cookie": "todo_tz=" + name} }

var dataTZ = regexp.MustCompile(`<body data-tz="([^"]*)">`)

func bodyZone(t *testing.T, body string) string {
	t.Helper()
	m := dataTZ.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no <body data-tz> in page")
	}
	return m[1]
}

// "Today" and "tomorrow" change at midnight in the browser's zone.
func TestDueLabelUsesBrowserZone(t *testing.T) {
	env := newTestEnv(t)
	warsaw := mustZone(t, "Europe/Warsaw")
	env.Clock.t = time.Date(2026, 10, 2, 23, 30, 0, 0, warsaw) // 22:30 in London
	if _, err := env.Svc.AddIn(context.Background(), env.User.ID, "x", "2026-10-03", "00:30", warsaw); err != nil {
		t.Fatal(err)
	}
	assertContains(t, do(t, env.H, "GET", "/", nil, tz("Europe/Warsaw")).Body.String(), "Due tomorrow, 00:30")
	assertContains(t, do(t, env.H, "GET", "/", nil, tz("Europe/London")).Body.String(), "Due today, 23:30")
}

// An item saved in Tokyo is the same moment when it is read in Warsaw.
func TestAddReadsDueInBrowserZone(t *testing.T) {
	env := newTestEnv(t)
	env.Clock.t = time.Date(2026, 10, 1, 12, 0, 0, 0, mustZone(t, "Europe/Warsaw"))
	form := url.Values{"text": {"Call"}, "due_date": {"2026-10-03"}, "due_time": {"14:00"}}
	if rec := do(t, env.H, "POST", "/items", form, tz("Asia/Tokyo")); rec.Code != http.StatusOK {
		t.Fatalf("add: %d", rec.Code)
	}
	items, _ := env.Svc.List(context.Background(), env.User.ID, false)
	want := time.Date(2026, 10, 3, 5, 0, 0, 0, time.UTC)
	if len(items) != 1 || items[0].DueAt == nil || !items[0].DueAt.Equal(want) {
		t.Fatalf("stored %+v, want due %v", items, want)
	}
	assertContains(t, do(t, env.H, "GET", "/", nil, tz("Europe/Warsaw")).Body.String(), "Due 3 Oct, 07:00")
	edit := do(t, env.H, "GET", "/items/"+strconv.FormatInt(items[0].ID, 10)+"/edit", nil, tz("Asia/Tokyo")).Body.String()
	assertContains(t, edit, `value="14:00"`)
}

func TestBodyNamesTheZoneUsed(t *testing.T) {
	env := newTestEnv(t)
	env.Clock.t = time.Date(2026, 10, 1, 12, 0, 0, 0, mustZone(t, "Europe/Warsaw"))
	if z := bodyZone(t, do(t, env.H, "GET", "/", nil, tz("Asia/Tokyo")).Body.String()); z != "Asia/Tokyo" {
		t.Fatalf("data-tz = %q, want Asia/Tokyo", z)
	}
	if z := bodyZone(t, do(t, env.H, "GET", "/", nil, nil).Body.String()); z != "Europe/Warsaw" {
		t.Fatalf("no cookie: data-tz = %q, want the server zone", z)
	}
}

func TestBadZoneCookieFallsBack(t *testing.T) {
	env := newTestEnv(t)
	env.Clock.t = time.Date(2026, 10, 1, 12, 0, 0, 0, mustZone(t, "Europe/Warsaw"))
	for _, bad := range []string{"../../etc/passwd", "Local", "/etc/localtime", strings.Repeat("A", 1024), "Mars/Base", "Europe/Warsaw\\x", ""} {
		rec := do(t, env.H, "GET", "/", nil, tz(bad))
		if rec.Code != http.StatusOK {
			t.Errorf("%q: status %d", bad, rec.Code)
			continue
		}
		if z := bodyZone(t, rec.Body.String()); z != "Europe/Warsaw" {
			t.Errorf("%q: data-tz = %q, want the server zone", bad, z)
		}
	}
}

func TestParallelRequestsKeepTheirZones(t *testing.T) {
	env := newTestEnv(t)
	env.Clock.t = time.Date(2026, 10, 1, 12, 0, 0, 0, mustZone(t, "Europe/Warsaw"))
	zones := []string{"Asia/Tokyo", "America/New_York"}
	got := make([]string, 20)
	var wg sync.WaitGroup
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Cookie", "todo_tz="+zones[i%2])
			rec := httptest.NewRecorder()
			env.H.ServeHTTP(rec, req)
			if m := dataTZ.FindStringSubmatch(rec.Body.String()); m != nil {
				got[i] = m[1]
			}
		}()
	}
	wg.Wait()
	for i, z := range got {
		if z != zones[i%2] {
			t.Errorf("request %d: data-tz = %q, want %q", i, z, zones[i%2])
		}
	}
}

func TestNotificationTextUsesBrowserZone(t *testing.T) {
	env := newTestEnv(t)
	env.Clock.t = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	env.Svc.AddIn(context.Background(), env.User.ID, "Call", "2026-10-02", "11:59", time.UTC)
	rec := do(t, env.H, "POST", "/notifications/claim", nil, tz("Asia/Tokyo"))
	var got []struct{ Due string }
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 1 {
		t.Fatalf("claim = %s, %v", rec.Body.String(), err)
	}
	if got[0].Due != "Due today, 20:59" {
		t.Fatalf("due text = %q, want Due today, 20:59 (Tokyo)", got[0].Due)
	}
}
