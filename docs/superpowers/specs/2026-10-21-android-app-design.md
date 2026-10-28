# Android app: design spec

Date: 2026-10-21
Status: native app chosen 2026-10-21; section 9 uses the recommended answers until Zbyszek says otherwise. Steps 1 to 6 are built
(2026-10-21); the app has not run on a phone yet (the build machine has no emulator).
Builds on: `2026-10-01-todo-app-design.md`, `2026-10-01-users-design.md`, `2026-10-14-labels-design.md`,
`2026-10-15-due-popup-design.md`
Mockup: canvas "Todo app UI proposal", page "17 · Android app".

## 1. Goal and scope

An Android app for TooDoo. A user logs in with their existing account and sees the same list as
on the web: add, edit, check off, delete, due dates, labels, label filter, hide done, drag order.
Due items notify on the phone.

Not in the first version (they stay on the web, the app opens the web page for them):

- Sign-up (it has the proof-of-work check and the honeypot), password change, account delete.
- Offline edits. The app shows the last list it loaded when there is no network, read-only,
  with a "No connection" bar. Edits need the server.
- Push from the server (Firebase). See section 6.
- Home-screen widget, Wear OS, tablets with two panes.
- Google Play. The first version is an APK you install yourself (section 7).

## 2. The main choices

### 2.1 Native app or a wrapped web page

| | A. Native, Kotlin + Jetpack Compose (recommended) | B. WebView / Trusted Web Activity around toodoo.fly.dev | C. Installable web app (PWA), no Android code |
|---|---|---|---|
| Look and feel | Real Android: Material 3, system back, swipe, share sheet, dark mode | The web page in a frame; long-press, back and keyboard feel like a browser | Same as B, from Chrome's "Add to home screen" |
| Notifications | Exact local alarms, also when the app is closed | Only while the page is open (today's browser polling), or full push work | Web push only, needs a push server anyway |
| Server work | A JSON API (section 3) | None, or a manifest | A manifest and a service worker |
| App work | The most: about 2 to 3 thousand lines of Kotlin | A few hundred lines | None |
| Upkeep | Two UIs to keep in step: each new feature is done twice | Features come for free | Features come for free |

A gives a real phone app and reliable reminders. Its cost is that every later feature is built
twice. B and C are cheap, but they are "the website with an icon", and the website already
works on a phone (the guide has a phone screenshot). If that is all you want, pick C and the
work is a manifest and an icon; it is a much smaller job.

Cross-platform tools (Flutter, Kotlin Multiplatform, React Native) only pay off with an iOS app
too. You asked for Android only, so they are left out.

### 2.2 Where the code lives

| | `android/` in this repo (recommended) | A new repo, e.g. `localghost/toodoo-android` |
|---|---|---|
| API and app change together | One commit, one branch | Two repos to keep in step |
| Go tooling | Unaffected: Go ignores `android/`; the Dockerfile copies only what it needs | |
| Size | The repo gains a Gradle project and its wrapper (about 100 KB) | |

### 2.3 Server address

The login screen has the server address, filled in with `https://toodoo.fly.dev`. It is hidden
under "Server" so most people never see it; a self-hosted copy (README "Run it yourself") can
use the app too. Only `https://` is accepted, except `http://10.0.2.2` and `http://127.0.0.1`
for the emulator.

## 3. Server: a JSON API

The web routes answer with HTML pieces for htmx, so the app needs its own routes. They live
under `/api/v1/`, call the same `todo.Service` and `auth.Service` methods as the web handlers,
and add no new rules: limits, the overdue order and label rules stay where they are.

### 3.1 Log-in and tokens

- `POST /api/v1/login` with JSON `{"username", "password"}` answers
  `{"token", "expires_at", "user": {"id", "username", "hide_done"}}`; a wrong password is 401
  `bad_login`. It uses `accounts.LogIn` with
  "keep me logged in", so the token is a normal 30-day session in the `sessions` table (stored
  hashed, counts toward the 20 sessions per user). The same per-IP and per-username limits as
  the web log-in apply; when locked, the answer is 429 with `Retry-After`.
- Every other API call sends `Authorization: Bearer <token>`. API routes do **not** read the
  session cookie, and web routes do not read the header. So the API needs no CSRF defense, and
  a web page can never call it with the user's cookie.
- **Renewal:** when a token has less than 15 days left, the server makes its expiry 30 days
  from now again. Someone who opens the app at least every two weeks stays logged in. Needs one
  new store method, `ExtendSession`. Without this, the app asks for the password every 30 days.
- `POST /api/v1/logout` ends the session. A password change on the web already ends all other
  sessions, so it logs the phone out too.
- `401` with `{"error": "logged_out"}` makes the app go to the log-in screen.

### 3.2 Routes

All answers are JSON. Errors are `{"error": "<code>", "message": "<text for the user>"}` with
the same texts the web shows ("Text is too long", "At most 5 labels per item", ...).

| Route | What it does |
|---|---|
| `GET /api/v1/me` | `{"id", "username", "hide_done"}`. |
| `GET /api/v1/items?label=<id>` | `{"items": [...], "open": 3, "done": 1, "hide_done": false, "label": null}`, in list order (overdue first). Respects the user's "hide done" setting, like the web. |
| `POST /api/v1/items` | `{"text", "labels": ["work"]}`. Returns the item. |
| `GET /api/v1/items/{id}` | One item. |
| `PATCH /api/v1/items/{id}` | Any of `{"text", "done"}`. Returns the item. |
| `DELETE /api/v1/items/{id}` | 204. |
| `PUT /api/v1/items/{id}/due` | `{"date": "2026-10-22", "time": "14:30"}`; empty time is all-day; both empty clears it. Uses the zone from the `Time-Zone` header. |
| `POST /api/v1/items/{id}/postpone` | `{"minutes": 5 \| 10 \| 15 \| 30}`. |
| `POST /api/v1/items/{id}/move` | `{"after": <id or 0>}`. 409 for an overdue item. |
| `POST /api/v1/items/{id}/labels` | `{"name"}`: adds the label, creating it if needed. |
| `DELETE /api/v1/items/{id}/labels/{label}` | Removes it from the item. |
| `POST /api/v1/items/done/delete?label=<id>` | Clears done items. Returns `{"deleted": 4}`. |
| `GET /api/v1/labels` | `[{"id", "name", "color", "items"}]`. |
| `PUT /api/v1/labels/{id}/color` | `{"color": "#rrggbb"}`. |
| `DELETE /api/v1/labels/{id}` | 204. |
| `PUT /api/v1/settings` | `{"hide_done": true}`. |

An item is:

```json
{"id": 12, "text": "Pay rent", "done": false, "due_at": "2026-10-22T12:30:00Z",
 "due_all_day": false, "overdue": true, "labels": [{"id": 3, "name": "home", "color": "#38c75d"}]}
```

Times are UTC (RFC 3339). The phone shows them in its own zone and sends its zone name
(`Time-Zone: Europe/Warsaw`) so all-day dates and "14:30" mean the same as on the web.

### 3.3 Code

- `internal/web/api.go`: the handlers, plus `apiProtect` (bearer token) next to `protect`
  (cookie). The log-in limits move into `checkLogin`, shared by the web and the API.
- `internal/auth`: `AuthenticateApp` renews the token; the store gains `ExtendSession`.
- The phone's zone comes from the `Time-Zone` header when there is no `todo_tz` cookie.
- `Content-Security-Policy` and the other headers stay; `CrossOriginProtection` lets native
  requests through already (they send no `Origin` or `Sec-Fetch-Site`).
- No schema change.

## 4. The app

### 4.1 Screens

1. **Log in:** username, password, "Server" (folded), Log in. "No account? Sign up on the web"
   opens `/signup` in the browser.
2. **List** (the main screen):
   - Top bar: "TooDoo", the counter ("3 open · 1 done"), a label filter button, a menu with
     "Hide done", "Clear done items", "Labels", "Account (web)", "Log out".
   - With a filter, a chip under the bar: "Label: work ×".
   - Rows: a round check box, the text, and a second line with the due date (red when overdue)
     and the label chips in their colors. Done items are gray and struck through, like the web.
   - An add field at the bottom, above the keyboard, with a send button. While filtered, the
     new item gets the filter's label (same as the web).
   - Swipe a row left to delete it, with "Undo" in a snackbar for 5 seconds (the delete is sent
     when the snackbar goes away). Pull down to reload.
   - Long-press the grip to drag a row. Overdue rows have no grip, as on the web.
   - Since 2026-10-22: no check box and no grip; swipe right for done, hold a row to drag.
     See `2026-10-22-android-gestures-design.md`.
3. **Item sheet** (tap a row): a bottom sheet with the text field, "Due" (date and time pickers
   from Material 3, time typed or picked, "All day", "Clear"), "Labels" (chips with × and an
   "Add label" button), "Postpone" (5, 10, 15, 30 min, only for due items), and Delete.
4. **Label picker:** a searchable list like the web's, check marks for the item's labels, a
   color dot that opens a color wheel, a × to delete a label (asks first). Typing a new name
   offers "Create “name”".
5. **Labels and filter:** the filter button opens the same list without check marks; a tap on
   a label filters.

The app uses the label colors from the server and makes the chip shades with the same rule as
`static/chips.js` (light fill, dark text with 7:1 contrast), with a dark-mode version.

### 4.2 Reminders

- After each list load the app sets one alarm for each open item with a due time in the future
  (Android `AlarmManager`, exact when the user allows it, inexact otherwise, a few minutes late).
- The notification shows the text and the due time, with "Done" and "Postpone 10 min" buttons.
  They call the API without opening the app.
- All-day items notify at 09:00, as on the web.
- The phone notifies by itself and does not call `/notifications/claim`, so a browser tab that
  is open on the laptop still notifies too. (The other way, "notify once on whichever device
  sees it first", would make the phone miss reminders the laptop claimed while you were away.)
- Alarms are set again after a reboot and when the app opens.
- Android 13+ asks for the notification permission on the first due date the user sets, not at
  start-up.

### 4.3 Code layout

```
android/
  settings.gradle.kts, build.gradle.kts, gradlew, gradle/   Gradle wrapper and version catalog
  app/src/main/kotlin/com/codeleet/toodoo/
    api/        ApiClient (OkHttp + kotlinx.serialization), the JSON types
    data/       TodoRepository, the token store (DataStore), the cached list (a JSON file)
    ui/         LoginScreen, ListScreen, ItemSheet, LabelPicker, ColorWheel, theme
    reminders/  AlarmScheduler, ReminderReceiver, BootReceiver
  app/src/test/  unit tests (repository, chip colors, time parsing "1430" → 14:30)
```

- Kotlin, Jetpack Compose with Material 3, one Activity, ViewModels with `StateFlow`.
- Android 8.0 (API 26) and newer; that is about 97 % of phones in use.
- Few libraries: OkHttp, kotlinx.serialization, DataStore, Compose. No Retrofit, no Room, no
  Firebase, no dependency-injection framework.
- The token stays in the app's private storage and is left out of Android backups.
- Package name `com.codeleet.toodoo` (any name you like; it cannot change once installed by others).

## 5. Docs

- README: an "Android app" line in "What it does", and how to build it in "Run it yourself".
- User guide: a short "Android app" section (install, log in, reminders). Screenshots of the
  app come from the emulator; `mise run guide:shots` stays for the web pages.
- `docs/android.md`: building, signing and installing the APK.

## 6. Later, if wanted

- **Push from the server (Firebase Cloud Messaging):** reminders arrive even if the phone has
  not opened the app since the due date was set on the laptop. Today the phone learns about a
  new due date only when it loads the list (on start, on pull-down, and every 30 minutes in the
  background with WorkManager). Push needs a Firebase project, a key on Fly, and a sender in the
  server.
- Offline edits with a sync queue.
- A home-screen widget with today's items.
- Sign-up in the app.

## 7. Build and install

- `mise run android:build` makes `android/app/build/outputs/apk/release/toodoo.apk`; it needs
  Java 17 and the Android SDK (Android Studio brings both, or `mise` can install them).
- The release key lives on your machine, outside the repo. Without it the task makes a debug
  APK, which is fine for your own phone.
- Install with `adb install` or by opening the APK on the phone.
- Optional: attach the APK to a GitHub release, or serve it from `/android` on the site.

## 8. Order of work

1. API with tests, deployed (the web app does not change).
2. App: log in, list, add, check off, delete, pull to reload.
3. Item sheet: edit, due date, postpone, labels, label filter, hide and clear done.
4. Drag order, color wheel.
5. Reminders.
6. Docs, guide section, release build.

Each step is a commit on main, and steps 1 to 3 already give a usable app.

## 9. Questions for you

1. Native app (A), or is an installable web app (C) enough?
2. `android/` in this repo, or a new repo?
3. Token renewal (stay logged in while you use the app), or log in again every 30 days?
4. Phone reminders on their own (recommended), or "notify once across devices"?
5. Any package name or app name other than `com.codeleet.toodoo` / "TooDoo"?
