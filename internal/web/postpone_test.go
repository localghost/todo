package web_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Only overdue open items show the four postpone chips.
func TestPostponeChipsOnlyOnOverdueItems(t *testing.T) {
	h, svc := newTestApp(t)
	ctx := context.Background()
	svc.Add(ctx, 1, "Overdue", "2020-01-01", "08:00")
	svc.Add(ctx, 1, "Future", "2099-01-01", "08:00")
	done, _ := svc.Add(ctx, 1, "Done overdue", "2020-01-01", "08:00")
	svc.Toggle(ctx, 1, done.ID)
	svc.Add(ctx, 1, "No due", "", "")

	body := do(t, h, "GET", "/", nil, nil).Body.String()
	for _, m := range []string{"5", "10", "15", "30"} {
		assertContains(t, body, `hx-post="/items/1/postpone?minutes=`+m+`"`, ">+"+m+" min</button>")
	}
	if n := strings.Count(body, `class="postpone-btn"`); n != 4 {
		t.Fatalf("found %d postpone buttons, want 4 (only the overdue item)", n)
	}
}

func TestPostponeRoute(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2020-01-01", "08:00")
	rec := do(t, h, "POST", "/items/1/postpone?minutes=10", nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	assertContains(t, body, `<li id="item-1" class="item">`, `<span class="due">Due `,
		`id="toolbar" class="toolbar" hx-swap-oob="true"`)
	assertNotContains(t, body, `class="due overdue"`, `class="postpone-btn"`)
}

func TestPostponeRouteErrors(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2020-01-01", "08:00")
	svc.Add(context.Background(), 1, "No due", "", "")
	cases := []struct {
		path    string
		headers map[string]string
		status  int
	}{
		{"/items/1/postpone?minutes=7", htmxHeaders, http.StatusBadRequest},
		{"/items/1/postpone?minutes=abc", htmxHeaders, http.StatusBadRequest},
		{"/items/99/postpone?minutes=5", htmxHeaders, http.StatusNotFound},
		{"/items/2/postpone?minutes=5", htmxHeaders, http.StatusUnprocessableEntity},
		{"/items/1/postpone?minutes=5", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
	}
	for _, c := range cases {
		if rec := do(t, h, "POST", c.path, nil, c.headers); rec.Code != c.status {
			t.Errorf("POST %s: status = %d, want %d", c.path, rec.Code, c.status)
		}
	}
}

// The chips disappear after a postpone, so focus moves to the row's checkbox.
func TestAppJSFocusAfterPostpone(t *testing.T) {
	h, _ := newTestApp(t)
	js := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, `.postpone-btn`, `"check-"`)
}
