# Hosting on Fly.io — design

Date: 2026-10-02. Status: design approved by the user; spec waiting for review.

## 1. Goal

The todo app runs on the internet at `https://mytodo.fly.dev`. One command deploys a new
version. The SQLite file survives restarts and new versions. The admin commands
(`todo users …`) work on the server. Common operations are mise tasks.

## 2. Decisions

| Topic | Decision |
|-------|----------|
| Platform | Fly.io, one machine in region `waw` (Warsaw) |
| App name | `mytodo` (must be free on Fly; change in `fly.toml` and `mise.toml` if taken) |
| Address | `mytodo.fly.dev`, HTTPS from Fly. Own domain later is possible. |
| Image build | Small `Dockerfile`, built by Fly's remote builder. No local Docker. |
| Machine | 256 MB RAM, shared CPU, 256 MB swap. Stops when idle, starts on a request. |
| Storage | One 1 GB volume `todo_data` at `/data`. Database `/data/todo.db`. |
| Backups | Fly volume snapshots only: daily, kept 5 days. |
| Deploys | Manual, through mise tasks that wrap `flyctl`. |
| App change | Limit of parallel password checks 4 → 2, so the checks fit in 256 MB. |

## 3. Image

- Build stage `golang:1.27.1`: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /todo ./cmd/todo`.
  The SQLite driver is pure Go, so the binary is static.
- Run stage `gcr.io/distroless/static-debian12`: no shell and no package manager.
- `ENTRYPOINT ["/todo"]`, `CMD ["-addr", ":8080", "-db", "/data/todo.db", "-trust-proxy"]`.
- The app runs as root inside its own Fly microVM, because Fly mounts volumes as root and the
  image has no shell to change the owner. The VM holds only this app and its data.
- `.dockerignore` keeps local databases, the built binary, `.git`, `tmp/`, `docs/`, and
  `.superpowers/` out of the build.

## 4. Fly configuration (`fly.toml`)

- `app = "mytodo"`, `primary_region = "waw"`, `kill_signal = "SIGTERM"`, `kill_timeout = "10s"`,
  `swap_size_mb = 256`.
- `[mounts]`: `source = "todo_data"`, `destination = "/data"`, `snapshot_retention = 5`.
- `[http_service]`: `internal_port = 8080`, `force_https = true`, `auto_stop_machines = "stop"`,
  `auto_start_machines = true`, `min_machines_running = 0`.
- Health check: `GET /login` every 30 s, timeout 5 s, grace period 10 s.
- `[[vm]]`: `memory = "256mb"`, `cpu_kind = "shared"`, `cpus = 1`.
- Exactly one machine, because a volume belongs to one machine. Deploys use `--ha=false`.

## 5. What the app needs from Fly's proxy

- Fly passes the original `Host` header, so the cross-origin check works.
- Fly appends the client address to `X-Forwarded-For`. The app's `-trust-proxy` rule takes the
  rightmost entry, so it gets the real client address. The app port is reachable only through
  Fly's proxy.
- HTTPS ends at Fly's edge. The browser sees HTTPS, so the `Secure` session cookie works.
- The limit counters live in memory and reset when the machine stops while idle. This is
  accepted: an attacker who sends requests keeps the machine running.

## 6. Memory

Each password check uses 64 MiB. With a limit of 2 parallel checks, the checks use at most
128 MiB. The Go program, SQLite, and the VM's Linux need about 50–70 MB more. This fits in
256 MB. A 3rd check at the same moment waits about 0.2 s. A flood of logins cannot make the
machine run out of memory. Swap is only a safety margin.

## 7. Storage and backups

The 1 GB volume costs about USD 0.15 per month, also while the machine is stopped. 100,000
items use about 50 MB. The volume can grow later with `fly volumes extend` but cannot shrink.
Fly takes a snapshot every day and keeps it 5 days. To restore, create a new volume from a
snapshot and attach it to the machine. `docs/deploy.md` describes the steps.

## 8. mise tasks (`mise.toml`)

`[tools] flyctl = "latest"`, so `mise install` installs flyctl. Tasks:

| Task | What it does |
|------|--------------|
| `fly:setup` | Creates the app and the volume (once). |
| `fly:build` | Builds the image on Fly without deploying (`fly deploy --build-only`). |
| `fly:deploy` | Deploys the current code (`fly deploy --ha=false`). |
| `fly:status` | Shows the machine state. |
| `fly:logs` | Shows the app logs. |
| `fly:open` | Opens the app in the browser. |
| `fly:users` | Lists the users. |
| `fly:reset-password <name>` | Sets a new random password and prints it. |
| `fly:delete-user <name>` | Asks for confirmation, then deletes the user and their items. |
| `fly:snapshots` | Lists the volume snapshots. |
| `fly:stop` / `fly:start` | Scales to 0 or 1 machines. |

The user commands run the app binary through `fly ssh console -C`, with `-db /data/todo.db`
and `--` before the username.

## 9. Documentation

`docs/deploy.md`: first setup (mise install, `fly auth login`, `fly:setup`, `fly:deploy`,
first sign-up), daily use (deploy, logs, user commands), snapshots and restore, costs and
how to stop the app.

## 10. Tests and checks

1. `TestDeployConfigMatches` (Go, `cmd/todo`): the db path in the `Dockerfile` is under the
   mount destination in `fly.toml`. The `-addr` port equals `internal_port`. `-trust-proxy` is
   set. The db path in `mise.toml` equals the one in the `Dockerfile`.
2. A Go test in `internal/auth` checks that the limit of parallel password checks times the
   memory per check is at most 128 MiB.
3. `mise tasks ls` lists all tasks without errors.
4. User checks after `fly auth login`:
   - `mise run fly:build` builds the image.
   - After the first deploy, the user signs up at `https://mytodo.fly.dev`.
   - After 6 wrong logins, the "login blocked" line in `mise run fly:logs` shows the user's own
     public IP. This proves the proxy rule.
   - `mise run fly:users` lists the new user.

## 11. Out of scope

Own domain, automatic deploys, Litestream or other continuous backups, more than one machine.
