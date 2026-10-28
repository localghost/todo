#!/usr/bin/env bash
# Makes the Google Play graphics (mise run play:graphics runs the Paparazzi test
# first): renders graphics.html with headless Chromium, then removes the alpha
# channel, because Google Play takes only 24-bit PNG. Needs headless Chromium
# ($CHROME, or `npx playwright install chromium-headless-shell`) and ImageMagick.
set -euo pipefail
cd "$(dirname "$0")"
chrome=${CHROME:-$(ls ~/.cache/ms-playwright/chromium_headless_shell-*/chrome-headless-shell-linux64/chrome-headless-shell 2>/dev/null | tail -1)}
[ -x "$chrome" ] || { echo "headless Chromium not found; set CHROME" >&2; exit 1; }
ls ../app/build/play-screens/play{1,2,3,4,5}*.png >/dev/null ||
  { echo "no app screens: run ./gradlew testDebugUnitTest --tests 'dev.toodoo.PlayScreenshotTest' first" >&2; exit 1; }
page="file://$PWD/graphics.html"
shoot() { # name width height hash
  "$chrome" --no-sandbox --hide-scrollbars --allow-file-access-from-files --virtual-time-budget=5000 \
    --window-size="$2,$3" --screenshot="$PWD/$1" "$page#$4" 2>/dev/null
  convert "$1" -background '#2450C7' -alpha remove -alpha off "PNG24:$1"
}
shoot feature-graphic.png 1024 500 feature
for n in 1 2 3 4 5; do shoot "screenshot-$n.png" 1080 1920 "$n"; done
echo "Google Play graphics saved in android/play"
