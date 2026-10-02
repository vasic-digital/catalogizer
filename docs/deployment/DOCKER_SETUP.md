# Catalogizer Docker Setup

This guide explains how to run Catalogizer using Docker and Docker Compose for both development and production environments.

## Prerequisites

- Docker 20.10 or later
- Docker Compose 2.0 or later

## Development Setup

### Quick Start

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Update the `.env` file with your local configuration (defaults are fine for development).

3. Start the development environment:
> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

   ```bash
   podman compose -f docker-compose.dev.yml up
   ```

4. Access the services:
   - **API**: http://localhost:8080
   - **API Health Check**: http://localhost:8080/health
   - **PostgreSQL**: localhost:5432
   - **Redis**: localhost:6379

### With Management Tools

To start with pgAdmin and Redis Commander for database/cache management:

```bash
podman compose -f docker-compose.dev.yml --profile tools up
```

Access management tools:
- **pgAdmin**: http://localhost:5050 (admin@catalogizer.dev / admin)
- **Redis Commander**: http://localhost:8081

### Development Features

- **Hot Reloading**: The API container uses Air for automatic reloading on code changes
- **Volume Mounting**: Your local code is mounted into the container
- **Debug Logging**: LOG_LEVEL is set to debug by default
- **PostgreSQL + Redis**: Matches production environment

### Individual Services

Start only specific services:

```bash
# Only database services
podman compose -f docker-compose.dev.yml up postgres redis

# Only the API (requires databases to be running)
podman compose -f docker-compose.dev.yml up api
```

## Production Setup

### Environment Configuration

1. Copy and configure the environment file:
   ```bash
   cp .env.example .env
   ```

2. **IMPORTANT**: Update these values in `.env` for production:
   ```env
   APP_ENV=production
   LOG_LEVEL=info
   POSTGRES_PASSWORD=<strong-secure-password>
   JWT_SECRET=<strong-secure-secret>
   CORS_ENABLED=false
   ```

### Start Production Stack

```bash
podman compose up -d
```

### With Nginx Reverse Proxy

```bash
podman compose --profile production up -d
```

This starts all services including Nginx as a reverse proxy.

### Health Checks

Verify all services are healthy:

```bash
podman compose ps
```

All services should show "healthy" status.

## Database Migrations

### Running Migrations

Migrations are automatically applied on container startup. Migration files should be placed in:
```
catalog-api/database/migrations/
```

### Manual Migration

To run migrations manually:

```bash
podman compose exec api go run database/migrations.go
```

## Common Commands

### View Logs

```bash
# All services
podman compose logs -f

# Specific service
podman compose logs -f api
podman compose logs -f postgres
podman compose logs -f redis
```

### Restart Services

```bash
# All services
podman compose restart

# Specific service
podman compose restart api
```

### Stop and Clean Up

```bash
# Stop services (preserves data)
podman compose down

# Stop and remove volumes (deletes data)
podman compose down -v
```

### Database Backup

```bash
# Backup PostgreSQL
podman compose exec postgres pg_dump -U catalogizer catalogizer > backup.sql

# Restore PostgreSQL
podman compose exec -T postgres psql -U catalogizer catalogizer < backup.sql
```

### Redis Operations

```bash
# Connect to Redis CLI
podman compose exec redis redis-cli

# Flush Redis cache
podman compose exec redis redis-cli FLUSHALL
```

## Resource Limits

Production services have resource limits configured:

- **PostgreSQL**: 2 CPU, 2GB RAM
- **Redis**: 1 CPU, 512MB RAM
- **API**: 2 CPU, 2GB RAM
- **Nginx**: 0.5 CPU, 256MB RAM

Adjust these in `docker-compose.yml` under the `deploy.resources` section if needed.

## Troubleshooting

### API Can't Connect to Database

1. Check if PostgreSQL is healthy:
   ```bash
   podman compose ps postgres
   ```

2. Check database logs:
   ```bash
   podman compose logs postgres
   ```

3. Verify environment variables:
   ```bash
   podman compose exec api env | grep DATABASE
   ```

### Port Already in Use

If ports 5432, 6379, or 8080 are already in use, update the `.env` file:

```env
POSTGRES_PORT=5433
REDIS_PORT=6380
API_PORT=8081
```

### Reset Everything

To completely reset the development environment:

```bash
podman compose -f docker-compose.dev.yml down -v
podman compose -f docker-compose.dev.yml up --build
```

## Development vs Production

| Feature | Development | Production |
|---------|-------------|------------|
| Hot Reload | ✅ Yes (Air) | ❌ No |
| Debug Logs | ✅ Yes | ❌ No |
| Volume Mount | ✅ Code mounted | ❌ Built into image |
| Resource Limits | ❌ No | ✅ Yes |
| Health Checks | ✅ Basic | ✅ Comprehensive |
| Nginx | ❌ Optional | ✅ Recommended |
| Database Tools | ✅ Included | ❌ Not included |

## Security Notes

For production deployments:

1. **Always** use strong passwords and secrets
2. **Never** commit `.env` files to version control
3. Enable Redis authentication by uncommenting `requirepass` in `redis.conf`
4. Use SSL/TLS certificates with Nginx
5. Consider using Docker secrets for sensitive values
6. Regularly update base images for security patches

## Monitoring

Production deployments should include:

- Container monitoring (Prometheus + Grafana)
- Log aggregation (ELK stack or similar)
- Health check endpoints
- Automated backups

See the production deployment guide for more details.
