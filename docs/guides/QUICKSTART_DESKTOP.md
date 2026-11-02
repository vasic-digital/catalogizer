# Catalogizer Desktop -- 5-Minute Quickstart

Install the Catalogizer desktop app and connect to your media server.

## Prerequisites

- Windows 10+, macOS 12+, or Linux (with AppImage or .deb support)
- catalog-api server running (locally or on your network)

## Step 1: Download or Build

### Pre-built Installers

| Platform | Format | File |
|----------|--------|------|
| Windows | MSI installer | `catalogizer_x.x.x_x64.msi` |
| macOS | DMG disk image | `catalogizer_x.x.x_x64.dmg` |
| Linux | AppImage | `catalogizer_x.x.x_x64.AppImage` |
| Linux | Debian package | `catalogizer_x.x.x_amd64.deb` |

### Build from Source

```bash
cd catalogizer-desktop
npm install
npm run tauri:build
```

The binary is generated in `src-tauri/target/release/`.

## Step 2: Install

- **Windows:** Run the `.msi` installer and follow the prompts.
- **macOS:** Open the `.dmg`, drag Catalogizer to Applications. Allow it in System Preferences > Security & Privacy on first launch.
- **Linux (AppImage):** Make it executable and run:
  ```bash
  chmod +x Catalogizer.AppImage
  ./Catalogizer.AppImage
  ```
- **Linux (Debian):**
  ```bash
  sudo dpkg -i catalogizer_*.deb
  ```

## Step 3: Configure the Server

On first launch, the app redirects you to **Settings** to configure your server.

1. Enter the **Server URL** (e.g., `http://localhost:8080` or `http://192.168.0.100:8080`).
2. Click **Test** to verify the connection.
3. Click **Save Settings**.

## Step 4: Log In

Enter your credentials:

- **Username:** `admin`
- **Password:** `<your-admin-password>`

Click **Sign In**. Your auth token is stored securely via Tauri's native storage.

## Step 5: Browse Your Library

The Home page shows an overview of your media collection. Use the sidebar to navigate:

- **Library** -- Browse all media by type
- **Search** -- Find media by title or keyword
- **Settings** -- Configure server, theme, and auto-start

Click any media item to see its details, metadata, and available files.

## Step 6: Use the Installer Wizard (Optional)

For guided storage source configuration, run the Installer Wizard:

```bash
cd installer-wizard
npm install
npm run tauri:dev
```

The wizard walks you through:

1. Network scanning for available servers
2. Protocol selection (SMB, FTP, NFS, WebDAV, Local)
3. Connection testing and credential entry
4. Configuration export as JSON

## Tips

- **Window size:** Default is 1200x800, fully resizable.
- **Theme:** Toggle dark/light mode in Settings.
- **Auto-start:** Enable auto-start in Settings to launch Catalogizer at login.
- **Secure storage:** Server URL, auth token, and preferences are stored in your OS application data directory via Tauri IPC.
- **SSRF protection:** The HTTP proxy validates that all requests target the configured server URL.

## What's Next

- [Desktop Guide](DESKTOP_GUIDE.md) -- Full feature walkthrough
- [Tauri IPC Guide](../architecture/TAURI_IPC_GUIDE.md) -- IPC commands and Rust backend details
- [Installer Wizard Guide](INSTALLER_WIZARD_GUIDE.md) -- Storage configuration wizard
- [Troubleshooting](TROUBLESHOOTING.md) -- Common issues and solutions
