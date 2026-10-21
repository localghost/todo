package web_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"todo/internal/todo"
)

// Too long text in the add form: 422, a clear message, the typed text stays,
// nothing is stored (canvas page 5, "limits").
func TestAddTextTooLong(t *testing.T) {
	h, svc := newTestApp(t)
	long := strings.Repeat("a", todo.MaxTextChars+1)
	rec := do(t, h, "POST", "/items", url.Values{"text": {long}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<p id="add-error" class="error">The text is too long: at most 500 characters.</p>`,
		`value="`+long+`"`, `aria-invalid="true"`)
	if open, done, _ := svc.Counts(context.Background(), 1); open+done != 0 {
		t.Fatalf("an item was stored")
	}
	rec = do(t, h, "POST", "/items", url.Values{"text": {"  "}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `<p id="add-error" class="error">Please type some text first.</p>`)
}

// Too long text in the edit row: 422, the message, the old text stays.
func TestEditTextTooLong(t *testing.T) {
	h, svc := newTestApp(t)
	mustAdd(t, svc, "Short")
	rec := do(t, h, "PUT", "/items/1", url.Values{"text": {strings.Repeat("b", todo.MaxTextChars+1)}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<p class="hint error" id="edit-error-1">The text is too long: at most 500 characters.</p>`)
	if got, _ := svc.Get(context.Background(), 1, 1); got.Text != "Short" {
		t.Fatalf("text = %q, want Short", got.Text)
	}
	rec = do(t, h, "PUT", "/items/1", url.Values{"text": {" "}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `<p class="hint error" id="edit-error-1">Text cannot be empty.</p>`)
}

// The 5,001st item: 422 and a message that says what to do.
func TestAddTooManyItems(t *testing.T) {
	env := newTestEnv(t)
	db, err := sql.Open("sqlite", "file:"+env.DB)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, _ := db.Begin()
	for i := 1; i <= todo.MaxItemsPerUser; i++ {
		if _, err := tx.Exec(`INSERT INTO items (user_id, text, done, position, created_at, updated_at) VALUES (1, 'x', 0, ?, ?, ?)`,
			i, "2026-10-01T12:00:00Z", "2026-10-01T12:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rec := do(t, env.H, "POST", "/items", url.Values{"text": {"Buy stamps"}}, htmxHeaders)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	assertContains(t, rec.Body.String(), `<p id="add-error" class="error">You have 5,000 items. Delete some first.</p>`, `value="Buy stamps"`)
}

// "Change password" allows 5 wrong current passwords per user in 15 minutes.
// Then it refuses at once, also the right password, with no slow hash
// (code review 2). Other users are not blocked.
func TestChangePasswordLimit(t *testing.T) {
	env := newTestEnv(t)
	bob := signedUp(t, env, "bob")
	carol := signedUp(t, env, "carol")
	wrong := url.Values{"current_password": {"wrong password"}, "new_password": {"new password 1"}}
	right := url.Values{"current_password": {pw}, "new_password": {"new password 1"}}
	for i := 0; i < 5; i++ {
		if rec := do(t, env.H, "POST", "/account/password", wrong, cookie(bob)); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("wrong password %d: status %d, want 422", i+1, rec.Code)
		}
	}
	rec := do(t, env.H, "POST", "/account/password", right, cookie(bob))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("after 5 wrong: status %d, want 429", rec.Code)
	}
	assertContains(t, rec.Body.String(), "Too many wrong passwords. Wait 15 minutes, then try again.")
	if rec := do(t, env.H, "POST", "/account/password", wrong, cookie(carol)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("carol: status %d, want 422", rec.Code)
	}
	env.Clock.t = env.Clock.t.Add(15*time.Minute + time.Second)
	rec = do(t, env.H, "POST", "/account/password", right, cookie(bob))
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("after 15 minutes: still 429")
	}
	assertContains(t, rec.Body.String(), "Password changed.")
}
