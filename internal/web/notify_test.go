package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

type claimed struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
	Due   string `json:"due"`
}

func TestClaimReturnsDueItemsOnce(t *testing.T) {
	h, svc := newTestApp(t)
	svc.Add(context.Background(), 1, "Call the dentist", "2020-01-01", "08:00")
	rec := do(t, h, "POST", "/notifications/claim", nil, htmxHeaders)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status = %d, type = %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got []claimed
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v\n%s", err, rec.Body.String())
	}
	if len(got) != 1 || got[0].Text != "Call the dentist" || got[0].Title != "Todo: overdue" || got[0].Due != "Overdue since 1 Jan 2020, 08:00" {
		t.Fatalf("claimed = %+v", got)
	}
	if again := do(t, h, "POST", "/notifications/claim", nil, htmxHeaders).Body.String(); again != "[]\n" {
		t.Fatalf("second claim body = %q, want []", again)
	}
}

func TestClaimEmptyIsArray(t *testing.T) {
	h, _ := newTestApp(t)
	if body := do(t, h, "POST", "/notifications/claim", nil, htmxHeaders).Body.String(); body != "[]\n" {
		t.Fatalf("body = %q, want []", body)
	}
}

func TestClaimRejectsCrossSite(t *testing.T) {
	h, _ := newTestApp(t)
	if rec := do(t, h, "POST", "/notifications/claim", nil, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestNotifyBarHasDue(t *testing.T) {
	h, svc := newTestApp(t)
	ctx := context.Background()
	assertContains(t, do(t, h, "GET", "/", nil, nil).Body.String(), `<div id="notify-bar" class="notify-bar" data-has-due="false"></div>`)

	done, _ := svc.Add(ctx, 1, "Done with due", "2026-10-03", "")
	svc.Toggle(ctx, 1, done.ID)
	assertContains(t, do(t, h, "GET", "/", nil, nil).Body.String(), `data-has-due="false"`)

	rec := do(t, h, "POST", "/items", url.Values{"text": {"Open with due"}, "due_date": {"2026-10-03"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `<div id="notify-bar" class="notify-bar" data-has-due="true" hx-swap-oob="true"></div>`)
}

func TestAppJSPollsClaims(t *testing.T) {
	h, _ := newTestApp(t)
	body := do(t, h, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, body, `"/notifications/claim"`, "requestPermission", "visibilitychange",
		"item.title", "Get a notification when an item is due.",
		"Notifications are blocked in your browser settings.", "30000")
}
