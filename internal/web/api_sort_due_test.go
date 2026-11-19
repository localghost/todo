package web_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// The due date order of the web, through the API: sort_due in /me and
// /settings, the items in that order, and no move while it is on.
func TestAPISortDue(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	none := mustAdd(t, env.Svc, "No date")
	later, _ := env.Svc.Add(ctx, 1, "Later", "2099-03-02", "")
	soon, _ := env.Svc.Add(ctx, 1, "Soon", "2099-03-01", "09:00")
	late, _ := env.Svc.Add(ctx, 1, "Late", "2020-01-01", "08:00")
	ids := func(id int64) string { return strconv.FormatInt(id, 10) }
	order := func(l jsonList) []int64 {
		var out []int64
		for _, it := range l.Items {
			out = append(out, it.ID)
		}
		return out
	}
	type user struct {
		HideDone bool `json:"hide_done"`
		SortDue  bool `json:"sort_due"`
	}
	type list struct {
		jsonList
		SortDue bool `json:"sort_due"`
	}

	if u := decode[user](t, apiDo(t, env, "GET", "/api/v1/me", env.Token, "")); u.SortDue {
		t.Fatalf("me = %+v, want sort_due false", u)
	}
	l := decode[list](t, apiDo(t, env, "GET", "/api/v1/items", env.Token, ""))
	if l.SortDue || order(l.jsonList)[0] != none.ID {
		t.Fatalf("by hand: sort_due %v, order %v", l.SortDue, order(l.jsonList))
	}

	rec := apiDo(t, env, "PUT", "/api/v1/settings", env.Token, `{"sort_due":true}`)
	wantStatus(t, rec, http.StatusOK)
	if u := decode[user](t, rec); !u.SortDue || u.HideDone {
		t.Fatalf("settings answer = %+v", u)
	}
	if u := decode[user](t, apiDo(t, env, "GET", "/api/v1/me", env.Token, "")); !u.SortDue {
		t.Fatalf("me after the change = %+v", u)
	}
	// The web shows the same setting.
	if u, _, _ := env.Auth.AuthenticateApp(ctx, env.Token); !u.SortDue {
		t.Fatal("the user's sort_due is not saved")
	}

	l = decode[list](t, apiDo(t, env, "GET", "/api/v1/items", env.Token, ""))
	want := []int64{late.ID, soon.ID, later.ID, none.ID}
	got := order(l.jsonList)
	if !l.SortDue || len(got) != len(want) {
		t.Fatalf("due order: sort_due %v, order %v", l.SortDue, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("due order = %v, want %v", got, want)
		}
	}

	wantError(t, apiDo(t, env, "POST", "/api/v1/items/"+ids(none.ID)+"/move", env.Token, `{"after":0}`), http.StatusConflict, "sort_due")

	// Hide done alone leaves the order as it is.
	wantStatus(t, apiDo(t, env, "PUT", "/api/v1/settings", env.Token, `{"hide_done":true}`), http.StatusOK)
	if u := decode[user](t, apiDo(t, env, "GET", "/api/v1/me", env.Token, "")); !u.SortDue || !u.HideDone {
		t.Fatalf("me after hide done = %+v", u)
	}

	wantStatus(t, apiDo(t, env, "PUT", "/api/v1/settings", env.Token, `{"sort_due":false}`), http.StatusOK)
	wantStatus(t, apiDo(t, env, "POST", "/api/v1/items/"+ids(none.ID)+"/move", env.Token, `{"after":`+ids(late.ID)+`}`), http.StatusNoContent)
	l = decode[list](t, apiDo(t, env, "GET", "/api/v1/items", env.Token, ""))
	if l.SortDue || order(l.jsonList)[0] == none.ID {
		t.Fatalf("back by hand: sort_due %v, order %v", l.SortDue, order(l.jsonList))
	}
}
