# Android app

The Android app is in `android/`: Kotlin and Jetpack Compose, Android 8.0 (API 26) and newer.
It talks to the server's JSON API under `/api/v1`. The design is in
`docs/superpowers/specs/2026-10-21-android-app-design.md`.

## 1. What you need

- Java 17 or newer.
- The Android SDK with `platforms;android-36` and `build-tools;36.0.0`. Android Studio brings
  both. Without Android Studio, install the
  [command-line tools](https://developer.android.com/studio#command-line-tools-only) and run
  `sdkmanager "platforms;android-36" "build-tools;36.0.0" "platform-tools"`.
- Tell Gradle where the SDK is: set `ANDROID_HOME`, or write `sdk.dir=/path/to/sdk` into
  `android/local.properties` (git ignores it).

## 2. Build

| Task | Command |
|------|---------|
| Unit tests | `mise run android:test` |
| Build the APK | `mise run android:build` |
| Build the bundle for Google Play | `mise run android:bundle` |
| Make the guide's Android screenshots again | `mise run android:shots` |

`android:build` makes a signed release APK when there is a release key (section 3):
`android/app/build/outputs/apk/release/toodoo.apk`, about 2 MB. Without a key it makes the debug
APK `android/app/build/outputs/apk/debug/toodoo-debug.apk`, which is fine for your own phone.

`android:bundle` makes the signed Android App Bundle that Google Play needs:
`android/app/build/outputs/bundle/release/app-release.aab`. It needs the release key (section 3).

The screenshot tests (`ScreenshotTest.kt`) render the screens on the computer with
[Paparazzi](https://github.com/cashapp/paparazzi), so they need no phone or emulator. Paparazzi
is a 2.0.0 alpha, because 1.3.5 does not work with API 36. Its renderer fails on the time input
(`timeInput`), so that test is ignored for now.

## 3. The release key

Android only updates an installed app with an APK signed by the same key. So make one key, keep
it safe, and never put it in the repo.

1. Make the key once, outside the repo:

       keytool -genkeypair -keystore ~/keys/toodoo.jks -alias toodoo -keyalg RSA -keysize 4096 -validity 10000

2. Write `android/keystore.properties` (git ignores it):

       storeFile=/home/you/keys/toodoo.jks
       storePassword=…
       keyAlias=toodoo
       keyPassword=…

3. `mise run android:build`.

A phone that has the debug APK needs it uninstalled before the release APK goes on, because
the two have different keys.

## 4. Install

- With a cable: turn on USB debugging on the phone, then
  `adb install -r android/app/build/outputs/apk/release/toodoo.apk`.
- Without a cable: copy the APK to the phone and open it. Android asks once to allow installs
  from that app (for example Files).

## 5. How reminders work

- After each load of the list, the app sets an alarm for each open item with a due time in the
  future. All-day items remind at 09:00, as on the web.
- The app also loads the list every 30 minutes in the background, so a due date set on the web
  reminds on the phone too. A due date set less than 30 minutes before it is due may be missed
  if the app is closed.
- On Android 12 and newer, alarms are exact only if the user allows "Alarms & reminders" for the
  app (menu: Allow exact reminders). Otherwise Android may deliver them a few minutes late.
- The phone notifies by itself. A browser tab that is open on a laptop notifies too.

## 6. Change the server

The log-in screen has the server address under "Server". It is `https://toodoo.fly.dev` unless
you change it. Only `https://` addresses work, except `http://10.0.2.2` (the computer, seen from
the emulator) and `http://127.0.0.1`.

## 7. Google Play

- The store icon is `android/play/icon-512.png` (512 × 512, 32-bit PNG). It is the launcher icon
  as a full square, because Google adds the rounded corners. The source is `android/play/icon.svg`.
  To make the PNG again:
  `convert -background none -density 1200 android/play/icon.svg -resize 512x512 PNG32:android/play/icon-512.png`
- The feature graphic (`android/play/feature-graphic.png`, 1024 × 500) and the phone screenshots
  (`android/play/screenshot-1.png` to `screenshot-5.png`, 1080 × 1920) are 24-bit PNG files. To make
  them again: `mise run play:graphics`. The Paparazzi test `PlayScreenshotTest` renders the app
  screens at full size to `android/app/build/play-screens/`. Then `android/play/make.sh` adds the
  captions and the phone frame (`android/play/graphics.html`) with headless Chromium. The dates in
  the screens are relative to today, so they change each time.
- Before each new upload, raise `versionCode` in `android/app/build.gradle.kts` by 1. Google Play
  takes each `versionCode` only once. `versionName` uses semantic versioning (for example `1.0.1`),
  and users see it on Google Play.
