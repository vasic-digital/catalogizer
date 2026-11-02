# Catalogizer Android -- 5-Minute Quickstart

Install the Catalogizer Android app and connect to your media server.

## Prerequisites

- Android 8.0+ device (minSdk 26)
- catalog-api server accessible on your LAN
- ADB installed (for sideloading the APK)

## Step 1: Build or Obtain the APK

Build from source:

```bash
cd catalogizer-android
./gradlew assembleDebug
```

The APK is generated at `app/build/outputs/apk/debug/app-debug.apk`.

## Step 2: Install the APK

Connect your device via USB and run:

```bash
adb install app/build/outputs/apk/debug/app-debug.apk
```

## Step 3: Launch the App

Open **Catalogizer** from your app drawer. On first launch, the app tries to reach `http://localhost:8080`.

## Step 4: Configure the Server URL

If the app cannot auto-connect, you will see the login screen with a server URL input.

- **Physical device on LAN:** Enter your server's IP, e.g., `http://192.168.0.100:8080`
- **Android emulator:** Enter `http://10.0.2.2:8080`
- **Local dev with ADB reverse:** See the tip below

Alternatively, tap **Discover** to scan your local network for a running catalog-api instance.

The server URL is persisted via DataStore and remembered across app restarts.

## Step 5: Log In

Enter your credentials:

- **Username:** `admin`
- **Password:** `<ADMIN_PASSWORD>`

Tap **Sign In**.

## Step 6: Browse Media

After login you land on the home screen. Browse your media library, search by title, and tap any item for details and playback.

## Step 7: Enable Offline Sync

The app uses Room for local caching. Media metadata syncs automatically for offline browsing. Background sync runs via WorkManager.

## Tips

- **ADB reverse for local dev:** If catalog-api runs on your development machine, set up port forwarding so the phone can reach it at `localhost:8080`:
  ```bash
  adb reverse tcp:8080 tcp:8080
  ```
  Then use `http://localhost:8080` as the server URL in the app.

- **Network security:** Cleartext HTTP is only allowed to local networks (10.x, 192.168.x, 127.x) via `network_security_config.xml`. Use HTTPS for remote servers.

- **ExoPlayer playback:** Media playback uses Media3 ExoPlayer with full streaming support.

- **Image loading:** Cover art and thumbnails are loaded via Coil with disk caching.

## What's Next

- [Android Guide](ANDROID_GUIDE.md) -- Full feature walkthrough
- [Android Architecture](../architecture/ANDROID_ARCHITECTURE.md) -- MVVM architecture details
- [Mobile Setup Tutorial](../tutorials/MOBILE_SETUP.md) -- Network and server configuration
- [Troubleshooting](TROUBLESHOOTING.md) -- Common issues and solutions
