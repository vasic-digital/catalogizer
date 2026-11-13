# Catalogizer Installer Wizard Guide

This guide covers the Catalogizer Installer Wizard, a Tauri-based desktop application that walks you through configuring storage sources for your Catalogizer server. The wizard generates a configuration file that defines how Catalogizer connects to your media storage.

## Table of Contents

1. [Overview](#overview)
2. [Installation](#installation)
3. [Step 1: Welcome](#step-1-welcome)
4. [Step 2: Protocol Selection](#step-2-protocol-selection)
5. [Step 3: Protocol Configuration](#step-3-protocol-configuration)
6. [Step 4: Network Scan (Optional)](#step-4-network-scan-optional)
7. [Step 5: Configuration Management](#step-5-configuration-management)
8. [Step 6: Summary](#step-6-summary)
9. [Testing Connections](#testing-connections)
10. [Post-Wizard Steps](#post-wizard-steps)
11. [Troubleshooting](#troubleshooting)

---

## Overview

The Installer Wizard is designed for first-time setup or when you need to add new storage sources to your Catalogizer installation. It produces a configuration file that tells the Catalogizer server where to find and how to access your media files.

### What You Will Need

Before starting the wizard, gather the following information:

- Access credentials for your storage systems (username, password, domain if applicable)
- Network addresses or hostnames of your storage servers
- Share names or paths where your media files are stored
- The location where you want to save the generated configuration file

### Supported Protocols

The wizard supports five storage protocols:

| Protocol | Use Case | Auth Required |
|----------|----------|---------------|
| SMB/CIFS | Windows file shares, NAS devices | Yes (username/password, optional domain) |
| FTP | Remote file servers, hosted media | Yes (username/password) |
| NFS | Unix/Linux file shares | No (host-based access control) |
| WebDAV | Web-based file access, cloud storage | Yes (username/password) |
| Local | Files on the same machine as the server | No |

---

## Installation

### From Pre-Built Binaries

Download the installer wizard binary for your platform from the Catalogizer releases page:

- **Windows**: `.msi` or `.exe` installer
- **macOS**: `.dmg` disk image
- **Linux**: `.AppImage` or `.deb` package

### Building from Source

```bash
cd installer-wizard
npm install
npm run tauri:build
```

---

## Step 1: Welcome

When you launch the wizard, you see the Welcome screen.

### What This Step Shows

- A brief introduction to the wizard's purpose
- Three info cards explaining the process:
  1. **Protocol Selection** -- choose your storage protocol
  2. **Source Configuration** -- configure connection details and paths
  3. **Configuration** -- generate and manage your configuration file
- A checklist of what you will need:
  - Access to your storage system
  - Valid credentials
  - A location to save your configuration file

### Navigation

Click **Next** to proceed to Protocol Selection.

---

## Step 2: Protocol Selection

This step presents the available storage protocols.

### Protocol Options

Each protocol is displayed as a selectable card with a description and feature list:

**SMB/CIFS**
- Windows file sharing protocol for network drives
- Features: Network discovery, Share browsing, Authentication, Domain support

**FTP**
- File Transfer Protocol for remote file access
- Features: Username/password auth, Passive/Active modes, Path specification, Port configuration

**NFS**
- Network File System for Unix/Linux file sharing
- Features: Mount point configuration, Version specification, Options support, Host-based access

**WebDAV**
- Web-based Distributed Authoring and Versioning
- Features: HTTP/HTTPS support, Username/password auth, Path specification, SSL/TLS encryption

**Local Files**
- Direct access to local filesystem paths
- Features: Base path configuration, No authentication, Fast access, Full permissions

### How to Select

1. Click on the card for the protocol you want to configure.
2. The selected card is highlighted with a blue border and background.
3. A confirmation message appears at the bottom showing your selection.
4. Click **Next** to proceed to the protocol-specific configuration step.

You can click **Previous** to go back to the Welcome step.

---

## Step 3: Protocol Configuration

Based on your protocol selection in Step 2, you are taken to a configuration form specific to that protocol.

### SMB/CIFS Configuration

The SMB configuration form collects:

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| Name | Yes | -- | A friendly name for this storage source |
| Host | Yes | -- | Hostname or IP address of the SMB server |
| Port | Yes | 445 | SMB port (usually 445) |
| Share Name | Yes | -- | The name of the shared folder |
| Username | Yes | -- | SMB authentication username |
| Password | Yes | -- | SMB authentication password |
| Domain | No | -- | Windows domain (for domain-joined servers) |
| Path | No | -- | Subdirectory within the share |
| Enabled | Yes | true | Whether this source is active |

**Validation**: All required fields are validated using Zod schema validation. Error messages appear next to any field with invalid input.

**Test Connection**: After filling in the fields, click the **Test Connection** button to verify that the server is reachable and your credentials are valid. Results are displayed as a success or error message.

**Multiple Sources**: You can add multiple SMB sources by filling out the form and clicking **Add**. Each added source appears in a list below the form, where it can be edited or deleted.

### FTP Configuration

| Field | Required | Description |
|-------|----------|-------------|
| Host | Yes | FTP server hostname or IP |
| Port | Yes | FTP port (default: 21) |
| Username | Yes | FTP login username |
| Password | Yes | FTP login password |
| Path | No | Remote directory path |
| Passive Mode | No | Use passive FTP mode (recommended for firewalled networks) |

### NFS Configuration

| Field | Required | Description |
|-------|----------|-------------|
| Host | Yes | NFS server hostname or IP |
| Export Path | Yes | The exported directory (e.g. `/exports/media`) |
| Mount Options | No | NFS mount options (e.g. `vers=4,nolock`) |
| NFS Version | No | NFS protocol version (3 or 4) |

### WebDAV Configuration

| Field | Required | Description |
|-------|----------|-------------|
| URL | Yes | Full WebDAV URL (e.g. `https://server.com/webdav/media`) |
| Username | Yes | WebDAV authentication username |
| Password | Yes | WebDAV authentication password |
| Use SSL | No | Whether to use HTTPS (recommended) |

### Local Files Configuration

| Field | Required | Description |
|-------|----------|-------------|
| Base Path | Yes | Absolute path to the local directory containing media (e.g. `/mnt/media` or `C:\Media`) |

### Navigation

- Click **Previous** to go back to Protocol Selection.
- Click **Next** to proceed to Configuration Management (after adding at least one source).

---

## Step 4: Network Scan (Optional)

The Network Scan step is available when configuring SMB sources and helps you discover available servers on your local network.

### How to Use

1. Click **Scan Network** to start scanning.
2. The wizard searches for devices on your local network that have SMB services available.
3. Discovered hosts appear in a list, each showing:
   - Hostname or IP address
   - Device type icon
   - Available status
4. Click on a host to select it (you can select multiple hosts).
5. Selected hosts are highlighted and added to your configuration context.

### Scan Status

- A spinner animation appears while scanning is in progress.
- If no hosts are found, a message informs you that the network scan returned no results.
- If the scan fails, an error message with a retry option is displayed.

### Proceeding

- You can proceed with selected hosts or skip this step if you prefer to configure hosts manually.
- Selected hosts auto-populate the Host field in the SMB Configuration step.

---

## Step 5: Configuration Management

This step allows you to review and manage all the storage sources you have configured.

### Features

- View all added sources in a list
- Edit existing source configurations
- Delete unwanted sources
- Add additional sources (returns to Protocol Selection)
- Review access credentials

### Saving the Configuration

Click **Save Configuration** to generate the configuration file. The wizard uses Tauri's native file dialog to let you choose where to save the file.

The generated configuration file includes:

- **accesses** -- credential definitions (username, password, domain) for each authentication scope
- **sources** -- storage source definitions with protocol type, URL, and access reference

---

## Step 6: Summary

The final step confirms that the wizard has completed successfully and shows a summary of your configuration.

### Summary Statistics

Four cards display:

- **Access Credentials** -- number of unique credential sets configured
- **Media Sources** -- total number of storage sources
- **SMB Sources** -- number specifically using the SMB protocol
- **Unique Hosts** -- number of distinct server hostnames

### Configured Sources List

A detailed list of every configured source, showing:

- Protocol type (e.g. SAMBA)
- Full URL (e.g. `smb://server/share/path`)
- Associated access credential name
- Status indicator (Configured)

### Next Steps

The wizard outlines what to do after completing configuration:

1. **Deploy your configuration** -- copy the saved configuration file to your Catalogizer server installation directory.
2. **Start Catalogizer server** -- launch the server with your new configuration.
3. **Access the web interface** -- open the Catalogizer web app to manage your media.
4. **Monitor and enjoy** -- the server automatically discovers and catalogs media from your configured sources.

### Important Notes

- Ensure credentials are secure and follow your organization's security policies.
- Test in a development environment before production deployment.
- Keep the configuration file backed up and version controlled.
- Monitor connection logs for authentication or connectivity issues.
- Update credentials in the configuration file if passwords change.

### Actions

- **Start Over** -- resets the wizard to begin fresh
- **Save Configuration Again** -- re-saves the configuration file to a new location

---

## Testing Connections

Connection testing is available during protocol configuration (Step 3) for protocols that support it.

### How Connection Testing Works

1. Fill in the required configuration fields (host, port, credentials).
2. Click the **Test Connection** button.
3. The wizard sends a test request via the Tauri backend to attempt a connection.
4. Results appear below the button:
   - **Success**: green message confirming the connection was established
   - **Failure**: red message with details about what went wrong

### Common Test Results

| Result | Meaning | Action |
|--------|---------|--------|
| Connection successful | Server is reachable and credentials are valid | Proceed to add the source |
| Connection refused | Server is not listening on the specified port | Verify host and port |
| Authentication failed | Username or password is incorrect | Check credentials |
| Host not found | DNS cannot resolve the hostname | Verify hostname spelling or use IP address |
| Connection timeout | Server did not respond in time | Check network connectivity and firewall rules |

### Tips for Successful Testing

- Use IP addresses instead of hostnames if DNS is unreliable.
- For SMB, ensure port 445 (or 139 for legacy NetBIOS) is not blocked by a firewall.
- For FTP behind NAT, enable passive mode.
- For WebDAV, ensure the URL includes the correct path and protocol (http vs https).

---

## Post-Wizard Steps

After completing the wizard and saving your configuration file:

### 1. Deploy the Configuration

Copy the configuration file to the Catalogizer server's expected location. By default, the server looks for configuration in:

- Environment variable: `CATALOG_CONFIG_PATH`
- Default path: `./config.json` in the server's working directory

### 2. Start the Server

```bash
cd catalog-api
go run main.go
```

Or via Docker:

```bash
docker-compose -f docker-compose.dev.yml up
```

### 3. Verify Sources Are Connected

1. Open the web interface.
2. Navigate to the Admin panel.
3. Check the Storage section to confirm all sources show a "Connected" status.

### 4. Trigger a Media Scan

From the Dashboard, click **Scan Library** to begin discovering media from your configured sources.

---

## Troubleshooting

### Wizard Fails to Open

- Ensure you have the correct binary for your operating system.
- On macOS, allow the app in System Preferences > Security & Privacy.
- On Linux, ensure the AppImage has execute permissions.

### Network Scan Finds No Hosts

- Ensure your computer is on the same network as the storage servers.
- Some networks block SMB discovery (multicast traffic). Try entering hosts manually.
- Firewall rules may prevent broadcast/multicast packets needed for discovery.

### Test Connection Fails with "Connection Refused"

- Verify the server is running and the SMB/FTP/NFS/WebDAV service is started.
- Check the port number is correct.
- Verify firewall rules allow inbound connections on the service port.

### Test Connection Fails with "Authentication Failed"

- Double-check your username and password.
- For SMB: ensure you are using the correct domain (or leave it empty for workgroup environments).
- Some servers require a specific username format (e.g. `DOMAIN\username` or `username@domain`).

### Configuration File Not Recognized by Server

- Ensure the file is valid JSON format.
- Verify the file path is correct in the server's environment or configuration.
- Check server logs for parsing errors.

### Cannot Save Configuration File

- Ensure you have write permissions to the selected directory.
- On some operating systems, certain directories require elevated permissions.
- Try saving to your home directory or desktop first.
