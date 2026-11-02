# Catalogizer Web -- 5-Minute Quickstart

Get the Catalogizer web interface running and browse your first media collection.

## Prerequisites

- Modern browser (Chrome, Firefox, Edge, Safari)
- catalog-api backend running (see [Development Setup](DEVELOPMENT_SETUP.md))

## Step 1: Start the Backend

```bash
cd catalog-api
go run main.go
```

The server writes its port to `.service-port`. Default is `8080`.

## Step 2: Start the Frontend

```bash
cd catalog-web
npm install
npm run dev
```

The dev server starts on port 3000 and automatically proxies `/api` requests to the backend by reading `../catalog-api/.service-port`.

> **Note:** Kill any process on port 3000 first: `ss -tlnp | grep :3000`

## Step 3: Open the App

Navigate to **http://localhost:3000** in your browser.

## Step 4: Log In

Enter the default credentials:

- **Username:** `admin`
- **Password:** `<ADMIN_PASSWORD>`

Click **Sign In**. You will be redirected to the dashboard.

## Step 5: Add a Storage Root

1. Go to **Settings** from the sidebar.
2. Under **Storage Roots**, click **Add Storage Root**.
3. Choose a protocol (Local, SMB, FTP, NFS, or WebDAV).
4. Enter the path or connection details and save.

## Step 6: Start a Scan

1. Navigate to the storage root you just added.
2. Click **Scan** to begin media detection.
3. The scan runs in the background. Progress updates appear via WebSocket in real time.

## Step 7: Browse Your Media

Once the scan completes, go to **Browse** (`/browse`) to explore detected media entities -- movies, TV shows, music, games, and more. Click any item to see its details, metadata, and associated files.

## Tips

- **Dark mode:** Toggle the theme in Settings.
- **Responsive layout:** The UI works on tablets and phones as well as desktop browsers.
- **Real-time updates:** WebSocket connection delivers scan progress, new media events, and system notifications without page refresh.
- **Search:** Use the search bar at the top to find media by title, type, or year.
- **Collections:** Create custom collections to organize media across storage roots.

## What's Next

- [Web App Guide](WEB_APP_GUIDE.md) -- Full feature walkthrough
- [Configuration Reference](CONFIGURATION_REFERENCE.md) -- All settings and environment variables
- [Troubleshooting](TROUBLESHOOTING.md) -- Common issues and solutions
- [API Documentation](../api/API_DOCUMENTATION.md) -- REST API reference
