package web_test

import (
	"net/http"
	"slices"
	"testing"
	"time"
)

// The Today view of the Android app (issue #15). The items of todayEnv: 1
// overdue, 2 today 18:00, 3 no date, 4 in two days, 5 today all day, 6 done
// today, 7 done yesterday.

var phoneInWarsaw = map[string]string{"Time-Zone": "Europe/Warsaw"}

func wantIDs(t *testing.T, got []jsonItem, want ...int64) {
	t.Helper()
	ids := []int64{}
	for _, it := range got {
		ids = append(ids, it.ID)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, want) {
		t.Fatalf("items = %v, want %v", ids, want)
	}
}

func TestAPIItemsToday(t *testing.T) {
	env := todayEnv(t)
	rec := apiDo(t, env, "GET", "/api/v1/items?due=today", env.Token, "", phoneInWarsaw)
	wantStatus(t, rec, http.StatusOK)
	got := decode[jsonList](t, rec)
	wantIDs(t, got.Items, 1, 2, 5, 6)
	if got.Open != 3 || got.Done != 1 {
		t.Fatalf("counts = %d open, %d done, want 3 and 1", got.Open, got.Done)
	}
}

// "Today" is the day on the phone. At 18:00 on 2 Nov in Warsaw it is already
// 3 Nov in Tokyo, so item 4 (due 3 Nov) is in the view there.
func TestAPIItemsTodayUsesPhoneZone(t *testing.T) {
	env := todayEnv(t)
	env.Clock.t = time.Date(2026, 11, 2, 18, 0, 0, 0, warsawZone(t))
	rec := apiDo(t, env, "GET", "/api/v1/items?due=today", env.Token, "", map[string]string{"Time-Zone": "Asia/Tokyo"})
	wantStatus(t, rec, http.StatusOK)
	wantIDs(t, decode[jsonList](t, rec).Items, 1, 2, 4, 5)

	rec = apiDo(t, env, "GET", "/api/v1/items?due=today", env.Token, "", phoneInWarsaw)
	wantStatus(t, rec, http.StatusOK)
	wantIDs(t, decode[jsonList](t, rec).Items, 1, 2, 5)
}

func TestAPIItemsTodayWithLabel(t *testing.T) {
	env := todayEnv(t)
	rec := apiDo(t, env, "GET", "/api/v1/items?due=today&label=1", env.Token, "", phoneInWarsaw)
	wantStatus(t, rec, http.StatusOK)
	got := decode[jsonList](t, rec)
	wantIDs(t, got.Items, 2)
	if got.Label == nil || got.Label.Name != "work" {
		t.Fatalf("label = %+v, want work", got.Label)
	}
}

// Without ?due=today, an app that does not know the view gets the whole list.
func TestAPIItemsAllStaysWhole(t *testing.T) {
	env := todayEnv(t)
	rec := apiDo(t, env, "GET", "/api/v1/items", env.Token, "", phoneInWarsaw)
	wantStatus(t, rec, http.StatusOK)
	wantIDs(t, decode[jsonList](t, rec).Items, 1, 2, 3, 4, 5, 6, 7)
}

func TestAPIClearDoneToday(t *testing.T) {
	env := todayEnv(t)
	rec := apiDo(t, env, "POST", "/api/v1/items/done/delete?due=today", env.Token, "", phoneInWarsaw)
	wantStatus(t, rec, http.StatusOK)
	if got := decode[map[string]int](t, rec); got["deleted"] != 1 {
		t.Fatalf("deleted = %v, want 1", got)
	}
	rec = apiDo(t, env, "GET", "/api/v1/items", env.Token, "", phoneInWarsaw)
	// 6 (done today) is gone; 7 (done yesterday) stays.
	wantIDs(t, decode[jsonList](t, rec).Items, 1, 2, 3, 4, 5, 7)
}

func TestAPIAddItemWithDueDate(t *testing.T) {
	env := todayEnv(t)
	rec := apiDo(t, env, "POST", "/api/v1/items", env.Token, `{"text":"Buy flowers","labels":["work"],"due_date":"2026-11-01"}`, phoneInWarsaw)
	wantStatus(t, rec, http.StatusCreated)
	got := decode[jsonItem](t, rec)
	if !got.DueAllDay || got.DueAt == nil || got.DueAt.In(warsawZone(t)).Format("2006-01-02") != "2026-11-01" {
		t.Fatalf("due = %v (all day %v), want 1 Nov, all day", got.DueAt, got.DueAllDay)
	}
	if len(got.Labels) != 1 || got.Labels[0].Name != "work" {
		t.Fatalf("labels = %+v", got.Labels)
	}
}

func TestAPIAddItemNoDueDate(t *testing.T) {
	env := todayEnv(t)
	rec := apiDo(t, env, "POST", "/api/v1/items", env.Token, `{"text":"Buy flowers"}`, phoneInWarsaw)
	wantStatus(t, rec, http.StatusCreated)
	if got := decode[jsonItem](t, rec); got.DueAt != nil {
		t.Fatalf("due = %v, want none", got.DueAt)
	}
}

func TestAPIAddItemBadDueDate(t *testing.T) {
	env := todayEnv(t)
	rec := apiDo(t, env, "POST", "/api/v1/items", env.Token, `{"text":"Buy flowers","due_date":"tomorrow"}`, phoneInWarsaw)
	wantError(t, rec, http.StatusUnprocessableEntity, "bad_due")
}
