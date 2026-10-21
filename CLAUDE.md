# Project rules for coding agents

## README

- Keep `README.md` up to date. When a change adds, removes, or changes a feature that a user
  can see, update the "What it does" list in the same branch, before the merge.
- When a change affects how to run or deploy the app (flags, `config.yaml`, Go version, Fly
  app name or address), update "Run it yourself" and the app link too.
- Keep the README short: a summary, the link to the deployed app, and short sections. Details
  belong in `docs/`.

## User guide

- The app serves `docs/user-guide.md` at `/guide`. When a change adds, removes, or changes a
  feature that a user can see, update the guide in the same branch, before the merge.
- If the change alters a screen that a guide screenshot shows, run `mise run guide:shots` and
  commit the new images in `docs/guide-images/`. Do not edit the images by hand.
