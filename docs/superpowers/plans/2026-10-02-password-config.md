# Configurable minimum password length (config.yaml) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

## Context

The minimum password length is a constant (10) in `internal/auth/rules.go`, repeated in the
error text and in two hints in `internal/web/templates/auth.html`. The user wants it in a
`config.yaml` file, with 8 as the default, and the file must be part of the Fly deployment.

**Goal:** `password.min_length` in `config.yaml` (default 8) sets the rule, the error text, and
the hints; the Fly image contains the same `config.yaml`.

**Path:** bounded (existing rule + small config loader). No separate spec. Execution: native,
then one whole-branch review by a fresh reviewer. Worktree
`~/claude-personal/todo-worktrees/password-config`, branch `password-config`, from `main`
(`5270852`). One commit per task. No push.

## Design

1. **File** `config.yaml` in the repo root (one file for local runs and for Fly):
   ```yaml
   # Settings for the todo app. Restart the app after a change.
   password:
     # Shortest allowed password, in characters (1–200).
     # It applies when someone signs up or changes a password, not to existing passwords.
     min_length: 8
   ```
2. **Package `internal/config`** (new): `Config{Password{MinLength int}}`, `Default()`
   (8, from `auth.DefaultMinPasswordChars`), `Load(path string, allowMissing bool) (Config, error)`.
   Strict YAML (`KnownFields(true)`), so a typo in a key stops the app with a clear error.
   An empty file gives the defaults. `min_length` outside 1–200 is an error. Library:
   `go.yaml.in/yaml/v3` (the maintained successor of gopkg.in/yaml.v3; first YAML dependency).
3. **Flag** `-config` (default `config.yaml`). A missing file at the default path gives the
   defaults. A missing file at a path given with `-config` is an error, so a wrong path in
   the deployment cannot silently fall back.
4. **auth:** `DefaultMinPasswordChars = 8`; `ValidatePassword(password string, minChars int)`;
   option `WithMinPasswordChars(n)`; method `MinPasswordChars() int`. Error text
   `Use at least N characters.` with the configured number. Existing passwords keep working
   (login does not check the rule). The admin `reset-password` still makes 16 characters.
5. **web:** `signupView` and `accountView` get `MinPassword`; both hints say
   `At least {{.MinPassword}} characters`. Only the number changes, so no Claude Design page.
6. **Fly:** run stage `COPY config.yaml /etc/todo/config.yaml`; CMD adds
   `"-config", "/etc/todo/config.yaml"`. To change the setting: edit `config.yaml`, then
   `mise run fly:deploy`. `TestDeployConfigMatches` checks the COPY target equals the `-config` path.

## Review Focus

1. A typo in `config.yaml` (for example `min_lenght`) silently uses the default. → Task 1 test (unknown key is an error).
2. A wrong `-config` path in the Dockerfile silently uses the default. → Task 3 (explicit path must exist) + `TestDeployConfigMatches`.
3. The hint shows a different number than the rule. → Task 2 web test with min 12.
4. `min_length: 0` or a value above the maximum (200) makes every password valid or none valid. → Task 1 range test.
5. The setting applies to sign-up but not to password change. → Task 2 test for both.

---

### Task 1: config package

**Files:** `internal/config/config.go`, `internal/config/config_test.go`, `go.mod`/`go.sum`
(`go get go.yaml.in/yaml/v3`), `internal/auth/rules.go` (export `MaxPasswordChars = 200`,
add `DefaultMinPasswordChars = 8`; no behavior change yet). Copy this plan to
`docs/superpowers/plans/2026-10-02-password-config.md`.

- [ ] Step 1: worktree, plan copy, ledger (`Pre-flight: Task 2 consumes auth.DefaultMinPasswordChars/MaxPasswordChars from Task 1`).
- [ ] Step 2: failing tests in `config_test.go`: `Default().Password.MinLength == 8`; file with `min_length: 12` → 12; empty file → 8; missing file + allowMissing → 8; missing file without allowMissing → error naming the path; unknown key `min_lenght` → error; `min_length: 0` and `201` → error mentioning `password.min_length`. Run → FAIL (package missing).
- [ ] Step 3: implement `config.go` (decode into `Default()`, `io.EOF` = empty file, validate range with `auth.MaxPasswordChars`). Errors start with the file path.
- [ ] Step 4: `go test -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: config package with password.min_length`.

### Task 2: auth and web use the configured minimum

**Files:** `internal/auth/rules.go`, `internal/auth/service.go`, `internal/auth/auth_test.go`,
`internal/web/auth_handlers.go`, `internal/web/templates/auth.html`, web tests (the test env
needs a way to pass auth options; add one to the `newTestEnv` helper), the users spec
(`docs/superpowers/specs/2026-10-01-users-design.md`: rule "at least 10" → "configurable,
default 8").

- [ ] Step 1: failing tests: `ValidatePassword(x, 8)`: 7 chars → `Use at least 8 characters.`, 8 chars (multi-byte) OK, 201 → max error; `NewService` default rejects 7 and accepts 8 on SignUp; `WithMinPasswordChars(12)` rejects 11 on SignUp and on ChangePassword; web: default env shows `At least 8 characters` on `/signup` and `/account`; env with min 12 shows `At least 12 characters` on both and a sign-up with 11 characters shows `Use at least 12 characters.` Update existing tests that expect 10. Run → FAIL.
- [ ] Step 2: implement (field + option + method in `Service`; views from `s.accounts.MinPasswordChars()`; every `signupView` goes through `newSignupView`).
- [ ] Step 3: `go test -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: minimum password length from the auth service, default 8`.

### Task 3: main, config.yaml, Fly image, docs

**Files:** `cmd/todo/main.go`, `config.yaml` (new), `Dockerfile`, `cmd/todo/deploy_test.go`,
`docs/deploy.md` (new section "Settings"), hosting spec (one line).

- [ ] Step 1: failing test: extend `TestDeployConfigMatches`: Dockerfile has `COPY config.yaml <p>` and CMD `"-config", "<p>"` with the same `<p>`; `config.yaml` loads with `config.Load` and has `MinLength == 8`. Run → FAIL.
- [ ] Step 2: `main.go`: flag `-config`; `allowMissing` = flag not set (`flag.Visit`); `config.Load`; `auth.NewService(store, auth.WithMinPasswordChars(cfg.Password.MinLength))`; log `config` path and `min_password_length` at start. Add `config.yaml`, Dockerfile COPY + CMD, docs section.
- [ ] Step 3: local checks: build from a `.dockerignore`-filtered copy; run with `-config` → file with `min_length: 12` → `/signup` shows `At least 12 characters`; run with `-config missing.yaml` → exits with an error; run in a folder without `config.yaml` and no flag → starts, hint says 8.
- [ ] Step 4: `go test -count=1 ./...`, `go vet ./...` → exit 0. Commit `feat: config.yaml with min_length 8, loaded by the server and shipped in the Fly image`.

## Final check

Fresh whole-branch review (most capable model) with the Review Focus. Then the user's
browser check: sign-up hint says 8, a 7-character password is rejected, an 8-character one
works; change `min_length` to 12, restart, the hint says 12. Then the merge question.

Open, not part of this plan: deploy-guide fix for calling `fly` without mise (waiting for the
user's choice: `fly:login` task + `mise exec`, or a note about mise activation).
