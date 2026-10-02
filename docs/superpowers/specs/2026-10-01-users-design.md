# Users and login — design spec

Date: 2026-10-01
Status: waiting for review
Builds on: `2026-10-01-todo-app-design.md`, `2026-10-01-due-dates-design.md`

## 1. Goal and scope

The app gets user accounts, so people can use it over the internet. Each person has an own todo list and sees only their own items.

The feature has these parts:

- Sign up with a username and a password. There is no email.
- Log in, log out, and "Keep me logged in".
- Change the password, and delete the account.
- Spam protection for sign-up and protection against password guessing, without third-party services.
- Commands on the server for the admin: list users, reset a password, delete a user.

These items are not part of this feature:

- Email, and a password reset by the user.
- Sharing lists between users.
- Roles, or an admin web page.
- Login with Google, GitHub, or others (OAuth).
- Two-factor login (2FA).

## 2. Decisions

| Topic | Decision |
|---|---|
| Who uses the app | People over the internet |
| Sign-up | Open, with username and password, no email |
| Spam protection | Own checks: rate limit, honeypot field, minimum fill time, proof-of-work puzzle. No third party. |
| Forgotten password | The admin resets it on the server |
| Existing items | Deleted. The app starts empty (with an explicit start flag, section 3). |
| Hosting | Not decided. The app runs behind any HTTPS proxy. |
| Sessions | Stored in SQLite; random token in a secure cookie (approach A) |
| Password hashing | argon2id from `golang.org/x/crypto` (current version) |
| Go version | Go 1.27.1 (upgraded before this feature) |
| Sign-up password field | One field with a "Show" button (canvas page 13, option A) |
| List header | Plain links: "Signed in as <name> · Account · Log out" (canvas page 13, option A) |
| Account page | Change password, and delete account with the username typed again (canvas page 13) |

UI reference: canvas page "13 · Accounts" in https://claude.ai/artifact/7mnRhC4Jd8jbnbAhjN7Woq

## 3. Data

The database gets a schema version in `PRAGMA user_version`. The setup runs each missing version step once, in order, at start. A new database goes to the newest version at once.

| Version | Content |
|---|---|
| 0 or 1 | The schema before this feature (`users` with `name`, the user "default", `items` with due dates) |
| 2 | This feature |

Version 2 has these tables:

| Table | Columns |
|---|---|
| `users` | `id INTEGER PRIMARY KEY`, `username TEXT NOT NULL UNIQUE COLLATE NOCASE`, `password_hash TEXT NOT NULL`, `created_at TEXT NOT NULL` |
| `sessions` | `token_hash TEXT PRIMARY KEY`, `user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `created_at TEXT NOT NULL`, `expires_at TEXT NOT NULL`, `persistent INTEGER NOT NULL` |
| `settings` | `key TEXT PRIMARY KEY`, `value TEXT NOT NULL` (holds the signing key, section 6) |
| `items` | As today, but `user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE` |

The step to version 2 drops the old `items` and `users` tables and creates the new ones. SQLite cannot add `ON DELETE CASCADE` to an existing column, so the tables are created again.

The step to version 3 (2026-10-02) adds `users.hide_done INTEGER NOT NULL DEFAULT 0`, the per-user "hide done items" setting. It keeps all data.

Start rule for old data: before the step to version 2, the server counts the old items. If there are any and the flag `-delete-old-items` is not set, the server does not start. It prints this message and exits with code 1:

```
todo: this database has 12 items from before user accounts.
Start again with -delete-old-items to delete them and continue.
```

With the flag, or with no old items, the step runs. The flag has no effect on a database that is already at version 2.

## 4. Accounts

The rules live in a new package `internal/auth`.

| Rule | Value | Message |
|---|---|---|
| Username characters and length | `^[A-Za-z0-9_-]{3,32}$` | "Use 3–32 letters, digits, - or _." |
| Username unique | Case does not matter ("Zbigniew" = "zbigniew") | "This username is taken." |
| Password length | N–200 characters (counted in Unicode characters). N is `password.min_length` in `config.yaml`, default 8 (changed 2026-10-02; was a fixed 10). | "Use at least N characters." / "Use at most 200 characters." |

The username keeps the case the user typed, for display. Login finds the user without regard to case.

Passwords are hashed with argon2id: 64 MiB memory, 3 passes, 4 threads, a 16-byte random salt, and a 32-byte key. The hash is stored in the standard text form `$argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>`. The check reads the parameters from the stored text, so they can change later without breaking old hashes. The comparison uses a constant-time function.

## 5. Sessions

1. At login and at sign-up, the server makes a token from 32 random bytes (`crypto/rand`). It stores only the SHA-256 hash of the token.
2. The token goes to the browser in the cookie `todo_session`: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`. Browsers accept `Secure` cookies on `http://localhost` too.
3. Without "Keep me logged in", the cookie has no `Max-Age`, so it ends when the browser closes. The server session ends after 12 hours.
4. With "Keep me logged in", the cookie and the session last 30 days.
5. Each login creates a new token. An older cookie in the same browser is replaced.
6. Logout deletes the session in the database and clears the cookie.
7. A password change deletes all other sessions of the user and keeps the current one.
8. Deleting a user deletes all their sessions and items (`ON DELETE CASCADE`).
9. Expired sessions are deleted at start and then every hour.

A middleware in `internal/web` reads the cookie, finds the session, and puts the user ID and username into the request context. All item handlers and the claim route use the user ID from the context. The constant `defaultUserID` is removed.

| Request without a valid session | Answer |
|---|---|
| Normal page request (`GET /`, `GET /account`) | `303` redirect to `/login` |
| htmx request | `401` with the header `HX-Redirect: /login` |
| `POST /notifications/claim` (fetch from app.js) | `401`. app.js stops polling until the page reloads. |

## 6. Security

### Cross-site requests

Go's `http.CrossOriginProtection` replaces the own middleware `sameOriginOnly`. It rejects changing requests (POST, PUT, DELETE) from other websites. The browser marks such requests with `Sec-Fetch-Site` or `Origin`. The login, sign-up, logout, and account forms are protected in the same way.

### Sign-up spam protection

The sign-up page contains a hidden field `form_token`. It holds the time when the page was made and 16 random bytes. A signature (HMAC-SHA256) protects it, made with a key that is created at first start and stored in `settings`.

A sign-up is accepted only if all checks pass:

| Check | Rule |
|---|---|
| Signature | `form_token` has a valid signature |
| Minimum fill time | At least 3 seconds since the page was made |
| Maximum age | At most 1 hour since the page was made |
| One use | Each `form_token` is accepted only once (kept in memory until it is 1 hour old) |
| Honeypot | The hidden field `website` is empty. It is hidden from people with CSS and `aria-hidden`, and it has `tabindex="-1"` and `autocomplete="off"`. |
| Proof of work | `SHA-256(form_token + ":" + nonce)` starts with at least `D` zero bits. The browser finds `nonce` before it sends the form. |
| Rate limit | At most 5 new accounts and at most 30 sign-up attempts per hour from one IP address |

`D` is a constant. Its first value is 16 bits, and the implementation measures it so that a normal laptop browser needs about one second. While the browser works on the puzzle, the button shows "Checking that you are human…".

If a check fails, the page shows "Sign-up failed. Please wait a moment and try again." It never says which check failed. A rule error for the username or the password shows its own message (section 4), and the form keeps the typed username.

### Login protection

| Limit | Rule |
|---|---|
| Per username | After 5 wrong passwords within 15 minutes, logins for this username are blocked for 15 minutes |
| Per IP address | After 20 failed logins within 15 minutes, logins from this address are blocked for 15 minutes |

While blocked, the page shows "Too many attempts. Please try again in 15 minutes." Wrong passwords and unknown usernames give the same message: "Wrong username or password." For an unknown username, the server still checks a dummy hash, so the response time is the same. The counters are kept in memory and reset at a restart.

A known trade-off: someone can block a username for 15 minutes by typing wrong passwords for it. For a small app this is acceptable.

### Client IP behind a proxy

The flag `-trust-proxy` makes the app take the client IP from the last address in `X-Forwarded-For`, which the proxy adds. Without the flag, the app uses the address of the direct connection. Behind Caddy or nginx the flag must be set. Otherwise all users share one address in the limits.

Some proxies put their own address last, for example Fly.io. For them, `-client-ip-header <name>` takes the client IP from a header that the proxy sets (`Fly-Client-IP` on Fly.io). This flag wins over `-trust-proxy`.

The proxy must also pass the original `Host` header (Caddy and nginx `proxy_set_header Host $host` do this). `http.CrossOriginProtection` compares `Origin` with `Host` when a browser sends no `Sec-Fetch-Site`, so a changed `Host` would reject correct form posts.

### Security headers

Every response gets these headers:

| Header | Value |
|---|---|
| `Content-Security-Policy` | `default-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `same-origin` |

The CSP blocks inline scripts and `eval`. htmx needs `eval` for `hx-on` attributes and for trigger filters such as `keydown[key=='Enter']`, and it adds its own `<style>` tag. So this feature changes the existing pages:

1. The htmx configuration gets `"allowEval": false` and `"includeIndicatorStyles": false`.
2. The two `hx-on` attributes in `page.html` (network error line) move to `app.js` as event listeners.
3. The filtered triggers in the edit row move to `app.js`. It listens for Enter and Escape in an edit row and fires the custom events `save-edit` and `cancel-edit`, which the templates already use. The templates keep only `hx-trigger="save-edit"` and `hx-trigger="cancel-edit"`.

The htmx configuration in the `<meta>` tag is not a script, so it stays.

## 7. Pages and routes

New routes:

| Route | Session needed | What it does |
|---|---|---|
| `GET /login` | No | The log-in page. With a valid session: `303` to `/`. |
| `POST /login` | No | Checks username and password. Success: new session, `303` to `/`. Wrong: the page again with the error. Blocked: the page with the "Too many attempts" message and status `429`. |
| `GET /signup` | No | The sign-up page with a fresh `form_token` |
| `POST /signup` | No | Runs the checks of section 6, creates the user and a session (not persistent), `303` to `/` |
| `POST /logout` | Yes | Deletes the session, clears the cookie, `303` to `/login` |
| `GET /account` | Yes | The account page |
| `POST /account/password` | Yes | Fields `current_password`, `new_password`. Success: the account page with "Password changed. You were logged out on your other devices." Wrong current password: "The current password is wrong." |
| `POST /account/delete` | Yes | Field `confirm_username` must match the username (case does not matter). Then deletes the user, clears the cookie, `303` to `/login?deleted=1`. The log-in page then shows "Your account was deleted." |

The log-in, sign-up, and account pages are plain HTML forms with `POST` and redirects, without htmx. They work without JavaScript, except for the puzzle on the sign-up page. All existing item routes and the claim route need a session.

Texts and layout follow canvas page 13:

- Log in: fields "Username" and "Password", the checkbox "Keep me logged in", the button "Log in", and the link "No account yet? Sign up".
- Sign up: "Username" with the hint "3–32 letters, digits, - or _", and "Password" with a "Show" button and the hint "At least N characters" (N from `config.yaml`). The note says "There is no email. If you forget your password, ask the admin." Below are the button "Create account" and the link "Already have an account? Log in".
- List header: "Signed in as <name> · Account · Log out". "Log out" is a small form with a button styled as a link, because logout must be a `POST`.
- Account page: "← Back to my list", "Signed in as <name> · member since <date>", the "Change password" section, and the "Delete account" section in red.

## 8. Admin command line

The binary gets subcommands. Without a subcommand, it starts the server as before.

| Command | Output |
|---|---|
| `todo users list -db todo.db` | A table: `USERNAME  CREATED  ITEMS  SESSIONS`, one line per user, sorted by username |
| `todo users reset-password -db todo.db <name>` | `New password for <name>: <16 random characters>`. All sessions of the user are deleted. |
| `todo users delete -db todo.db <name> [-yes]` | Without `-yes`: "Type the username to delete <name> and all items:" and reads one line. On a match: `Deleted <name>.` |

An unknown username prints `todo: no user "<name>"` and exits with code 1. The commands work while the server runs: SQLite with WAL mode and `busy_timeout` allows this. The commands do not run the start rule for old data. They work only on a database at version 2, and otherwise print a message and exit with code 1.

## 9. Errors and edge cases

| Case | Behavior |
|---|---|
| A user is deleted while their tab is open | The next request has no valid session, so the tab goes to `/login` |
| The session ends during an htmx request | `401` + `HX-Redirect: /login`. The browser opens the log-in page. |
| Log out in one tab, then click in another tab | The other tab goes to `/login` on its next request |
| "Zbigniew" and "zbigniew" | They are the same user. Sign-up of the second name says "This username is taken." |
| Clock change on the server | Session expiry uses the server clock. A large jump can end sessions early or late. This is accepted. |
| Restart of the server | Sessions stay, because they are in the database. The login and sign-up counters and the used `form_token` list reset. |
| Admin resets a password | All sessions of that user end. The user logs in with the new password. |

## 10. Testing

| Area | What the tests check |
|---|---|
| `internal/auth` rules | Username pattern and length, case-insensitive uniqueness, password length (Unicode), messages |
| Hashing | The hash text format, a correct and a wrong password, reading the parameters from the stored text |
| Sessions | Creation, expiry after 12 h and 30 days (with a test clock), rotation at login, logout, other sessions deleted on password change, cleanup |
| Migration | A database file from before this feature: refuses to start without `-delete-old-items` and keeps the items; with the flag: version 2, old items gone. Running twice changes nothing. |
| Isolation | For every item route and the claim route: user A cannot see, change, check, postpone, or delete user B's items (`404`) |
| Access | Each protected route without a session gives a `303` to `/login`, or a `401` with `HX-Redirect` for htmx |
| Cross-site | A `POST` with `Sec-Fetch-Site: cross-site` is rejected |
| Login limits | The 6th wrong password for one username is blocked; the 21st failed login from one IP is blocked; the same message for unknown and wrong; reset after 15 minutes (test clock) |
| Sign-up checks | Each check fails alone (bad signature, too fast, too old, reused token, honeypot filled, weak puzzle, rate limit) with the neutral message. A valid sign-up logs the user in. |
| Headers | CSP and the other headers on pages and fragments. No template contains `hx-on`, a trigger filter (`[`…`]` in `hx-trigger`), or an inline `<script>`. The htmx configuration has `allowEval: false`. |
| Command line | `list`, `reset-password` (sessions gone, new password works), `delete` with and without `-yes`, unknown user |
| Browser (user) | Sign up, log in, "Keep me logged in", log out, change password, delete account, two users in two browsers, puzzle time |

---

Co-authored with [Claude Code](https://claude.com/claude-code).
