# Catalogizer Android TV -- 5-Minute Quickstart

Install the Catalogizer Android TV app on your big-screen device and start browsing media with your remote.

## Prerequisites

- Android TV device: Xiaomi Mi Box 4, NVIDIA Shield, or compatible (minSdk 26)
- TV remote control (D-pad navigation)
- catalog-api server accessible on your LAN
- ADB installed on your development machine

## Step 1: Connect to Your TV via ADB

Enable developer options on your TV, then connect:

```bash
adb connect 192.168.0.214:5555
```

Replace `192.168.0.214` with your TV's actual IP address (find it in Settings > Network on the TV).

Verify the connection:

```bash
adb devices
```

## Step 2: Build and Install the APK

Build from source:

```bash
cd catalogizer-androidtv
./gradlew assembleDebug
```

Install on the connected TV:

```bash
adb install app/build/outputs/apk/debug/app-debug.apk
```

## Step 3: Set Up ADB Reverse Proxy (Optional)

If catalog-api runs on your development machine, let the TV reach it at `localhost:8080`:

```bash
adb reverse tcp:8080 tcp:8080
```

## Step 4: Launch the App

Open **Catalogizer** from the TV's app launcher using your remote.

## Step 5: Enter the Server URL

On the login screen, you need to enter the server URL using the on-screen keyboard and D-pad.

- **With ADB reverse:** Use `http://localhost:8080`
- **On LAN:** Use your server's IP, e.g., `http://192.168.0.100:8080`

Tap **Discover** to auto-detect servers on the local network.

> **D-pad input sequence:** Press **D-pad Center** to activate a text field before typing. Use **Tab** (KEYCODE_TAB) to move between the username, password, and server URL fields.

The server URL is persisted via DataStore across restarts.

## Step 6: Log In

Navigate to the username and password fields with your remote:

- **Username:** `admin`
- **Password:** `<ADMIN_PASSWORD>`

Press **D-pad Center** on the Sign In button.

## Step 7: Browse and Play Media

Use the D-pad to navigate horizontally through media rails and vertically between categories. Press **D-pad Center** to select an item and start playback.

Playback uses Media3 ExoPlayer with TV media session support -- control play/pause directly from your remote.

## ADB Command Reference

```bash
adb connect DEVICE_IP:5555                              # Connect to TV
adb install app/build/outputs/apk/debug/app-debug.apk  # Install APK
adb reverse tcp:8080 tcp:8080                           # Reverse proxy for local dev
adb devices                                             # Check connected devices
```

## Tips

- **D-pad navigation:** All UI elements are focusable with visible focus indicators. No touch needed.
- **Leanback UI:** Uses Compose for TV (`tv-foundation`, `tv-material`) optimized for 10-foot experience.
- **Home screen:** Integrates with TV Provider API for home screen recommendations.
- **Offline cache:** Room database caches media metadata for offline browsing.

## What's Next

- [Android TV Guide](ANDROID_TV_GUIDE.md) -- Full feature walkthrough
- [Android Architecture](../architecture/ANDROID_ARCHITECTURE.md) -- MVVM architecture details
- [Mobile Setup Tutorial](../tutorials/MOBILE_SETUP.md) -- Network and server configuration
- [Troubleshooting](TROUBLESHOOTING.md) -- Common issues and solutions
