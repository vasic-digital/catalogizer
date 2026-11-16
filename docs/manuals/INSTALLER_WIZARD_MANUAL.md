# Catalogizer -- Installer Wizard Manual

## Table of Contents

1. [What the Installer Wizard Does](#what-the-installer-wizard-does)
2. [Launching the Wizard](#launching-the-wizard)
3. [Network Discovery](#network-discovery)
4. [Share Configuration](#share-configuration)
5. [Storage Root Setup](#storage-root-setup)
6. [Initial Scan Configuration](#initial-scan-configuration)
7. [User Account Creation](#user-account-creation)
8. [Verification and Finish](#verification-and-finish)
9. [Re-Running the Wizard](#re-running-the-wizard)
10. [Troubleshooting](#troubleshooting)

---

## What the Installer Wizard Does

The Catalogizer Installer Wizard is a dedicated Tauri application that guides you through the initial setup of a Catalogizer deployment. It handles the tasks that must be completed before the main desktop or web application can be used effectively:

- **Discovering NAS devices and network shares** on your local network.
- **Configuring SMB, NFS, and WebDAV shares** as media sources.
- **Creating storage roots** that tell Catalogizer where to find your media files.
- **Running an initial scan** to populate the media library.
- **Creating the first user account** (admin) so you can log in.

The wizard is intended to be run once during initial deployment. It communicates with the Catalogizer API server to perform all configuration changes. After the wizard completes, you use the desktop app or web interface for ongoing management.

---

## Launching the Wizard

### From the Installer Package

The wizard is distributed alongside the desktop app. On Windows, it is installed as a separate Start Menu entry: **Catalogizer Installer Wizard**. On macOS, it is a separate app in the Applications folder. On Linux, it is a separate AppImage or binary.

### From the Command Line

You can also launch the wizard from the command line:

```bash
# Linux AppImage
./catalogizer-installer-wizard.AppImage

# macOS
open /Applications/Catalogizer\ Installer\ Wizard.app

# Windows
"C:\Program Files\Catalogizer\installer-wizard.exe"
```

### Server Connection

On launch, the wizard asks for the Catalogizer API server URL. Enter the address and port of your running Catalogizer API server:

```
https://192.168.0.100:8080
```

The wizard validates the connection before proceeding. If the server is not reachable, check that the API container is running and that your machine is on the same network.

---

## Network Discovery

The first step of the wizard is network discovery. The wizard scans your local network to find NAS devices, file servers, and other machines that expose network shares.

### How Discovery Works

The discovery process uses multiple detection methods:

1. **mDNS/Bonjour** -- Discovers devices advertising SMB or NFS services via multicast DNS.
2. **NetBIOS name resolution** -- Finds Windows-style network shares using NetBIOS broadcasts.
3. **IP range scanning** -- Probes common SMB (port 445), NFS (port 2049), and WebDAV (port 80/443) ports on your local subnet.

Discovery typically completes within 10-30 seconds depending on network size and responsiveness.

### Discovery Results

Discovered devices are listed with:

| Column | Description |
|--------|-------------|
| **Name** | The hostname or NetBIOS name of the device |
| **IP Address** | The network address |
| **Type** | NAS, Server, PC, or Unknown |
| **Protocols** | Detected protocols (SMB, NFS, WebDAV) |
| **Status** | Reachable or Unreachable |

### Manual Entry

If a device is not discovered automatically (for example, it is on a different subnet or has discovery services disabled), click **Add Manually** and enter the IP address or hostname and the protocol to use.

---

## Share Configuration

After selecting a device, the wizard lists the available network shares on that device and guides you through configuring access credentials.

### SMB Shares

For SMB (Server Message Block) shares, commonly used by Windows file servers and most NAS devices:

1. The wizard lists all visible SMB shares on the selected device.
2. Select the share(s) you want to use as media sources.
3. Enter credentials if the share requires authentication:
   - **Username** -- The SMB user account.
   - **Password** -- The SMB password.
   - **Domain** (optional) -- The Windows domain, if applicable.
4. The wizard tests the connection by attempting to list the share's root directory.

### NFS Shares

For NFS (Network File System) shares, commonly used by Linux servers and some NAS devices:

1. The wizard queries the device for exported NFS paths.
2. Select the export(s) to mount.
3. NFS typically uses host-based authentication rather than username/password. Ensure the Catalogizer server's IP is in the NFS export's allowed hosts list.
4. The wizard tests access by listing the export root.

### WebDAV Shares

For WebDAV shares, used by some cloud storage appliances and NAS devices:

1. Enter the WebDAV URL (e.g., `https://nas.local:5006/webdav`).
2. Enter the username and password.
3. The wizard tests the connection by performing a PROPFIND request.

### FTP Shares

If the target device offers FTP access:

1. Enter the FTP host, port (default 21), username, and password.
2. The wizard validates the connection and lists the root directory.
3. FTPS (FTP over TLS) is used automatically if the server supports it.

### Connection Test

For every configured share, the wizard performs a connection test before proceeding. A green checkmark indicates success. A red indicator with an error message means the connection failed -- check credentials, network access, and that the share service is running on the target device.

---

## Storage Root Setup

After configuring shares, the wizard creates **storage roots** in the Catalogizer database. A storage root is a top-level directory path that Catalogizer monitors and scans for media files.

### What Is a Storage Root

A storage root tells Catalogizer: "This directory (and everything inside it) contains media that should be cataloged." Each storage root maps to one network share or local directory path.

### Creating Roots

For each configured share, the wizard proposes a storage root:

- **Name** -- A human-readable label (e.g., "NAS Movies", "NAS TV Shows"). The wizard suggests a name based on the device hostname and share name.
- **Path** -- The full path to the share as seen by the Catalogizer server (e.g., `smb://nas.local/media/movies`).
- **Protocol** -- Automatically set based on the share type (SMB, NFS, WebDAV, FTP, or local).
- **Enabled** -- Whether the root is active for scanning (enabled by default).

You can edit the name, adjust the path to point to a subdirectory, or disable a root to skip it during the initial scan.

### Local Directories

In addition to network shares, you can add local directories as storage roots. Click **Add Local Directory** and browse to a folder on the Catalogizer server's filesystem. This is useful for media stored on directly attached storage.

---

## Initial Scan Configuration

After creating storage roots, the wizard configures and optionally triggers the first media scan.

### Scan Options

| Option | Description | Default |
|--------|-------------|---------|
| **Scan on finish** | Start scanning immediately when the wizard completes | Enabled |
| **Scan depth** | Maximum directory depth to traverse | Unlimited |
| **File extensions** | Which file types to include in the scan | All recognized media types |
| **Exclude patterns** | Glob patterns for files or directories to skip (e.g., `*.nfo`, `Thumbs.db`, `.@__thumb/`) | Common junk patterns |

### Scan Behavior

The initial scan traverses every storage root, discovers media files, and feeds them into the aggregation pipeline. Depending on library size and network speed, this can take anywhere from a few seconds (small library on local storage) to several hours (large library over SMB on a slow network).

The wizard displays a progress indicator during the scan. You can close the wizard and let the scan continue in the background on the server -- progress is tracked server-side and visible from the web admin panel.

### Skipping the Initial Scan

If you prefer to configure the scan later (for example, to adjust exclusion patterns from the admin panel first), uncheck **Scan on finish**. You can trigger scans manually from the admin panel or the desktop app at any time.

---

## User Account Creation

The wizard creates the initial administrator account for the Catalogizer instance.

### Admin Account Setup

1. **Username** -- Choose a username for the admin account (default suggestion: `admin`).
2. **Password** -- Enter a strong password. The wizard enforces a minimum length of 8 characters.
3. **Confirm password** -- Re-enter the password to confirm.
4. **Email** (optional) -- An email address for account recovery.

This account receives the **admin** role, granting full access to all features including the admin panel, user management, storage configuration, and system settings.

### Additional Users

The wizard optionally allows creating additional user accounts with the **user** role. Click **Add Another User** to create accounts for family members or team members. These accounts can browse and play media but cannot access admin functions.

Additional users can also be created later from the admin panel.

---

## Verification and Finish

The final step of the wizard displays a summary of everything that was configured.

### Configuration Summary

The summary includes:

- **Server** -- The connected Catalogizer API server URL.
- **Storage roots** -- The number and names of created storage roots, with protocol and path details.
- **Shares** -- The network shares that were configured, with connection status.
- **User accounts** -- The accounts that were created (usernames only, passwords are not displayed).
- **Scan status** -- Whether an initial scan was triggered and its current progress.

### Verification Checks

The wizard runs a series of verification checks:

1. **Server health** -- Confirms the API server is responding.
2. **Storage root accessibility** -- Tests that each storage root can be reached from the server.
3. **Database integrity** -- Verifies that all configuration was written correctly to the database.
4. **Authentication** -- Confirms that the created admin account can log in successfully.

Each check displays a pass/fail status. If any check fails, the wizard provides an explanation and an option to retry or go back and fix the issue.

### Completion

When all checks pass, click **Finish** to close the wizard. The wizard displays direct links to:

- **Web application** -- Open the Catalogizer web interface in your browser.
- **Desktop application** -- Launch the Catalogizer desktop app (if installed).
- **Admin panel** -- Go directly to the admin panel to continue configuration.

---

## Re-Running the Wizard

The installer wizard can be run again at any time to add new storage roots, reconfigure shares, or create additional user accounts. Re-running the wizard does not overwrite existing configuration -- it adds to it.

### Common Reasons to Re-Run

- **New NAS device** -- You added a new NAS to your network and want to add its shares as storage roots.
- **Changed credentials** -- A share password was changed and you need to update the stored credentials.
- **Additional storage** -- You connected a new external drive to the server and want to add it as a local storage root.
- **New user accounts** -- You need to create accounts for new family members or team members.

### What Is Preserved

When re-running the wizard on an existing Catalogizer instance:

- Existing storage roots are not modified or deleted.
- Existing user accounts are not affected.
- The media library and all entity data are untouched.
- Previously scanned files are not re-scanned unless you explicitly trigger a new scan.

### Resetting Configuration

If you need to start from scratch, the wizard does not provide a reset function. Use the admin panel to delete storage roots and user accounts manually, or re-deploy the Catalogizer server with a fresh database.

---

## Troubleshooting

### No Devices Found During Discovery

- Ensure your machine is on the same local network (subnet) as the NAS devices.
- Some NAS devices disable mDNS or NetBIOS by default. Enable these services in the NAS admin panel, or use the **Add Manually** option.
- Firewalls on the NAS or on your machine may block discovery traffic. Temporarily disable the firewall for testing.
- VPN connections can interfere with local network discovery. Disconnect VPN if you are trying to discover devices on your physical LAN.

### Share Connection Fails

- Double-check the username and password. SMB credentials are case-sensitive on some NAS operating systems.
- Verify that the share service (SMB, NFS, or WebDAV) is enabled on the target device.
- For NFS shares, ensure the Catalogizer server's IP address is listed in the NFS export configuration on the NAS.
- For SMB on newer NAS firmware, SMBv1 may be disabled. Catalogizer uses SMBv2/v3 by default, but very old NAS devices may only support SMBv1.

### Storage Root Shows as Inaccessible

- The storage root path must be accessible from the Catalogizer API server, not from the machine running the wizard. If the wizard runs on a different machine than the server, the server must be able to reach the share.
- Check that the API container has the necessary network configuration. The container may need `--add-host` flags for hostname resolution.

### Initial Scan Takes Very Long

- Large libraries (100,000+ files) over SMB can take several hours for the initial scan. This is normal.
- Check server resource usage (`podman stats --no-stream`). The API container should not exceed its CPU and memory limits.
- Ensure the NAS is not overloaded by other tasks during the scan.
- You can close the wizard and monitor scan progress from the admin panel.

### Wizard Cannot Reach the Server

- Verify the server URL and port are correct.
- Ensure the Catalogizer API container is running: `podman ps | grep catalog-api`.
- If the server uses HTTP/3 (QUIC) and your network blocks UDP, the wizard falls back to HTTP/2 automatically. If the fallback also fails, check general network connectivity between the wizard machine and the server.
