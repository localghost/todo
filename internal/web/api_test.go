package web_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"todo/internal/auth"
)

// apiDo sends a JSON request to the API with the bearer token (none if "").
// The tester's cookie is never sent.
func apiDo(t *testing.T, env *testEnv, method, target, token, body string, extra ...map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, r)
	req.Header.Set(anonHeader, "1")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, e := range extra {
		for k, v := range e {
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	env.H.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("JSON: %v\n%s", err, rec.Body.String())
	}
	return v
}

type jsonLabel struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Items *int   `json:"items"`
}

type jsonItem struct {
	ID        int64       `json:"id"`
	Text      string      `json:"text"`
	Done      bool        `json:"done"`
	DueAt     *time.Time  `json:"due_at"`
	DueAllDay bool        `json:"due_all_day"`
	Overdue   bool        `json:"overdue"`
	Labels    []jsonLabel `json:"labels"`
}

type jsonList struct {
	Items    []jsonItem `json:"items"`
	Open     int        `json:"open"`
	Done     int        `json:"done"`
	HideDone bool       `json:"hide_done"`
	Label    *jsonLabel `json:"label"`
}

type jsonError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d\n%s", rec.Code, code, rec.Body.String())
	}
}

func wantError(t *testing.T, rec *httptest.ResponseRecorder, code int, errCode string) jsonError {
	t.Helper()
	wantStatus(t, rec, code)
	e := decode[jsonError](t, rec)
	if e.Error != errCode || e.Message == "" {
		t.Fatalf("error = %+v, want code %q and a message", e, errCode)
	}
	return e
}

func TestAPILogin(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	rec := apiDo(t, env, "POST", "/api/v1/login", "", `{"username":"alice","password":"`+pw+`"}`)
	wantStatus(t, rec, http.StatusOK)
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("the API log-in set a cookie: %v", rec.Result().Cookies())
	}
	got := decode[struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
		User      struct {
			Username string `json:"username"`
		} `json:"user"`
	}](t, rec)
	if got.Token == "" || got.User.Username != "alice" || time.Until(got.ExpiresAt) < 29*24*time.Hour {
		t.Fatalf("login = %+v", got)
	}
	me := apiDo(t, env, "GET", "/api/v1/me", got.Token, "")
	wantStatus(t, me, http.StatusOK)
	assertContains(t, me.Body.String(), `"username":"alice"`)

	wantStatus(t, apiDo(t, env, "POST", "/api/v1/logout", got.Token, ""), http.StatusNoContent)
	wantError(t, apiDo(t, env, "GET", "/api/v1/me", got.Token, ""), http.StatusUnauthorized, "logged_out")
}

func TestAPILoginWrongPasswordAndLimit(t *testing.T) {
	env := newTestEnv(t)
	env.Auth.SignUp(context.Background(), "alice", pw)
	// 5 wrong passwords block the username for that IP only (code review 6).
	phone := fromIP("203.0.113.1")
	for i := 0; i < 5; i++ {
		rec := apiDo(t, env, "POST", "/api/v1/login", "", `{"username":"alice","password":"wrong password"}`, phone)
		wantError(t, rec, http.StatusUnauthorized, "bad_login")
	}
	rec := apiDo(t, env, "POST", "/api/v1/login", "", `{"username":"alice","password":"`+pw+`"}`, phone)
	wantError(t, rec, http.StatusTooManyRequests, "locked")
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After header")
	}
	// The web log-in shares the limit.
	if code := login(t, env, "alice", pw, phone); code != http.StatusTooManyRequests {
		t.Fatalf("web log-in after 5 wrong API tries: %d, want 429", code)
	}
	// Another IP still logs in.
	wantStatus(t, apiDo(t, env, "POST", "/api/v1/login", "", `{"username":"alice","password":"`+pw+`"}`, fromIP("198.51.100.7")), http.StatusOK)
}

func TestAPIBadJSON(t *testing.T) {
	env := newTestEnv(t)
	wantError(t, apiDo(t, env, "POST", "/api/v1/login", "", `{"username":`), http.StatusBadRequest, "bad_request")
	wantError(t, apiDo(t, env, "POST", "/api/v1/items", env.Token, `[1,2]`), http.StatusBadRequest, "bad_request")
}

func TestAPINeedsBearerNotCookie(t *testing.T) {
	env := newTestEnv(t)
	wantError(t, apiDo(t, env, "GET", "/api/v1/items", "", ""), http.StatusUnauthorized, "logged_out")
	// The tester's valid session cookie does not open the API ...
	rec := apiDo(t, env, "GET", "/api/v1/items", "", "", cookie(env.Token))
	wantError(t, rec, http.StatusUnauthorized, "logged_out")
	wantError(t, apiDo(t, env, "GET", "/api/v1/items", "not-a-token", ""), http.StatusUnauthorized, "logged_out")
	// ... and a bearer token does not open the web pages.
	if rec := apiDo(t, env, "GET", "/", env.Token, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("web page with only a bearer token: %d, want 303 to /login", rec.Code)
	}
}

func TestAPIUnknownRouteIsJSON(t *testing.T) {
	env := newTestEnv(t)
	wantError(t, apiDo(t, env, "GET", "/api/v1/nothing", env.Token, ""), http.StatusNotFound, "not_found")
}

func TestAPIRenewsAppToken(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	token, _, err := env.Auth.StartSession(ctx, env.User.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	soon := time.Now().Add(10 * 24 * time.Hour)
	if err := env.Store.ExtendSession(ctx, auth.HashToken(token), soon); err != nil {
		t.Fatal(err)
	}
	// The web does not renew: its cookie has a fixed lifetime.
	if rec := do(t, env.H, "GET", "/", nil, cookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("web page: %d", rec.Code)
	}
	if _, sess, _ := env.Auth.Authenticate(ctx, token); sess.ExpiresAt.After(soon.Add(time.Minute)) {
		t.Fatalf("the web renewed the session to %v", sess.ExpiresAt)
	}
	wantStatus(t, apiDo(t, env, "GET", "/api/v1/me", token, ""), http.StatusOK)
	_, sess, err := env.Auth.Authenticate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if left := time.Until(sess.ExpiresAt); left < 29*24*time.Hour {
		t.Fatalf("after an API call the token has %v left, want about 30 days", left)
	}
}

func TestAPIItemsListOrderAndCounts(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	first := mustAdd(t, env.Svc, "First")
	late, _ := env.Svc.Add(ctx, 1, "Late", "2020-01-01", "08:00")
	done := mustAdd(t, env.Svc, "Finished")
	env.Svc.Toggle(ctx, 1, done.ID)

	rec := apiDo(t, env, "GET", "/api/v1/items", env.Token, "")
	wantStatus(t, rec, http.StatusOK)
	l := decode[jsonList](t, rec)
	if l.Open != 2 || l.Done != 1 || l.HideDone || l.Label != nil || len(l.Items) != 3 {
		t.Fatalf("list = %+v", l)
	}
	// The hand-made order: the overdue item stays at its place.
	if l.Items[0].ID != first.ID || l.Items[1].ID != late.ID || !l.Items[1].Overdue || !l.Items[2].Done {
		t.Fatalf("order = %+v", l.Items)
	}
	if l.Items[1].Labels == nil {
		t.Error("labels is null, want []")
	}

	wantStatus(t, apiDo(t, env, "PUT", "/api/v1/settings", env.Token, `{"hide_done":true}`), http.StatusOK)
	l = decode[jsonList](t, apiDo(t, env, "GET", "/api/v1/items", env.Token, ""))
	if !l.HideDone || len(l.Items) != 2 || l.Done != 1 {
		t.Fatalf("with hide done: %+v", l)
	}
}

func TestAPIAddEditToggleDelete(t *testing.T) {
	env := newTestEnv(t)
	rec := apiDo(t, env, "POST", "/api/v1/items", env.Token, `{"text":"  Buy   milk ","labels":["home"]}`)
	wantStatus(t, rec, http.StatusCreated)
	it := decode[jsonItem](t, rec)
	if it.Text != "Buy milk" || len(it.Labels) != 1 || it.Labels[0].Name != "home" {
		t.Fatalf("added = %+v", it)
	}
	id := strconv.FormatInt(it.ID, 10)

	wantError(t, apiDo(t, env, "POST", "/api/v1/items", env.Token, `{"text":"  "}`), http.StatusUnprocessableEntity, "bad_text")
	long := strings.Repeat("x", 501)
	e := wantError(t, apiDo(t, env, "PATCH", "/api/v1/items/"+id, env.Token, `{"text":"`+long+`"}`), http.StatusUnprocessableEntity, "bad_text")
	if !strings.Contains(e.Message, "500") {
		t.Errorf("message = %q", e.Message)
	}

	it = decode[jsonItem](t, apiDo(t, env, "PATCH", "/api/v1/items/"+id, env.Token, `{"text":"Buy oat milk","done":true}`))
	if it.Text != "Buy oat milk" || !it.Done || len(it.Labels) != 1 {
		t.Fatalf("patched = %+v", it)
	}
	// Setting the same state again does not toggle it back.
	it = decode[jsonItem](t, apiDo(t, env, "PATCH", "/api/v1/items/"+id, env.Token, `{"done":true}`))
	if !it.Done {
		t.Fatal("done:true twice made the item open")
	}
	wantStatus(t, apiDo(t, env, "GET", "/api/v1/items/"+id, env.Token, ""), http.StatusOK)

	wantStatus(t, apiDo(t, env, "DELETE", "/api/v1/items/"+id, env.Token, ""), http.StatusNoContent)
	wantError(t, apiDo(t, env, "DELETE", "/api/v1/items/"+id, env.Token, ""), http.StatusNotFound, "not_found")
	wantError(t, apiDo(t, env, "PATCH", "/api/v1/items/"+id, env.Token, `{"done":false}`), http.StatusNotFound, "not_found")
	wantError(t, apiDo(t, env, "GET", "/api/v1/items/abc", env.Token, ""), http.StatusNotFound, "not_found")
}

func TestAPIOtherUsersItemIsNotFound(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	other, _ := env.Auth.SignUp(ctx, "alice", pw)
	theirs, _ := env.Svc.Add(ctx, other.ID, "Secret", "", "")
	id := strconv.FormatInt(theirs.ID, 10)
	wantError(t, apiDo(t, env, "GET", "/api/v1/items/"+id, env.Token, ""), http.StatusNotFound, "not_found")
	wantError(t, apiDo(t, env, "PATCH", "/api/v1/items/"+id, env.Token, `{"text":"Mine"}`), http.StatusNotFound, "not_found")
	wantError(t, apiDo(t, env, "DELETE", "/api/v1/items/"+id, env.Token, ""), http.StatusNotFound, "not_found")
}

func TestAPIDueUsesPhoneZone(t *testing.T) {
	env := newTestEnv(t)
	it := mustAdd(t, env.Svc, "Call mum")
	id := strconv.FormatInt(it.ID, 10)
	warsaw := map[string]string{"Time-Zone": "Europe/Warsaw"}
	rec := apiDo(t, env, "PUT", "/api/v1/items/"+id+"/due", env.Token, `{"date":"2030-01-15","time":"1430"}`, warsaw)
	wantStatus(t, rec, http.StatusOK)
	got := decode[jsonItem](t, rec)
	want := time.Date(2030, 1, 15, 13, 30, 0, 0, time.UTC) // 14:30 in Warsaw (UTC+1)
	if got.DueAt == nil || !got.DueAt.Equal(want) || got.DueAllDay {
		t.Fatalf("due = %v (all day %v), want %v", got.DueAt, got.DueAllDay, want)
	}
	if !strings.Contains(rec.Body.String(), `"due_at":"2030-01-15T13:30:00Z"`) {
		t.Errorf("due_at is not UTC: %s", rec.Body.String())
	}

	got = decode[jsonItem](t, apiDo(t, env, "PUT", "/api/v1/items/"+id+"/due", env.Token, `{"date":"2030-01-16","time":""}`, warsaw))
	if !got.DueAllDay {
		t.Fatalf("no time: %+v, want all day", got)
	}
	wantError(t, apiDo(t, env, "PUT", "/api/v1/items/"+id+"/due", env.Token, `{"date":"2030-02-30","time":""}`), http.StatusUnprocessableEntity, "bad_due")
	got = decode[jsonItem](t, apiDo(t, env, "PUT", "/api/v1/items/"+id+"/due", env.Token, `{"date":"","time":""}`))
	if got.DueAt != nil {
		t.Fatalf("cleared due = %v", got.DueAt)
	}
}

func TestAPIPostponeAndMove(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	a := mustAdd(t, env.Svc, "A")
	b := mustAdd(t, env.Svc, "B")
	late, _ := env.Svc.Add(ctx, 1, "Late", "2020-01-01", "08:00")
	ids := func(id int64) string { return strconv.FormatInt(id, 10) }

	wantError(t, apiDo(t, env, "POST", "/api/v1/items/"+ids(a.ID)+"/postpone", env.Token, `{"minutes":10}`), http.StatusConflict, "cannot_postpone")
	wantError(t, apiDo(t, env, "POST", "/api/v1/items/"+ids(late.ID)+"/postpone", env.Token, `{"minutes":7}`), http.StatusBadRequest, "bad_postpone")
	got := decode[jsonItem](t, apiDo(t, env, "POST", "/api/v1/items/"+ids(late.ID)+"/postpone", env.Token, `{"minutes":10}`))
	if got.DueAt == nil || got.DueAt.Before(time.Now()) || got.Overdue {
		t.Fatalf("postponed = %+v", got)
	}

	wantStatus(t, apiDo(t, env, "POST", "/api/v1/items/"+ids(a.ID)+"/move", env.Token, `{"after":`+ids(b.ID)+`}`), http.StatusNoContent)
	l := decode[jsonList](t, apiDo(t, env, "GET", "/api/v1/items", env.Token, ""))
	if l.Items[0].ID != b.ID || l.Items[1].ID != a.ID {
		t.Fatalf("after move: %+v", l.Items)
	}
	late2, _ := env.Svc.Add(ctx, 1, "Late 2", "2020-01-01", "09:00")
	// An overdue item moves like any other item.
	wantStatus(t, apiDo(t, env, "POST", "/api/v1/items/"+ids(late2.ID)+"/move", env.Token, `{"after":0}`), http.StatusNoContent)
	if l := decode[jsonList](t, apiDo(t, env, "GET", "/api/v1/items", env.Token, "")); l.Items[0].ID != late2.ID {
		t.Fatalf("after the move of an overdue item: %+v", l.Items)
	}
	wantError(t, apiDo(t, env, "POST", "/api/v1/items/"+ids(a.ID)+"/move", env.Token, `{"after":-1}`), http.StatusNotFound, "not_found")
}

func TestAPILabels(t *testing.T) {
	env := newTestEnv(t)
	it := mustAdd(t, env.Svc, "Report")
	other := mustAdd(t, env.Svc, "Groceries")
	id := strconv.FormatInt(it.ID, 10)

	got := decode[jsonItem](t, apiDo(t, env, "POST", "/api/v1/items/"+id+"/labels", env.Token, `{"name":"work","color":"#112233"}`))
	if len(got.Labels) != 1 || got.Labels[0].Name != "work" || got.Labels[0].Color != "#112233" {
		t.Fatalf("labels = %+v", got.Labels)
	}
	work := got.Labels[0].ID
	wantError(t, apiDo(t, env, "POST", "/api/v1/items/"+id+"/labels", env.Token, `{"name":"  "}`), http.StatusUnprocessableEntity, "bad_label")

	labels := decode[[]jsonLabel](t, apiDo(t, env, "GET", "/api/v1/labels", env.Token, ""))
	if len(labels) != 1 || labels[0].Items == nil || *labels[0].Items != 1 {
		t.Fatalf("labels = %+v", labels)
	}

	l := decode[jsonList](t, apiDo(t, env, "GET", "/api/v1/items?label="+strconv.FormatInt(work, 10), env.Token, ""))
	if len(l.Items) != 1 || l.Items[0].ID != it.ID || l.Label == nil || l.Label.Name != "work" || l.Open != 1 {
		t.Fatalf("filtered = %+v (other item %d)", l, other.ID)
	}

	lid := strconv.FormatInt(work, 10)
	wantStatus(t, apiDo(t, env, "PUT", "/api/v1/labels/"+lid+"/color", env.Token, `{"color":"#AABBCC"}`), http.StatusNoContent)
	wantError(t, apiDo(t, env, "PUT", "/api/v1/labels/"+lid+"/color", env.Token, `{"color":"red"}`), http.StatusUnprocessableEntity, "bad_label")
	got = decode[jsonItem](t, apiDo(t, env, "GET", "/api/v1/items/"+id, env.Token, ""))
	if got.Labels[0].Color != "#aabbcc" {
		t.Fatalf("color = %q", got.Labels[0].Color)
	}

	got = decode[jsonItem](t, apiDo(t, env, "DELETE", "/api/v1/items/"+id+"/labels/"+lid, env.Token, ""))
	if len(got.Labels) != 0 {
		t.Fatalf("after remove: %+v", got.Labels)
	}
	wantStatus(t, apiDo(t, env, "DELETE", "/api/v1/labels/"+lid, env.Token, ""), http.StatusNoContent)
	e := wantError(t, apiDo(t, env, "DELETE", "/api/v1/labels/"+lid, env.Token, ""), http.StatusNotFound, "not_found")
	if !strings.Contains(e.Message, "label") {
		t.Errorf("message = %q", e.Message)
	}
}

func TestAPICreateLabel(t *testing.T) {
	env := newTestEnv(t)
	rec := apiDo(t, env, "POST", "/api/v1/labels", env.Token, `{"name":" garden ","color":"#112233"}`)
	wantStatus(t, rec, http.StatusOK)
	got := decode[jsonLabel](t, rec)
	if got.ID == 0 || got.Name != "garden" || got.Color != "#112233" {
		t.Fatalf("label = %+v", got)
	}
	// The same name gives the label that exists.
	again := decode[jsonLabel](t, apiDo(t, env, "POST", "/api/v1/labels", env.Token, `{"name":"GARDEN"}`))
	if again.ID != got.ID || again.Color != "#112233" {
		t.Fatalf("again = %+v, want %+v", again, got)
	}
	labels := decode[[]jsonLabel](t, apiDo(t, env, "GET", "/api/v1/labels", env.Token, ""))
	if len(labels) != 1 || labels[0].Items == nil || *labels[0].Items != 0 {
		t.Fatalf("labels = %+v", labels)
	}
	wantError(t, apiDo(t, env, "POST", "/api/v1/labels", env.Token, `{"name":"  "}`), http.StatusUnprocessableEntity, "bad_label")
	wantStatus(t, apiDo(t, env, "POST", "/api/v1/labels", "", `{"name":"x"}`), http.StatusUnauthorized)
}

func TestAPIClearDone(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	for _, text := range []string{"One", "Two"} {
		it := mustAdd(t, env.Svc, text)
		env.Svc.Toggle(ctx, 1, it.ID)
	}
	mustAdd(t, env.Svc, "Open")
	rec := apiDo(t, env, "POST", "/api/v1/items/done/delete", env.Token, "")
	wantStatus(t, rec, http.StatusOK)
	if got := decode[map[string]int](t, rec); got["deleted"] != 2 {
		t.Fatalf("deleted = %v", got)
	}
}
