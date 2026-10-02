# Quick Start Guide

Get Catalogizer up and running in 10 minutes using Docker.

## Prerequisites

- Docker and Docker Compose installed
- At least 4 GB RAM available
- Network access to your media storage (SMB, FTP, NFS, WebDAV, or local)

## Step 1: Clone the Repository

```bash
git clone <repository-url>
cd Catalogizer
```

**Expected result:** The Catalogizer directory is created with all project files.

## Step 2: Configure Environment Variables

```bash
cp .env.example .env
```

Edit `.env` with your settings. At minimum, set:

```env
# Required
POSTGRES_PASSWORD=your_secure_password
JWT_SECRET=your_jwt_secret_at_least_32_characters_long

# Optional but recommended
GRAFANA_PASSWORD=your_grafana_password
```

**Expected result:** A `.env` file exists in the project root with your configuration values.

## Step 3: Start Services with Docker Compose

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
podman compose up -d
```

This starts the following services:
- **PostgreSQL** database on port 5432
- **Redis** cache on port 6379
- **Catalogizer API** on port 8080

**Expected result:** All three containers start and reach healthy status within 60 seconds.

Verify with:

```bash
podman compose ps
```

You should see all services listed as "running" with health status "healthy".

## Step 4: Verify the API Is Running

```bash
curl http://localhost:8080/health
```

**Expected result:** A JSON response indicating the server is healthy:

```json
{"status": "ok"}
```

## Step 5: Create an Admin User

Access the API to register the first admin user:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "your_secure_password",
    "email": "admin@example.com"
  }'
```

**Expected result:** A JSON response with user details and a JWT token.

Save the returned token for subsequent API calls.

## Step 6: Connect Your First Storage Source

Using the token from Step 5, add a storage source. This example uses a local filesystem path:

```bash
curl -X POST http://localhost:8080/api/v1/sources \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-token>" \
  -d '{
    "name": "My Media",
    "protocol": "local",
    "settings": {
      "path": "/media"
    },
    "enabled": true
  }'
```

For an SMB share:

```bash
curl -X POST http://localhost:8080/api/v1/sources \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-token>" \
  -d '{
    "name": "NAS Media",
    "protocol": "smb",
    "settings": {
      "host": "192.168.1.100",
      "share": "media",
      "username": "media_user",
      "password": "share_password"
    },
    "enabled": true
  }'
```

**Expected result:** A JSON response confirming the source was created, including its unique ID.

## Step 7: Browse Your Catalog

Once the source is connected and scanning completes, browse your media:

```bash
# List all media items
curl -H "Authorization: Bearer <your-token>" \
  http://localhost:8080/api/v1/media

# Search for specific media
curl -H "Authorization: Bearer <your-token>" \
  "http://localhost:8080/api/v1/media/search?q=matrix"
```

**Expected result:** JSON arrays of detected media items with metadata such as title, type, quality, and file path.

## Step 8: (Optional) Start the Web Frontend

If you want the web interface:

```bash
cd catalog-web
npm install
npm run dev
```

Access the web UI at http://localhost:5173 (Vite dev server) or http://localhost:3000 depending on your configuration.

**Expected result:** The Catalogizer web interface loads in your browser with a login screen. Use the credentials from Step 5 to log in.

## Next Steps

- [Connect an SMB Share](CONNECT_SMB_SHARE.md) for detailed network storage setup
- [Set Up Monitoring](SETUP_MONITORING.md) to enable Prometheus and Grafana dashboards
- [Mobile Setup](MOBILE_SETUP.md) to install the Android app
- [Subtitle Management](SUBTITLE_MANAGEMENT.md) to search and manage subtitles

## Troubleshooting

### Docker containers fail to start

Check logs for the failing service:

```bash
podman compose logs api
podman compose logs postgres
podman compose logs redis
```

Common causes:
- Port conflicts: Another service is using port 5432, 6379, or 8080
- Missing `.env` values: `POSTGRES_PASSWORD` and `JWT_SECRET` are required
- Insufficient memory: Ensure at least 4 GB RAM is available

### API returns connection refused

Wait 30-40 seconds after starting containers. The API has a `start_period` of 40 seconds in its health check. Verify with:

```bash
podman compose ps
```

If the API container keeps restarting, check the logs:

```bash
podman compose logs -f api
```

### Cannot connect to storage source

- Verify network connectivity to the storage host from the Docker container
- For SMB, ensure ports 139 and 445 are accessible
- For FTP, ensure port 21 (and passive ports) are accessible
- For NFS, ensure the NFS service is running and exports are configured
- Check credentials are correct
