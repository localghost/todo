#!/usr/bin/env bash
# The fixed steps of a Google Play release of the Android app (SKILL.md in
# this folder says when to run them). Run it from the repo root:
#
#   release.sh prepare [VERSION|patch|minor|major]   sets the new version
#   release.sh finish                                tests, builds, commits, tags, pushes
#
# Between the two, the agent writes android/play/release-notes/<version>.txt
# and updates the user guide.
set -euo pipefail

gradle_file=android/app/build.gradle.kts
notes_dir=android/play/release-notes

die() { echo "release: $*" >&2; exit 1; }

version_name() { sed -n 's/^ *versionName = "\(.*\)"$/\1/p' "$gradle_file"; }
version_code() { sed -n 's/^ *versionCode = \([0-9]*\)$/\1/p' "$gradle_file"; }
last_tag() { git describe --tags --match 'android-v*' --abbrev=0 2>/dev/null || true; }
is_semver() { [[ $1 =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; }
# True if version $1 is higher than version $2.
version_gt() { [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -1)" = "$1" ]; }

# bump CURRENT patch|minor|major
bump() {
  local major minor patch
  IFS=. read -r major minor patch <<<"$1"
  case $2 in
    major) echo "$((major + 1)).0.0" ;;
    minor) echo "$major.$((minor + 1)).0" ;;
    patch) echo "$major.$minor.$((patch + 1))" ;;
  esac
}

# The commits since the last release that a release can be about.
release_commits() {
  local tag; tag=$(last_tag)
  git log --format='%h %s' ${tag:+"$tag..HEAD"} -- android docs/user-guide.md
}

# The checkout of main (the first worktree that has it).
main_checkout() {
  git worktree list --porcelain | awk '/^worktree /{w=substr($0,10)} /^branch refs\/heads\/main$/{print w; exit}'
}

# The text of the notes between <en-US> and </en-US>, or an error.
notes_text() {
  local file=$1
  [ -f "$file" ] || die "no release notes: write $file first"
  python3 - "$file" <<'EOF'
import sys
lines = open(sys.argv[1], encoding="utf-8").read().strip("\n").split("\n")
if len(lines) < 3 or lines[0] != "<en-US>" or lines[-1] != "</en-US>":
    sys.exit("release: the notes must start with the line <en-US> and end with the line </en-US>")
import re
if any(re.fullmatch(r"</?[a-z]{2}-[A-Z]{2}>", l.strip()) for l in lines[1:-1]):
    sys.exit("release: a language tag line inside the notes; keep one <en-US> at the start and one </en-US> at the end")
text = "\n".join(lines[1:-1]).strip()
if not text:
    sys.exit("release: the notes are empty")
if len(text) > 500:
    sys.exit(f"release: the notes have {len(text)} characters; Google Play takes at most 500")
print(text)
EOF
}

prepare() {
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not in a git work tree"
  [ -f "$gradle_file" ] || die "run this from the repo root"
  git diff --quiet && git diff --cached --quiet || die "there are uncommitted changes; commit or stash them first"
  git fetch -q origin
  git merge-base --is-ancestor origin/main HEAD || die "HEAD does not contain origin/main; start from the newest main"

  local main; main=$(main_checkout)
  for f in local.properties keystore.properties; do
    if [ ! -f "android/$f" ] && [ -n "$main" ] && [ -f "$main/android/$f" ]; then
      cp "$main/android/$f" "android/$f"
      echo "copied android/$f from $main"
    fi
  done
  [ -f android/keystore.properties ] || die "android/keystore.properties is missing: Google Play needs a signed bundle (docs/android.md, section 3)"

  local current code tag want new
  current=$(version_name); code=$(version_code); tag=$(last_tag); want=${1:-}
  is_semver "$current" || die "versionName \"$current\" is not MAJOR.MINOR.PATCH"
  if [ -z "$want" ] && [ -z "$tag" ]; then
    want=$current # the first release keeps the version in the build file
  elif [ -z "$want" ]; then
    if git log --format=%s "$tag..HEAD" -- android | grep -qE '^feat(\(|!|:)'; then want=minor; else want=patch; fi
  fi
  case $want in
    patch | minor | major) new=$(bump "$current" "$want") ;;
    *) is_semver "$want" || die "\"$want\" is not a version (MAJOR.MINOR.PATCH) or patch, minor, major"; new=$want ;;
  esac
  git rev-parse -q --verify "refs/tags/android-v$new" >/dev/null && die "the tag android-v$new exists already"

  if [ -z "$tag" ] && [ "$new" = "$current" ]; then
    echo "first release: version $new, versionCode $code (unchanged)"
  else
    version_gt "$new" "$current" || die "the new version $new must be higher than $current"
    code=$((code + 1))
    sed -i "s/^\( *versionName = \)\".*\"$/\1\"$new\"/; s/^\( *versionCode = \)[0-9]*$/\1$code/" "$gradle_file"
    [ "$(version_name)" = "$new" ] && [ "$(version_code)" = "$code" ] || die "could not write the version into $gradle_file"
    echo "version $current -> $new, versionCode $code"
  fi
  echo "release notes: $notes_dir/$new.txt (<en-US> ... </en-US>, at most 500 characters)"
  echo "commits since ${tag:-the start}:"
  release_commits | sed 's/^/  /'
}

finish() {
  [ -f "$gradle_file" ] || die "run this from the repo root"
  local version code tag notes
  version=$(version_name); code=$(version_code); tag=android-v$version
  notes=$(notes_text "$notes_dir/$version.txt")
  git rev-parse -q --verify "refs/tags/$tag" >/dev/null && die "the tag $tag exists already"

  echo "== tests"
  (cd android && ./gradlew -q testDebugUnitTest verifyPaparazziDebug)
  go test ./internal/web/

  echo "== bundle"
  (cd android && ./gradlew -q bundleRelease)
  local aab=android/app/build/outputs/play/toodoo-$version.aab
  [ -f "$aab" ] || die "the build made no $aab"
  jarsigner -verify "$aab" | grep -q "jar verified" || die "$aab is not signed"

  echo "== commit, tag, push"
  git add -u
  git add "$notes_dir/$version.txt"
  git commit -q -F - <<EOF
release(android): $version

versionCode $code. Release notes: $notes_dir/$version.txt

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
  git tag -a "$tag" -m "Android $version" -m "$notes"
  local branch main
  branch=$(git branch --show-current)
  if [ "$branch" != main ]; then
    main=$(main_checkout)
    [ -n "$main" ] || die "no checkout of main found; merge $branch into main yourself"
    git -C "$main" merge -q --ff-only "$branch"
  fi
  git push -q origin main
  git push -q origin "$tag"

  echo
  echo "Released $version (versionCode $code), tag $tag, pushed."
  echo "Upload: $(pwd)/$aab"
  echo "Release notes for the Play Console:"
  sed 's/^/  /' "$notes_dir/$version.txt"
}

case ${1:-} in
  prepare) shift; prepare "$@" ;;
  finish) finish ;;
  *) echo "usage: release.sh prepare [VERSION|patch|minor|major] | release.sh finish" >&2; exit 2 ;;
esac
