# Label list: several labels, colors and delete — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the Labels page. The list under + shows all labels, adds or removes several
labels while it stays open, changes a label's color with a color wheel, picks the color of a new
label, and deletes a label after a question.

**Architecture:** The server keeps answering label changes with the row (or the list section),
as today. The page moves the open label list into the new row and loads it again, so the list
stays open. Color changes go to a new `PUT /labels/{id}/color` that answers 204; the page
recolors the chips itself. The wheel code moves from `labels.js` to `wheel.js`.

**Tech Stack:** Go 1.27 (`net/http`, `html/template`), SQLite, htmx 2, plain JavaScript.

**Spec:** `docs/superpowers/specs/2026-10-20-label-list-manage-design.md`. Design: canvas "Todo app
UI proposal", page "16 · Labels", board "NEW: label list – several labels, color wheel, × deletes
(no Labels page)".

Work in the worktree `/home/kostrzewa/claude-personal/todo-worktrees/labels-in-popup` (branch
`labels-in-popup`). Run all commands from there.

## Global Constraints

- No rename of labels anywhere.
- Limits stay: name at most 30 characters, no comma; at most 5 labels per item, 50 per user.
- New labels get `todo.LabelColors[count % 8]` unless the user picked a color.
- The 50-labels message: "You have 50 labels. Delete one first (× on its line)."
- The 5-labels line in the list: "An item can have at most 5 labels."
- Delete question: "Delete “NAME”? It comes off N item(s). The items stay." or
  "Delete “NAME”? No items use it."
- The page's CSP blocks `style` attributes in HTML. Set styles only from JavaScript.
- Each commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

- A name click while the item list is filtered by that label: the row leaves the list, and the
  pop-up must close without errors.
- A failed request (network error) while the list waits for its row: the list must not stay
  "busy" and must still close on Esc or a click outside.
- Pop-up closed while the wheel previews a color (Esc twice, click outside): the chips must get
  their old color back.
- A 422 answer (bad name, 50 labels) after Create: the list shows the message and keeps the
  chosen color of the new label.
- Another user's label id in `PUT /labels/{id}/color` and `DELETE /labels/{id}`: 404, nothing
  changes.

---

### Task 1: Service and store — color of a new label, color only update

**Files:**
- Modify: `internal/todo/todo.go` (Store interface, the `UpdateLabel` line)
- Modify: `internal/todo/label.go` (`UpdateLabel` → `SetLabelColor`, `AddLabel`)
- Modify: `internal/store/sqlite/labels.go` (`UpdateLabel` → `SetLabelColor`)
- Test: `internal/todo/label_test.go`, `internal/store/sqlite/labels_test.go`
- Modify callers: `internal/web/labels_handlers.go:150`, `internal/web/due_popup_test.go:104`

**Interfaces:**
- Produces: `(*todo.Service).SetLabelColor(ctx, userID, id int64, color string) error` —
  `*LabelError{"Pick a color."}` (is `ErrBadLabel`) for a bad color, `ErrNotFound` for an
  unknown or foreign label.
- Produces: `(*todo.Service).AddLabel(ctx, userID, itemID int64, name, color string) (Item, error)`
- Produces: `todo.Store.SetLabelColor(ctx, userID, id int64, color string) error`

- [ ] **Step 1: Write the failing tests.** In `internal/todo/label_test.go` replace
  `TestUpdateLabelChecksNameAndColor` with:

```go
func TestSetLabelColor(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "work"}, nil)
	id := it.Labels[0].ID
	for _, color := range []string{"red", "#d1382", "#d1382e0", "#g1382e", "d1382e9", ""} {
		if err := svc.SetLabelColor(ctx, 1, id, color); !errors.Is(err, todo.ErrBadLabel) {
			t.Errorf("SetLabelColor(%q) err = %v, want ErrBadLabel", color, err)
		}
	}
	if err := svc.SetLabelColor(ctx, 1, id, "#2ED1C8"); err != nil {
		t.Fatal(err)
	}
	if l, _ := svc.Label(ctx, 1, id); l.Name != "work" || l.Color != "#2ed1c8" {
		t.Fatalf("label = %+v", l)
	}
	if err := svc.SetLabelColor(ctx, 1, 999, "#2ed1c8"); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("unknown label: %v, want ErrNotFound", err)
	}
}

func TestAddLabelWithColor(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	it, _ := svc.AddItem(ctx, 1, todo.Input{Text: "A", Labels: "work"}, nil)
	old := it.Labels[0].Color
	b, _ := svc.AddItem(ctx, 1, todo.Input{Text: "B"}, nil)

	// A new label gets the chosen color.
	b, err := svc.AddLabel(ctx, 1, b.ID, "gym", "#12AB34")
	if err != nil || len(b.Labels) != 1 || b.Labels[0].Color != "#12ab34" {
		t.Fatalf("new label = %+v, %v", b.Labels, err)
	}
	// A label that exists keeps its color.
	b, err = svc.AddLabel(ctx, 1, b.ID, "WORK", "#000000")
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := svc.Label(ctx, 1, it.Labels[0].ID); l.Color != old {
		t.Fatalf("work color = %q, want %q", l.Color, old)
	}
	// No color or a bad one: the next fixed color.
	b, _ = svc.AddLabel(ctx, 1, b.ID, "home", "red")
	for _, l := range b.Labels {
		if l.Name == "home" && l.Color != todo.LabelColors[2] {
			t.Fatalf("home color = %q, want %q", l.Color, todo.LabelColors[2])
		}
	}
}
```

  Change the four other `svc.AddLabel(ctx, 1, it.ID, X)` calls in that file to
  `svc.AddLabel(ctx, 1, it.ID, X, "")`.

  In `internal/store/sqlite/labels_test.go`: in the test with Bob's label, replace the
  `s.UpdateLabel(ctx, 1, bobLabel, "mine", "red")` check with:

```go
	if err := s.SetLabelColor(ctx, 1, bobLabel, "#d1382e"); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("SetLabelColor(bob's) for alice: %v, want ErrNotFound", err)
	}
```

  Replace `TestUpdateLabelRenameClash` with:

```go
func TestSetLabelColorKeepsName(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	it := mustLabeled(t, s, "A", "work")
	if err := s.SetLabelColor(ctx, 1, it.Labels[0].ID, "#6b44bb"); err != nil {
		t.Fatal(err)
	}
	l, err := s.Label(ctx, 1, it.Labels[0].ID)
	if err != nil || l.Name != "work" || l.Color != "#6b44bb" || l.Items != 1 {
		t.Fatalf("label = %+v, %v", l, err)
	}
}
```

- [ ] **Step 2: Run the tests and see them fail to build.**
  Run: `go test ./internal/todo/ ./internal/store/sqlite/`
  Expected: FAIL, `svc.SetLabelColor undefined`, too many arguments in `AddLabel`.

- [ ] **Step 3: Implement.** In `internal/todo/todo.go` replace the two `UpdateLabel` lines of
  the `Store` interface with:

```go
	// SetLabelColor returns ErrNotFound if the user has no label id.
	SetLabelColor(ctx context.Context, userID, id int64, color string) error
```

  In `internal/store/sqlite/labels.go` replace `UpdateLabel` with:

```go
func (s *Store) SetLabelColor(ctx context.Context, userID, id int64, color string) error {
	return s.update(ctx, `UPDATE labels SET color = ? WHERE id = ? AND user_id = ?`, color, id, userID)
}
```

  Remove the `strings` import there if nothing else uses it.

  In `internal/todo/label.go` replace `UpdateLabel` with:

```go
// SetLabelColor gives a label a new color, #rrggbb in any case.
func (s *Service) SetLabelColor(ctx context.Context, userID, id int64, color string) error {
	color, ok := CleanColor(color)
	if !ok {
		return &LabelError{Msg: "Pick a color."}
	}
	return s.store.SetLabelColor(ctx, userID, id, color)
}
```

  Change `AddLabel`:

```go
// AddLabel puts one label on an item. A name the user has no label for
// makes a new label, with color if it is #rrggbb, else with the next fixed
// color. A label the item already has (ignoring case) changes nothing.
func (s *Service) AddLabel(ctx context.Context, userID, itemID int64, name, color string) (Item, error) {
	name, err := cleanLabelName(name)
	if err != nil {
		return Item{}, err
	}
	it, err := s.store.Get(ctx, userID, itemID)
	if err != nil {
		return Item{}, err
	}
	names := make([]string, 0, len(it.Labels)+1)
	for _, l := range it.Labels {
		if strings.EqualFold(l.Name, name) {
			return it, nil
		}
		names = append(names, l.Name)
	}
	if len(names) >= MaxLabelsPerItem {
		return Item{}, &LabelError{Msg: "An item can have at most 5 labels."}
	}
	labels, err := s.store.Labels(ctx, userID)
	if err != nil {
		return Item{}, err
	}
	isNew := !slices.ContainsFunc(labels, func(l Label) bool { return strings.EqualFold(l.Name, name) })
	it, err = s.setLabels(ctx, userID, it, append(names, name))
	color, ok := CleanColor(color)
	if err != nil || !isNew || !ok {
		return it, err
	}
	for _, l := range it.Labels {
		if strings.EqualFold(l.Name, name) {
			if err := s.store.SetLabelColor(ctx, userID, l.ID, color); err != nil {
				return Item{}, err
			}
			return s.store.Get(ctx, userID, it.ID)
		}
	}
	return it, nil
}
```

  Callers: `internal/web/labels_handlers.go:150` becomes
  `s.svc.AddLabel(r.Context(), userID(r), id, name, r.FormValue("color"))`;
  `internal/web/due_popup_test.go:104` becomes `svc.AddLabel(ctx, 1, 1, "work", "")`.
  `saveLabel` in `labels_handlers.go` still calls `UpdateLabel`: delete `saveLabel` now (Task 3
  removes its route; until then change the route line in `server.go` to nothing — remove
  `mux.Handle("POST /labels/{id}", s.protect(s.saveLabel))`). Remove the parts of
  `TestLabelsPage` and `TestLabelsOfAnotherUserCannotBeChanged` that POST to `/labels/{id}`.

- [ ] **Step 4: Run the tests.**
  Run: `go vet ./... && go test ./...`
  Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add -A internal
git commit -m "feat: a new label can get a chosen color; labels change only their color

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 2: Server — the new label list, color and delete routes

**Files:**
- Modify: `internal/web/views.go` (`pickerView`, `labelsMessage`)
- Modify: `internal/web/labels_handlers.go` (`pickerView` builder, `addItemLabel`, new
  `setLabelColor`, new `deleteLabel`)
- Modify: `internal/web/server.go` (routes)
- Modify: `internal/web/templates/item.html` (`label-picker`, chips, new icons)
- Modify: `internal/web/templates/list.html:13`, `internal/web/templates/form.html` (chips get
  `data-label`)
- Test: `internal/web/labels_test.go`

**Interfaces:**
- Consumes: `SetLabelColor`, `AddLabel(…, name, color)` from Task 1.
- Produces (markup that Task 5 relies on):
  - `.picker.label-picker#picker-{item}` with `data-item`, `data-full` ("true"/"false"),
    `data-new-color`.
  - `.picker-main` with `.picker-search`, `ul.picker-list`, `.picker-empty`.
  - Each label: `li.picker-opt[data-name][data-label][data-items]` with
    `button.sw[data-color][data-label]`, `button.picker-name[aria-pressed]` (and
    `aria-disabled="true"` when full and not pressed), `span.picker-check` when pressed,
    `button.picker-x`.
  - `li.picker-opt.picker-create[hidden]` with `button.sw[data-color]` and
    `button.picker-name > span.picker-create-text`.
  - `.picker-color[hidden]` with `.picker-color-title`, `.wheel > .wheel-ring`,
    `.color-preview .chip`, `[data-color-done]`, `[data-color-cancel]` (twice: ← and Cancel).
  - Every chip of a label: `data-label="{id}"` after `data-color`.
- Produces routes: `PUT /labels/{id}/color` (204/404/422), `DELETE /labels/{id}` (list section,
  or `HX-Redirect: /` with 204 when the page is filtered by that label).

- [ ] **Step 1: Write the failing tests.** In `internal/web/labels_test.go`:

  Replace the middle of `TestPlusAddsOneLabel` (the list checks) so that the list shows all
  labels with `aria-pressed`:

```go
	// The list offers all the user's labels; the item's labels are pressed.
	body = do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, body, `data-name="home"`, `data-name="work"`, `class="picker-search"`,
		`aria-pressed="false">home</button>`, `class="picker-x" aria-label="Delete label “home”"`,
		`class="wheel" role="slider"`, `data-full="false"`,
		fmt.Sprintf(`data-new-color="%s"`, todo.LabelColors[2]))
	assertNotContains(t, body, `role="listbox"`, `data-taken`)
```

  and after the POST of "work":

```go
	assertContains(t, rec.Body.String(), fmt.Sprintf(`<span class="chip" data-color="%s" data-label="%d"><a href="/?label=%d"`, got.Labels[0].Color, work, work),
		fmt.Sprintf(`hx-delete="/items/%d/labels/%d"`, a.ID, work), `id="toolbar"`)

	body = do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, body, `aria-pressed="true">work</button><span class="picker-check"`, `aria-pressed="false">home</button>`)
```

  (remove the old `data-taken` asserts). In `TestPlusShowsLabelErrorsInTheList` change the last
  assert to:

```go
	full := do(t, env.H, "GET", path, nil, htmxHeaders).Body.String()
	assertContains(t, full, "An item can have at most 5 labels.", `data-full="true"`)
	assertNotContains(t, full, "Remove one first.")
```

  Add these tests:

```go
func TestCreateLabelWithColor(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "")
	path := fmt.Sprintf("/items/%d/labels", a.ID)
	rec := do(t, env.H, "POST", path, url.Values{"name": {"gym"}, "color": {"#12ab34"}}, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	got, _ := env.Svc.Get(context.Background(), env.User.ID, a.ID)
	if len(got.Labels) != 1 || got.Labels[0].Color != "#12ab34" {
		t.Fatalf("labels = %+v", got.Labels)
	}
	// After an error the list keeps the chosen color of the new label.
	rec = do(t, env.H, "POST", path, url.Values{"name": {strings.Repeat("x", 31)}, "color": {"#abcdef"}}, htmxHeaders)
	assertContains(t, rec.Body.String(), `data-new-color="#abcdef"`)
}

func TestSetLabelColorRoute(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work")
	work := labelID(t, a, "work")
	path := fmt.Sprintf("/labels/%d/color", work)
	if rec := do(t, env.H, "PUT", path, url.Values{"color": {"#6B44BB"}}, htmxHeaders); rec.Code != http.StatusNoContent {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}
	if l, _ := env.Svc.Label(context.Background(), env.User.ID, work); l.Color != "#6b44bb" {
		t.Fatalf("color = %q", l.Color)
	}
	if rec := do(t, env.H, "PUT", path, url.Values{"color": {"red"}}, htmxHeaders); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad color: %d, want 422", rec.Code)
	}
	if rec := do(t, env.H, "PUT", "/labels/999/color", url.Values{"color": {"#000000"}}, htmxHeaders); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown: %d, want 404", rec.Code)
	}
}

func TestDeleteLabelRoute(t *testing.T) {
	env := newTestEnv(t)
	a := addLabeled(t, env.Svc, env.User.ID, "Invoice", "work, home")
	work := labelID(t, a, "work")
	rec := do(t, env.H, "DELETE", fmt.Sprintf("/labels/%d", work), nil, htmxHeaders)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d", rec.Code)
	}
	assertContains(t, rec.Body.String(), `id="list-section"`, ">home</a>")
	assertNotContains(t, rec.Body.String(), ">work</a>")
	if _, err := env.Svc.Label(context.Background(), env.User.ID, work); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("label still there: %v", err)
	}
	// Deleting the label of the filter goes to the full list.
	home := labelID(t, a, "home")
	rec = do(t, env.H, "DELETE", fmt.Sprintf("/labels/%d", home), nil, filtered(home))
	if rec.Header().Get("HX-Redirect") != "/" {
		t.Fatalf("filter label: HX-Redirect = %q", rec.Header().Get("HX-Redirect"))
	}
}
```

  Rewrite `TestLabelsOfAnotherUserCannotBeChanged` to use the new routes:

```go
	do(t, env.H, "PUT", fmt.Sprintf("/labels/%d/color", id), url.Values{"color": {"#d1382e"}}, htmxHeaders)
	do(t, env.H, "DELETE", fmt.Sprintf("/labels/%d", id), nil, htmxHeaders)
	l, err := env.Svc.Label(ctx, bob.ID, id)
	if err != nil || l.Color == "#d1382e" {
		t.Fatalf("bob's label = %+v, %v", l, err)
	}
```

  Add `"errors"` to the imports. In `TestLabelsPage` remove the part from
  `// Delete asks first` to its end (the route goes away now; Task 3 removes the whole test).

- [ ] **Step 2: Run the tests and see them fail.**
  Run: `go test ./internal/web/ -run 'Label|Plus'`
  Expected: FAIL (old markup, routes missing).

- [ ] **Step 3: Implement the view.** In `internal/web/views.go` replace `pickerView`:

```go
// pickerOption is one line of the label list.
type pickerOption struct {
	todo.Label
	On bool // the item has the label
}

// pickerView is the data for the label list that + opens on an item.
type pickerView struct {
	Item     todo.Item
	Options  []pickerOption // all the user's labels, by name
	Full     bool           // the item has the most labels it can have
	NewColor string         // the color of a new label
	Value    string         // the search field (what the user typed, after an error)
	Error    string
}
```

  In `labelsMessage` the 50-labels text becomes
  `"You have 50 labels. Delete one first (× on its line)."`.

  In `internal/web/labels_handlers.go` replace the `pickerView` builder:

```go
// pickerView loads the label list for item: all the user's labels, and the
// color the next new label gets (as the store gives it).
func (s *server) pickerView(r *http.Request, item todo.Item) (pickerView, error) {
	labels, err := s.svc.Labels(r.Context(), userID(r))
	if err != nil {
		return pickerView{}, err
	}
	v := pickerView{Item: item, Full: len(item.Labels) >= todo.MaxLabelsPerItem,
		NewColor: todo.LabelColors[len(labels)%len(todo.LabelColors)]}
	for _, l := range labels {
		v.Options = append(v.Options, pickerOption{Label: l, On: item.HasLabel(l.ID)})
	}
	return v, nil
}
```

  In `addItemLabel` read `color := r.FormValue("color")`, pass it to `AddLabel`, and in the
  error branch after `v.Value, v.Error = name, msg` add:

```go
		if c, ok := todo.CleanColor(color); ok {
			v.NewColor = c
		}
```

  Add the handlers (replace the old `deleteLabel`):

```go
// setLabelColor saves a label's color. The page changes its chips itself.
func (s *server) setLabelColor(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := s.svc.SetLabelColor(r.Context(), userID(r), id, r.FormValue("color"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, todo.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, todo.ErrBadLabel):
		http.Error(w, "Pick a color.", http.StatusUnprocessableEntity)
	default:
		s.serverError(w, r, err)
	}
}

// deleteLabel deletes a label: it comes off all items, the items stay. The
// answer is the list section. If the list is filtered by that label, the
// page goes to the full list instead.
func (s *server) deleteLabel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	filter, err := s.labelFilter(r)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.svc.DeleteLabel(r.Context(), userID(r), id); errors.Is(err, todo.ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		s.serverError(w, r, err)
		return
	}
	if filter != nil && filter.ID == id {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	lv, err := s.listView(r, currentUser(r).HideDone)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, part{"list", lv})
}
```

  In `internal/web/server.go` replace the line `POST /labels/{id}/delete` with:

```go
	mux.Handle("PUT /labels/{id}/color", s.protect(s.setLabelColor))
	mux.Handle("DELETE /labels/{id}", s.protect(s.deleteLabel))
```

- [ ] **Step 4: Implement the markup.** In `internal/web/templates/item.html` add two icons
  after `plus-icon`:

```html
{{define "tick-icon"}}<svg width="14" height="14" viewBox="0 0 12 12" fill="none" aria-hidden="true"><path d="M2.5 6.2l2.3 2.3 4.7-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>{{end}}

{{define "back-icon"}}<svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true"><path d="M7.5 2.5L4 6l3.5 3.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>{{end}}
```

  In the `item` template the chip becomes
  `<span class="chip" data-color="{{.Color}}" data-label="{{.ID}}">`. Do the same for the filter
  chip in `list.html:13` (`data-label="{{.ID}}"` after `data-color`) and the chip in
  `form.html` ("New items get the label").

  Replace the whole `label-picker` template:

```html
{{define "label-picker"}}<div class="picker label-picker" id="picker-{{.Item.ID}}" role="dialog" aria-label="Labels of “{{.Item.Text}}”" data-item="{{.Item.ID}}" data-full="{{.Full}}" data-new-color="{{.NewColor}}">
<div class="picker-main">
<input type="text" class="picker-search" value="{{.Value}}" placeholder="Search or create…" aria-label="Search or create a label" autocomplete="off" maxlength="30"{{if .Error}} aria-invalid="true" aria-describedby="picker-msg-{{.Item.ID}}"{{end}}>
{{if .Error}}<p class="picker-msg error" id="picker-msg-{{.Item.ID}}" role="alert">{{.Error}}</p>{{end}}
<ul class="picker-list" aria-label="Your labels">
{{range .Options}}<li class="picker-opt" data-name="{{.Name}}" data-label="{{.ID}}" data-items="{{.Items}}"><button type="button" class="sw" data-color="{{.Color}}" data-label="{{.ID}}" aria-label="Color of “{{.Name}}”" title="Change the color"></button><button type="button" class="picker-name" aria-pressed="{{.On}}"{{if and $.Full (not .On)}} aria-disabled="true"{{end}}>{{.Name}}</button>{{if .On}}<span class="picker-check" title="The item has this label">{{template "tick-icon"}}</span>{{end}}<button type="button" class="picker-x" aria-label="Delete label “{{.Name}}”" title="Delete label">{{template "x-small-icon"}}</button></li>
{{end}}<li class="picker-opt picker-create" hidden><button type="button" class="sw" data-color="{{.NewColor}}" aria-label="Color of the new label" title="Choose a color for the new label"></button><button type="button" class="picker-name"><span class="picker-create-text"></span></button></li>
</ul>
<p class="picker-empty" hidden></p>
{{if .Full}}<p class="picker-msg">An item can have at most 5 labels.</p>{{end}}
</div>
<div class="picker-color" role="group" aria-label="Color" hidden>
<p class="picker-color-head"><button type="button" class="picker-back" data-color-cancel aria-label="Back to the labels">{{template "back-icon"}}</button><span class="picker-color-title"></span></p>
<div class="wheel" role="slider" tabindex="0" aria-label="Hue and strength" aria-valuemin="0" aria-valuemax="359" aria-valuenow="0"><span class="wheel-ring"></span></div>
<p class="color-preview">Preview <span class="chip"></span></p>
<div class="due-actions"><button type="button" class="due-save" data-color-done>Done</button><button type="button" class="due-remove" data-color-cancel>Cancel</button></div>
</div>
</div>{{end}}
```

- [ ] **Step 5: Run the tests.**
  Run: `go vet ./... && go test ./...`
  Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add -A internal
git commit -m "feat: the label list shows all labels; routes to change a label's color and delete it

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 3: Remove the Labels page

**Files:**
- Delete: `internal/web/templates/labels.html`, `internal/web/static/labels.js`
- Modify: `internal/web/labels_handlers.go` (remove `labelRow`, `labelsView`, `labelsPage`)
- Modify: `internal/web/server.go` (`GET /labels` → redirect)
- Modify: `internal/web/templates/page.html:22` (no "Labels" link)
- Modify: `internal/web/static/app.css` (labels page rules)
- Test: `internal/web/labels_test.go`

- [ ] **Step 1: Write the failing test.** Replace `TestLabelsPage` with:

```go
func TestLabelsPageIsGone(t *testing.T) {
	env := newTestEnv(t)
	rec := do(t, env.H, "GET", "/labels", nil, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("GET /labels: %d %q, want 303 to /", rec.Code, rec.Header().Get("Location"))
	}
	assertNotContains(t, do(t, env.H, "GET", "/", nil, nil).Body.String(), `href="/labels"`)
	if rec := do(t, env.H, "GET", "/static/labels.js", nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("labels.js: %d, want 404", rec.Code)
	}
}
```

  In `TestLabelNamesAreEscaped` check `"/"` and `fmt.Sprintf("/items/%d/labels", it.ID)`
  (with `htmxHeaders`) instead of `"/labels"`; keep the item from `addLabeled` in `it`.

- [ ] **Step 2: Run it and see it fail.**
  Run: `go test ./internal/web/ -run 'LabelsPageIsGone|Escaped'`
  Expected: FAIL (the page still answers 200).

- [ ] **Step 3: Implement.**
  - `server.go`: `mux.Handle("GET /labels", http.RedirectHandler("/", http.StatusSeeOther))`.
  - Remove `labelRow`, `labelsView` and `labelsPage` from `labels_handlers.go`.
  - `git rm internal/web/templates/labels.html internal/web/static/labels.js`.
  - `page.html`: the nav becomes
    `Signed in as <strong>{{.Username}}</strong> · <a href="/account">Account</a> · <form …>`.
  - `app.css`: remove the block from `/* Labels page */` to the end of the
    `@media (min-width: 481px)` block for `.label-buttons`, but keep `.wheel`, `.wheel:focus-visible`,
    `.wheel-ring`, `.color-preview`, `.btn-small` (and `.primary`, `.danger`,
    `.danger-solid`), `.confirm`, `.confirm p`, `.confirm-buttons`, and the phone rules for
    `.confirm`, `.confirm-buttons` and `.confirm .btn-small`. Before you remove a class, run
    `grep -rn "<class>" internal/web/templates internal/web/static/*.js` and keep it if a
    template or script uses it. Change `.chip, .sw, .color-btn {` to `.chip, .sw {`.

- [ ] **Step 4: Run the tests.**
  Run: `go vet ./... && go test ./...`
  Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add -A internal
git commit -m "feat: remove the Labels page; /labels goes to the list

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 4: wheel.js — the color wheel for the label list

**Files:**
- Create: `internal/web/static/wheel.js`
- Modify: `internal/web/templates/page.html` (load it after `chips.js`)
- Create: `internal/web/wheel_script_test.go`

**Interfaces:**
- Consumes: `hexToHsl`, `hslToHex` from `chips.js`.
- Produces: `wheelPick(dx, dy) → [hue, strength]`, `wheelRing(hue, strength) → [left%, top%]`,
  `makeWheel(el, onChange) → { set(hex), color() }`. `onChange(hex)` runs on each pointer or
  arrow key change, not on `set`.

- [ ] **Step 1: Write the failing test** `internal/web/wheel_script_test.go`:

```go
package web_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// wheelHarness runs the math of wheel.js and prints one line per case.
const wheelHarness = `
const src = require("fs").readFileSync(process.argv[2], "utf8");
const fns = new Function(src + "\nreturn { wheelPick, wheelRing };")();
const r = (a) => a.map((x) => Math.round(x * 1000) / 1000).join(" ");
console.log("top " + r(fns.wheelPick(0, -0.5)));
console.log("right " + r(fns.wheelPick(0.5, 0)));
console.log("half down " + r(fns.wheelPick(0, 0.25)));
console.log("outside " + r(fns.wheelPick(0, -2)));
console.log("ring right " + r(fns.wheelRing(90, 1)));
console.log("ring middle " + r(fns.wheelRing(0, 0)));
`

func TestWheelScriptMath(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	harness := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(harness, []byte(wheelHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, "static/wheel.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	want := "top 0 1\nright 90 1\nhalf down 180 0.5\noutside 0 1\nring right 100 50\nring middle 50 50"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPageLoadsWheelScript(t *testing.T) {
	env := newTestEnv(t)
	body := do(t, env.H, "GET", "/", nil, nil).Body.String()
	i, j := strings.Index(body, `src="/static/chips.js"`), strings.Index(body, `src="/static/wheel.js"`)
	k := strings.Index(body, `src="/static/app.js"`)
	if i < 0 || j < i || k < j {
		t.Fatal("want chips.js, wheel.js, app.js in this order")
	}
}
```

- [ ] **Step 2: Run it and see it fail.**
  Run: `go test ./internal/web/ -run Wheel`
  Expected: FAIL (`static/wheel.js` does not exist).

- [ ] **Step 3: Implement** `internal/web/static/wheel.js`:

```js
// The color wheel of the label list (spec 2026-10-20-label-list-manage-design.md;
// first on the Labels page, spec 2026-10-20-label-color-wheel-design.md). The
// angle is the hue, the distance from the middle the strength; the middle is
// gray. Needs chips.js (hexToHsl, hslToHex).
"use strict";

// wheelPick returns [hue, strength] for a point dx, dy from the middle of
// the wheel, in parts of its width (-0.5 to 0.5).
function wheelPick(dx, dy) {
  return [(Math.atan2(dx, -dy) * 180 / Math.PI + 360) % 360, Math.min(1, 2 * Math.hypot(dx, dy))];
}

// wheelRing returns the place of the ring for hue and strength, in percent
// of the wheel's size from its left and top edge.
function wheelRing(hue, strength) {
  const a = hue * Math.PI / 180;
  return [50 + 50 * strength * Math.sin(a), 50 - 50 * strength * Math.cos(a)];
}

// makeWheel makes el a wheel. onChange gets each color the user picks.
// set(hex) moves the wheel to a color; color() returns the current one.
function makeWheel(el, onChange) {
  const ring = el.querySelector(".wheel-ring");
  let hue = 0, strength = 0, dragging = false;
  const color = () => hslToHex(hue, strength, 0.5);

  function show(notify) {
    const [left, top] = wheelRing(hue, strength);
    ring.style.left = left + "%";
    ring.style.top = top + "%";
    ring.style.background = color();
    el.setAttribute("aria-valuenow", String(Math.round(hue) % 360));
    el.setAttribute("aria-valuetext", "hue " + Math.round(hue) % 360 + " degrees, strength " + Math.round(strength * 100) + " percent");
    if (notify) onChange(color());
  }

  function pickAt(e) {
    const r = el.getBoundingClientRect();
    [hue, strength] = wheelPick((e.clientX - r.left) / r.width - 0.5, (e.clientY - r.top) / r.height - 0.5);
    show(true);
  }

  el.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    el.setPointerCapture(e.pointerId);
    el.focus();
    dragging = true;
    pickAt(e);
  });
  el.addEventListener("pointermove", (e) => {
    if (dragging) pickAt(e);
  });
  el.addEventListener("pointerup", () => { dragging = false; });
  el.addEventListener("pointercancel", () => { dragging = false; });
  // ← → turn the hue, ↑ ↓ change the strength; with Shift in bigger steps.
  el.addEventListener("keydown", (e) => {
    const step = e.shiftKey ? 15 : 5;
    switch (e.key) {
      case "ArrowLeft": hue = (hue - step + 360) % 360; break;
      case "ArrowRight": hue = (hue + step) % 360; break;
      case "ArrowUp": strength = Math.min(1, strength + step / 100); break;
      case "ArrowDown": strength = Math.max(0, strength - step / 100); break;
      default: return;
    }
    e.preventDefault();
    show(true);
  });

  return {
    set(hex) {
      const hsl = hexToHsl(hex) || [0, 0, 0.5];
      hue = hsl[0];
      strength = hsl[1];
      dragging = false;
      show(false);
    },
    color,
  };
}
```

  In `page.html` add `<script src="/static/wheel.js" defer></script>` right after the
  `chips.js` line.

- [ ] **Step 4: Run the tests.**
  Run: `go test ./internal/web/ -run 'Wheel|Order'`
  Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/web/static/wheel.js internal/web/wheel_script_test.go internal/web/templates/page.html
git commit -m "feat: wheel.js, the color wheel for the label list

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 5: The page — several labels, color view, delete question

**Files:**
- Modify: `internal/web/static/app.js` (section 13, lines 364-617)
- Modify: `internal/web/static/app.css` (label list rules)
- Test: `internal/web/labels_test.go` (static checks), Chromium check in Task 7

**Interfaces:**
- Consumes: the markup of Task 2, `makeWheel` of Task 4, `paintColor` through the
  `MutationObserver` of `chips.js` (changing `data-color` is enough).

- [ ] **Step 1: Write the failing test** (static, so later edits keep the parts):

```go
func TestLabelListScriptAndStyle(t *testing.T) {
	env := newTestEnv(t)
	js := do(t, env.H, "GET", "/static/app.js", nil, nil).Body.String()
	assertContains(t, js, "reopenFor", "makeWheel(", `"/labels/" + edit.label + "/color"`, "askDelete(", "No items use it.")
	assertNotContains(t, js, "aria-activedescendant", "data-taken")
	css := do(t, env.H, "GET", "/static/app.css", nil, nil).Body.String()
	assertContains(t, css, ".picker-x", ".picker-opt.asking", ".picker-color[hidden]")
}
```

- [ ] **Step 2: Run it and see it fail.**
  Run: `go test ./internal/web/ -run LabelListScript`
  Expected: FAIL.

- [ ] **Step 3: Replace section 13 of `app.js`** (from the `// 13.` comment to the end of the
  `htmx:afterSwap` handler, just before `// 14.`) with:

```js
// 13. Pop-ups on a row: the label list under the + (canvas page "16 ·
// Labels", board "label list – several labels, color wheel, × deletes"; spec
// 2026-10-20-label-list-manage-design.md) and the due date pop-up under the
// calendar icon (canvas page "10 · Due date"). Only one is open at a time.
// Esc, a click outside, or focus that leaves it closes it.
// The label list shows all labels. A click on a name adds or takes off that
// label; the server answers with the row, and the list moves into the new
// row and loads again, so it stays open. Typing narrows the list; a name
// that matches no label offers "Create …". A line's circle shows the color
// wheel in the pop-up; the × asks, then deletes the label.
// The due date pop-up is a form: Enter saves and Tab moves inside it. The
// due text opens it too.
let picker = null; // the open pop-up
let pickerButton = null; // its + or calendar icon
let reopenFor = null; // the item whose label list stays open after the next swap
let pickerBusy = false; // a label request is on its way; focus can leave for a moment
let wheel = null; // the wheel of the open label list (wheel.js)
let colorEdit = null; // { swatch, label, old } while the wheel view is open

function isDuePicker() {
  return picker !== null && picker.classList.contains("due-picker");
}

function isLabelPicker() {
  return picker !== null && !isDuePicker();
}

// focusPicker puts the cursor in the first field of the pop-up: the search
// of the label list, or the date of the due date pop-up.
function focusPicker() {
  const input = picker.querySelector('input:not([type="hidden"])');
  if (input) {
    input.focus();
  }
  return input;
}

// popupButton is the + or calendar icon that a press on target belongs to.
// A due text belongs to the calendar icon of its row.
function popupButton(target) {
  if (!target.closest) {
    return null;
  }
  const due = target.closest("button.due");
  return due ? due.parentElement.querySelector(".due-add") : target.closest(".label-add, .due-add");
}

function closePicker(refocus) {
  if (!picker) {
    return;
  }
  if (colorEdit) {
    paintLabel(colorEdit, colorEdit.old);
    colorEdit = null;
  }
  picker.remove();
  pickerButton.setAttribute("aria-expanded", "false");
  if (refocus && pickerButton.isConnected) {
    pickerButton.focus();
  }
  picker = null;
  pickerButton = null;
  reopenFor = null;
  pickerBusy = false;
  wheel = null;
}

// installPicker gets a pop-up that just came from the server ready.
function installPicker() {
  placePicker();
  filterPicker();
  const el = picker.querySelector(".wheel");
  wheel = el ? makeWheel(el, previewColor) : null;
  return focusPicker();
}

function openPicker(button) {
  closePicker(false);
  pickerButton = button;
  button.setAttribute("aria-expanded", "true");
  const wrap = button.parentElement;
  htmx.ajax("GET", button.dataset.picker, { target: wrap, swap: "beforeend" }).then(() => {
    const box = wrap.querySelector(".picker");
    if (!box || pickerButton !== button) {
      if (box) box.remove();
      return;
    }
    picker = box;
    installPicker();
  });
}

// Open to the left of the + when the list does not fit on the right.
// If it fits neither way (a narrow screen), move it in from the left edge.
function placePicker() {
  const width = document.documentElement.clientWidth;
  picker.classList.remove("flip");
  picker.style.left = "";
  picker.style.removeProperty("--caret");
  if (picker.getBoundingClientRect().right <= width - 8) {
    return;
  }
  picker.classList.add("flip");
  if (picker.getBoundingClientRect().left >= 8) {
    return;
  }
  // The pointer at the top still points at the +.
  const plus = picker.parentElement.getBoundingClientRect();
  picker.classList.remove("flip");
  picker.style.left = (8 - plus.left) + "px";
  picker.style.setProperty("--caret", (plus.left + plus.width / 2 - 8 - 6) + "px");
}

// labelLines are the lines of the label list that the search shows.
function labelLines() {
  return Array.from(picker.querySelectorAll(".picker-opt")).filter((o) => !o.hidden);
}

function cleanName(v) {
  return v.trim().replace(/\s+/g, " ");
}

function filterPicker() {
  const input = picker && picker.querySelector(".picker-search");
  if (!input) {
    return;
  }
  const typed = cleanName(input.value);
  const q = typed.toLowerCase();
  const full = picker.dataset.full === "true";
  let exact = false;
  picker.querySelectorAll(".picker-opt:not(.picker-create)").forEach((o) => {
    const name = o.dataset.name.toLowerCase();
    o.hidden = q !== "" && !name.includes(q);
    exact = exact || name === q;
  });
  const create = picker.querySelector(".picker-create");
  create.hidden = q === "" || exact || full;
  create.dataset.name = typed;
  create.querySelector(".picker-create-text").textContent = "Create “" + typed + "”";
  create.querySelector(".sw").setAttribute("aria-label", "Color of the new label “" + typed + "”");
  const empty = picker.querySelector(".picker-empty");
  empty.hidden = labelLines().length > 0;
  empty.textContent = full ? "No label has this name." : "Type a name to create a label.";
}

// sendForList sends a label change. The answer replaces the row or the list
// section; the htmx:afterSwap handler below keeps the list open.
function sendForList(method, url, options) {
  reopenFor = picker.dataset.item;
  pickerBusy = true;
  htmx.ajax(method, url, options);
}

// toggleLabel adds the line's label to the item, or takes it off. The Create
// line makes the label with the color of its circle.
function toggleLabel(line) {
  const id = picker.dataset.item;
  const name = line.querySelector(".picker-name");
  if (name.getAttribute("aria-disabled") === "true") {
    return;
  }
  const row = { target: "#item-" + id, swap: "outerHTML" };
  if (line.classList.contains("picker-create")) {
    sendForList("POST", "/items/" + id + "/labels", { ...row, values: { name: line.dataset.name, color: picker.dataset.newColor } });
  } else if (name.getAttribute("aria-pressed") === "true") {
    sendForList("DELETE", "/items/" + id + "/labels/" + line.dataset.label, row);
  } else {
    sendForList("POST", "/items/" + id + "/labels", { ...row, values: { name: line.dataset.name } });
  }
}

function smallButton(text, cls, data) {
  const b = document.createElement("button");
  b.type = "button";
  b.className = cls;
  b.setAttribute(data, "");
  b.textContent = text;
  return b;
}

// askDelete turns a line into the question whether to delete its label.
function askDelete(line) {
  const n = Number(line.dataset.items);
  const p = document.createElement("p");
  p.textContent = "Delete “" + line.dataset.name + "”? " +
    (n === 0 ? "No items use it." : "It comes off " + n + (n === 1 ? " item" : " items") + ". The items stay.");
  const yes = smallButton("Delete", "btn-small danger-solid", "data-ask-delete");
  const buttons = document.createElement("div");
  buttons.className = "confirm-buttons";
  buttons.append(yes, smallButton("Cancel", "btn-small", "data-ask-cancel"));
  const ask = document.createElement("div");
  ask.className = "picker-ask";
  ask.setAttribute("role", "alert");
  ask.append(p, buttons);
  line.classList.add("asking");
  line.append(ask);
  yes.focus();
}

function closeAsk(line) {
  line.classList.remove("asking");
  line.querySelector(".picker-ask").remove();
  line.querySelector(".picker-x").focus();
}

// paintLabel shows color on all chips and circles of the edited label, or on
// the circle of the new label.
function paintLabel(edit, color) {
  if (edit.label) {
    document.querySelectorAll('[data-label="' + edit.label + '"][data-color]').forEach((el) => { el.dataset.color = color; });
  } else {
    edit.swatch.dataset.color = color;
  }
}

function previewColor(color) {
  if (!colorEdit) {
    return;
  }
  picker.querySelector(".picker-color .chip").dataset.color = color;
  paintLabel(colorEdit, color);
}

// openColor shows the wheel in place of the list.
function openColor(swatch, line) {
  const isNew = line.classList.contains("picker-create");
  colorEdit = { swatch, label: isNew ? "" : line.dataset.label, old: swatch.dataset.color };
  const view = picker.querySelector(".picker-color");
  view.querySelector(".picker-color-title").textContent =
    (isNew ? "Color of the new label “" : "Color of “") + line.dataset.name + "”";
  const chip = view.querySelector(".chip");
  chip.textContent = line.dataset.name;
  chip.dataset.color = colorEdit.old;
  picker.querySelector(".picker-main").hidden = true;
  view.hidden = false;
  wheel.set(colorEdit.old);
  view.querySelector(".wheel").focus();
}

// endColor goes back to the list. keep saves the color: a label's color at
// once, the new label's color when the label is created.
function endColor(keep) {
  const edit = colorEdit;
  colorEdit = null;
  const color = keep ? wheel.color() : edit.old;
  paintLabel(edit, color);
  if (!edit.label) {
    picker.dataset.newColor = color;
  } else if (keep && color !== edit.old) {
    saveColor(edit, color);
  }
  picker.querySelector(".picker-color").hidden = true;
  picker.querySelector(".picker-main").hidden = false;
  edit.swatch.focus();
}

function saveColor(edit, color) {
  fetch("/labels/" + edit.label + "/color", { method: "PUT", body: new URLSearchParams({ color }) })
    .then((res) => {
      if (!res.ok) throw new Error("status " + res.status);
    })
    .catch(() => {
      paintLabel(edit, edit.old);
      const line = document.getElementById("error");
      if (line) line.textContent = "The color was not saved. Try again.";
    });
}

document.addEventListener("click", (e) => {
  const button = popupButton(e.target);
  if (button) {
    if (pickerButton === button) {
      closePicker(true);
    } else {
      openPicker(button);
    }
    return;
  }
  if (!isLabelPicker() || !picker.contains(e.target) || pickerBusy) {
    return;
  }
  const line = e.target.closest(".picker-opt");
  if (e.target.closest("[data-color-done]")) endColor(true);
  else if (e.target.closest("[data-color-cancel]")) endColor(false);
  else if (!line) return;
  else if (e.target.closest("[data-ask-delete]")) sendForList("DELETE", "/labels/" + line.dataset.label, { target: "#list-section", swap: "outerHTML" });
  else if (e.target.closest("[data-ask-cancel]")) closeAsk(line);
  else if (e.target.closest(".picker-x")) askDelete(line);
  else if (e.target.closest(".sw")) openColor(e.target.closest(".sw"), line);
  else if (e.target.closest(".picker-name")) toggleLabel(line);
});

// A press in a pop-up must not take the focus out of its field; a press on
// another field of the due date pop-up still moves the focus there. Its
// buttons must not take the focus first: Safari and Firefox on macOS do not
// focus a pressed button, so the field would blur and focusout would close
// the pop-up before the click. A press on the pop-up's own button keeps the
// focus, so its click can close the pop-up.
document.addEventListener("mousedown", (e) => {
  if (!picker) {
    return;
  }
  if (picker.contains(e.target)) {
    if (!e.target.closest(isDuePicker() ? "input" : ".picker-search")) {
      e.preventDefault();
    }
  } else if (popupButton(e.target) === pickerButton) {
    e.preventDefault();
  } else {
    closePicker(false);
  }
});

document.addEventListener("input", (e) => {
  if (picker && e.target.matches(".picker-search")) {
    filterPicker();
  }
});

// Keys in the label list: ↓ goes from the search field to the names, ↑ ↓
// move between names, Enter in the search field picks the first line. In
// the wheel view Enter is Done and Esc is Cancel; elsewhere Esc closes the
// question or the list.
document.addEventListener("keydown", (e) => {
  if (!isLabelPicker() || !picker.contains(e.target)) {
    return;
  }
  if (colorEdit) {
    if (e.key === "Escape" || (e.key === "Enter" && e.target.matches(".wheel"))) {
      e.preventDefault();
      endColor(e.key === "Enter");
    }
    return;
  }
  if (e.key === "Escape") {
    e.preventDefault();
    const asking = e.target.closest(".picker-opt.asking");
    if (asking) closeAsk(asking);
    else closePicker(true);
    return;
  }
  const names = labelLines().map((l) => l.querySelector(".picker-name"));
  if ((e.key === "ArrowDown" || e.key === "ArrowUp") && e.target.matches(".picker-search, .picker-name")) {
    e.preventDefault();
    const at = names.indexOf(e.target) + (e.key === "ArrowDown" ? 1 : -1);
    if (at < 0) picker.querySelector(".picker-search").focus();
    else if (names.length) names[Math.min(at, names.length - 1)].focus();
  } else if (e.key === "Enter" && e.target.matches(".picker-search")) {
    e.preventDefault();
    if (names.length && !e.isComposing && !pickerBusy) toggleLabel(names[0].closest(".picker-opt"));
  }
});

// Esc closes the due date pop-up without saving. Enter submits its form.
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && isDuePicker() && picker.contains(e.target)) {
    e.preventDefault();
    closePicker(true);
  }
});

// Focus that leaves a pop-up closes it (the due date pop-up without
// saving). The decision waits one tick, as for an edit row (section 2).
// While a label request is on its way, the list is moved to a new row and
// the focus is lost for a moment: that does not close it.
document.addEventListener("focusout", (e) => {
  if (!picker || !picker.contains(e.target)) {
    return;
  }
  const box = picker;
  setTimeout(() => {
    const now = document.activeElement;
    if (picker === box && !pickerBusy && !box.contains(now) && now !== pickerButton) {
      closePicker(false);
    }
  }, 0);
});

// A request of the label list that failed: the list is not waiting any more.
document.addEventListener("htmx:afterRequest", (e) => {
  if (pickerBusy && e.detail.failed) {
    pickerBusy = false;
    reopenFor = null;
  }
});

// The pop-up belongs to the row: a swap of the row closes it. An error
// answer replaces only the pop-up (HX-Retarget): go on in the new one. After
// a label change the row (or the list) comes back: the old list moves into
// the new row and loads again, so it stays open.
document.addEventListener("htmx:afterSwap", () => {
  if (!picker || picker.isConnected) {
    return;
  }
  const fresh = pickerButton.isConnected && pickerButton.parentElement.querySelector(".picker");
  const button = !fresh && reopenFor && document.getElementById("label-add-" + reopenFor);
  if (button) {
    reopenFor = null;
    pickerButton = button;
    button.setAttribute("aria-expanded", "true");
    button.parentElement.append(picker);
    placePicker();
    htmx.ajax("GET", button.dataset.picker, { target: picker, swap: "outerHTML" });
    return;
  }
  reopenFor = null;
  pickerBusy = false;
  if (fresh) {
    picker = fresh;
    const input = installPicker();
    if (input && input.matches(".picker-search")) {
      input.setSelectionRange(input.value.length, input.value.length);
    }
    return;
  }
  if (pickerButton.isConnected) {
    pickerButton.setAttribute("aria-expanded", "false");
  }
  picker = null;
  pickerButton = null;
  wheel = null;
});
```

  Note: the old `choose()` set `focusAfterSwap`; it is gone. The `htmx:beforeSwap` handler of
  section 1 does nothing for these requests, because the focus is not on `.check`, `.delete` or
  `.chip-x`.

- [ ] **Step 4: Replace the label list CSS** in `app.css` (from `.picker-list {` to the
  `@media (hover: none)` block before the due date pop-up comment) with:

```css
.picker-list { list-style: none; margin: 0; padding: 0; max-height: 196px; overflow-y: auto; }
.picker-opt {
  display: flex; align-items: center; gap: 6px; min-height: 32px; padding: 0 4px 0 8px;
  border-radius: 5px;
}
.picker-opt[hidden], .picker-empty[hidden], .picker-main[hidden], .picker-color[hidden] { display: none; }
.picker-opt:hover, .picker-opt:focus-within { background: var(--ground); }
/* The color circle of a line: the label's color, a larger click area. */
.picker-opt .sw {
  position: relative; flex: none; width: 14px; height: 14px; padding: 0; border-radius: 50%; cursor: pointer;
  background: var(--l-c); border: 1px solid rgba(31, 30, 27, 0.25); box-shadow: inset 0 0 0 2px var(--card);
}
.picker-opt .sw::after { content: ""; position: absolute; inset: -6px; }
.picker-opt .sw:hover { box-shadow: inset 0 0 0 2px var(--card), 0 0 0 2px var(--accent); }
.picker-name {
  flex: 1 1 auto; min-width: 0; padding: 6px 0; text-align: left; cursor: pointer; overflow-wrap: anywhere;
  font: inherit; color: inherit; background: none; border: none;
}
.picker-name[aria-disabled="true"] { cursor: default; color: var(--done); }
.picker-check { flex: none; display: inline-flex; color: var(--accent); }
/* The × of a line shows when the pointer is on the line or focus is in it. */
.picker-x {
  flex: none; width: 24px; height: 24px; padding: 0; border: none; border-radius: 50%; opacity: 0; cursor: pointer;
  display: inline-flex; align-items: center; justify-content: center; background: none; color: var(--muted);
}
.picker-opt:hover .picker-x, .picker-opt:focus-within .picker-x { opacity: 1; }
.picker-x:hover { background: var(--line); color: var(--error); }
.picker-opt .sw:focus-visible, .picker-name:focus-visible, .picker-x:focus-visible, .picker-back:focus-visible {
  outline: 2px solid var(--accent); outline-offset: 2px;
}
.picker-opt.asking, .picker-opt.asking:hover { padding: 2px 0; background: none; }
.picker-opt.asking > :not(.picker-ask) { display: none; }
.picker-ask {
  flex: 1 1 auto; display: flex; flex-direction: column; gap: 8px; padding: 8px; font-size: 13px;
  background: #FCEDEC; border: 1px solid #F2C4C0; border-radius: 6px;
}
.picker-ask p { margin: 0; }
/* The wheel view in place of the list. */
.picker-color { display: flex; flex-direction: column; gap: 10px; padding: 4px; }
.picker-color-head { margin: 0; display: flex; align-items: center; gap: 6px; font-size: 13px; font-weight: 700; color: var(--muted); }
.picker-back { display: inline-flex; padding: 2px; border: none; background: none; color: var(--accent); cursor: pointer; }
.picker-color .due-actions { justify-content: flex-start; }
.picker-msg { margin: 4px; font-size: 13px; color: var(--muted); }
.picker-msg.error { color: var(--error); }
.picker-empty { margin: 4px 8px; font-size: 13px; color: var(--done); }
@media (hover: none) {
  .picker-search { font-size: 16px; padding: 8px; }
  .picker-opt { min-height: 44px; font-size: 16px; }
  .picker-opt .sw { width: 22px; height: 22px; }
  .picker-opt .sw::after { inset: -8px; }
  .picker-x { opacity: 1; width: 36px; height: 36px; }
  .picker-ask { font-size: 15px; }
  .picker-ask .btn-small { flex: 1 1 0; min-height: 44px; font-size: 15px; }
  .picker-color .wheel { width: 200px; height: 200px; }
}
```

  Remove the old `.picker-opt[aria-selected="true"]`, `.picker-opt .sw` and `.sw-new` rules
  (they are inside the replaced block).

- [ ] **Step 5: Run the tests.**
  Run: `go vet ./... && go test ./...` and `node --check internal/web/static/app.js`
  Expected: PASS, no syntax error.

- [ ] **Step 6: Commit.**

```bash
git add internal/web/static/app.js internal/web/static/app.css internal/web/labels_test.go
git commit -m "feat: the label list adds several labels, changes colors and deletes labels (canvas page 16)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 6: Docs

**Files:**
- Modify: `README.md` ("What it does")
- Modify: `docs/superpowers/specs/2026-10-14-labels-design.md`,
  `docs/superpowers/specs/2026-10-20-label-color-wheel-design.md`,
  `docs/superpowers/specs/2026-10-14-label-plus-design.md` (status lines)
- Modify: `docs/superpowers/specs/2026-10-20-label-list-manage-design.md` (status: built)

- [ ] **Step 1: README.** Replace the labels bullet with:

```markdown
- Put labels such as "work" or "home" on items with the small + on each item. It opens a
  searchable list of all your labels: click names to add or remove several labels, click a
  label's color circle to pick any color from a color wheel, and use its × to delete it. Point
  at a label on an item to remove it with its ×. Click a label to see only its items.
```

- [ ] **Step 2: Specs.** Under the title of each older spec add one line:
  `Note (2026-10-20): the Labels page is gone; see 2026-10-20-label-list-manage-design.md.`
  (for `label-plus-design.md`: `Note (2026-10-20): the list now shows all labels and stays
  open; see 2026-10-20-label-list-manage-design.md.`). In the new spec set
  `Status: approved and built (2026-10-20).`

- [ ] **Step 3: Commit.**

```bash
git add README.md docs
git commit -m "docs: label list spec, plan and README; the Labels page is gone

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 7: Check in the browser

- [ ] **Step 1:** Run `go run ./cmd/todo` in the worktree (in the background), sign up a test
  user at http://127.0.0.1:8811, and add two items with a few labels.
- [ ] **Step 2:** In Chromium at 1000 px wide, try each frame of the canvas board: add and remove
  several labels with the list open; change a color (chips change while dragging; Done keeps it
  after a reload; Cancel and Esc put it back); type a new name, pick its color, create it; delete
  a label (question, Delete, Cancel); 5 labels on one item; Esc, click outside, Tab out.
  Open `/?label=<id>`, then delete that label from the list: the page must go to `/` (this
  checks that htmx follows `HX-Redirect` on a 204 answer). Take the filter's label off an item
  in the filtered list: the row leaves and the list closes.
- [ ] **Step 3:** Repeat at 390 px with touch emulation: the × always shows, the circles and
  lines are larger, the list fits on the screen.
- [ ] **Step 4:** Check the browser console: no errors, no CSP messages. Fix any problem with a
  test first where a Go test can show it, then commit the fix.
- [ ] **Step 5:** Stop the server.

### Task 8: Merge to main and deploy (asked by the user)

- [ ] **Step 1:** In the worktree: `go vet ./... && go test ./...` → PASS.
- [ ] **Step 2:** Merge without squashing, from the main checkout:

```bash
git -C /home/kostrzewa/claude-personal/todo merge --ff-only labels-in-popup
```

  If `--ff-only` fails because `main` moved, stop and ask the user.
- [ ] **Step 3:** Deploy from the main checkout:
  `cd /home/kostrzewa/claude-personal/todo && mise run fly:deploy`
- [ ] **Step 4:** Check: `curl -fsS -o /dev/null -w '%{http_code}\n' https://toodoo.fly.dev/login`
  → `200`, and `curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' https://toodoo.fly.dev/labels`
  → `303` to `/`. Run `mise run fly:logs` for a short time and look for errors.
- [ ] **Step 5:** Remove the worktree and the branch only if the user asks.
