# Deploy to Fly.io

This guide shows how to run the todo app on Fly.io and how to look after it.
All common commands are mise tasks. `mise tasks ls` lists them.

## 1. What runs where

- The app runs on one Fly machine named `mytodo` in Warsaw (`waw`).
- The machine has 256 MB of memory. It stops when nobody uses it and starts on the next request.
- The volume `todo_data` (1 GB) holds the database `/data/todo.db`.
- Fly takes a snapshot of the volume every day and keeps it for 5 days.
- The address is `https://mytodo.fly.dev`.

## 2. First setup

1. Install the tools: `mise trust && mise install`. This installs `flyctl` and `jq`.
2. Log in to Fly: `fly auth login`.
3. Create the app and the volume: `mise run fly:setup`.
   If the name `mytodo` is taken, change it in `fly.toml` (`app`) and in `mise.toml` (`FLY_APP`).
   Then run the task again.
4. Build the image on Fly without deploying it: `mise run fly:build`.
5. Deploy: `mise run fly:deploy`.
6. Open the app: `mise run fly:open`. Sign up with your username and password.

## 3. Check the proxy rule once

The app takes the client address from the `Fly-Client-IP` header that Fly's proxy sets.
Check once that this works:

1. Log out and enter a wrong password 6 times.
2. Run `mise run fly:logs`.
3. Find the line `login blocked`. Its `ip=` value must be your own public IP address.

If the line shows a Fly address, all users share one login limit. Stop and report the problem.

## 4. Daily use

| Task | Command |
|------|---------|
| Deploy a new version | `mise run fly:deploy` |
| Show the logs | `mise run fly:logs` |
| Show the machine state | `mise run fly:status` |

## 5. Users

| Task | Command |
|------|---------|
| List the users | `mise run fly:users` |
| Set a new password | `mise run fly:reset-password <name>` |
| Delete a user and their items | `mise run fly:delete-user <name>` |

The new password appears in the output. Give it to the user. They can change it on the account page.
`fly:delete-user` asks for confirmation first.

If the machine is stopped, these tasks start it first with a web request. This takes a few seconds.

## 6. Snapshots and restore

To list the snapshots, run `mise run fly:snapshots`.

To restore the database from a snapshot:

1. Find the snapshot ID with `mise run fly:snapshots`.
2. Create a new volume from it:
   `fly volumes create todo_data --snapshot-id <snapshot id> --region waw --size 1 --yes`.
3. Find the machine ID with `fly machine list` and the new volume ID with `fly volumes list`.
4. Start a copy of the machine with the new volume:
   `fly machine clone <machine id> --attach-volume <new volume id>:/data`.
5. Remove the old machine at once, so that only the new machine gets requests:
   `fly machine destroy <old machine id> --force`.
   Changes saved on the old machine after step 1 are lost.
6. Open the app and check your items. If they are wrong, keep the old volume and ask for help.
7. When the items are correct, remove the old volume: `fly volumes destroy <old volume id>`.

Fly's documentation: https://fly.io/docs/volumes/snapshots/

## 7. Costs

- The machine costs about USD 2 per month while it runs. It costs nothing while it is stopped.
- The volume costs about USD 0.15 per month, also while the machine is stopped.
- Fly may charge for snapshots per GB. Check Fly's pricing page for the current price.

The machine stops by itself when nobody uses it. So you do not need to stop it to save money.

To take the app off the internet, run `fly scale count 0 --yes`. This command destroys the
machine, but the volume and its data stay. To bring the app back, run `mise run fly:deploy`.
Then run `fly volumes list` and check that the volume is attached to the new machine.

## 8. Grow the volume

If the volume becomes full, run `fly volumes extend <volume id> --size 2`.
A volume can grow, but it cannot shrink.
