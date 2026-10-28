---
name: play-release
description: Use when releasing a new version of the toodoo Android app to Google Play, or when asked to bump the Android version, write Play Store release notes, or build the signed AAB for an upload.
argument-hint: "[version|patch|minor|major]"
---

# Release the Android app to Google Play

One call prepares a release: the version, the release notes, the user guide, the signed
bundle, a commit, the tag `android-v<version>`, and the push. The script `release.sh` in this
folder does the fixed steps. You write the release notes and the guide text.

## Steps

1. Work in a new worktree from `origin/main` (the user's rule):
   `git fetch origin && git worktree add ../todo-worktrees/release-<YYYY-MM-DD> -b release-<YYYY-MM-DD> origin/main`,
   then work in that folder.
2. Run `bash .claude/skills/play-release/release.sh prepare $ARGUMENTS`.
   - An argument is a version (`1.2.0`) or `patch`, `minor`, `major`.
   - Without one: a `feat` commit in `android/` since the last tag means `minor`, else `patch`.
     The first release (no `android-v*` tag yet) keeps the version in the build file.
   - It prints the version, the notes file and the commits since the last release. If it stops
     with an error, report the error and stop.
3. Write the release notes into the printed file, `android/play/release-notes/<version>.txt`:
   ```
   <en-US>
   • A short line for each change a user can see
   </en-US>
   ```
   - Only user-visible changes. Leave out refactoring, tests, build and docs-only commits.
   - Plain English for a CEFR B2 reader. At most 500 characters between the tags (Google Play's limit).
   - For the first release: a short summary of what the app does.
4. Update the user guide, `docs/user-guide.md`, section "Android app":
   - Each user-visible change in the commits has a short, correct description.
   - The "Install" bullet says that the app is on Google Play:
     https://play.google.com/store/apps/details?id=dev.toodoo
   - If a screen that a guide image shows has changed, run `mise run android:shots`. Keep only the
     images that really changed; restore images that changed only their dates with `git checkout`.
   - If a feature changed, keep the README list "What it does" right (project CLAUDE.md).
5. Run `bash .claude/skills/play-release/release.sh finish`. It checks the notes, runs the Android and
   guide tests, builds the signed bundle, commits, tags, fast-forwards `main` and pushes `main` and the tag.
6. Report to the user:
   - the bundle path (`android/app/build/outputs/play/toodoo-<version>.aab`) and the release notes text,
   - the tag and the commit,
   - the next steps: in the Play Console, create a release on a track, upload the bundle, and paste the notes,
   - that the guide change goes live with the next web deploy (`mise run fly:deploy`); this skill does not deploy.

## Common mistakes

| Mistake | Fix |
|---------|-----|
| Starting from an old branch | `prepare` stops if HEAD does not contain `origin/main`; start from a fresh worktree. |
| Notes over 500 characters, or without the `<en-US>` lines | `finish` stops; shorten or fix the file and run `finish` again. |
| A test fails in `finish` | Nothing is committed yet. Fix the cause, then run `finish` again. |
| Running `prepare` twice | The second run raises the version again. If the first run worked, go on with step 3. |
| Committing `keystore.properties` | Git ignores it; never `git add -f` it. |
