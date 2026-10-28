# Node floor: images.lock.yaml IMG-NODE (20); override with --build-arg NODE_IMAGE=docker.io/library/node:22-alpine (also verified)
ARG NODE_IMAGE=docker.io/library/node:20-alpine
FROM ${NODE_IMAGE} AS builder

WORKDIR /app

# Copy submodule package files first
COPY submodules/websocket_client_ts/package*.json /app/submodules/websocket_client_ts/
COPY submodules/media_types_ts/package*.json /app/submodules/media_types_ts/
COPY submodules/ui_components_react/package*.json /app/submodules/ui_components_react/
COPY submodules/media_browser_react/package*.json /app/submodules/media_browser_react/
COPY submodules/media_player_react/package*.json /app/submodules/media_player_react/
COPY submodules/collection_manager_react/package*.json /app/submodules/collection_manager_react/
COPY submodules/dashboard_analytics_react/package*.json /app/submodules/dashboard_analytics_react/
COPY submodules/auth_context_react/package*.json /app/submodules/auth_context_react/
COPY submodules/catalogizer_api_client_ts/package*.json /app/submodules/catalogizer_api_client_ts/

# Copy main app package files
COPY catalog-web/package*.json /app/catalog-web/

# Install dependencies for submodules (only package files are present at this point; the builds follow the source copy below)
WORKDIR /app/submodules/media_types_ts
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/websocket_client_ts
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/catalogizer_api_client_ts
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/auth_context_react
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/ui_components_react
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/media_browser_react
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/media_player_react
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/collection_manager_react
RUN npm ci --legacy-peer-deps

WORKDIR /app/submodules/dashboard_analytics_react
RUN npm ci --legacy-peer-deps

# Copy submodule source
COPY submodules/websocket_client_ts/ /app/submodules/websocket_client_ts/
COPY submodules/media_types_ts/ /app/submodules/media_types_ts/
COPY submodules/ui_components_react/ /app/submodules/ui_components_react/
COPY submodules/media_browser_react/ /app/submodules/media_browser_react/
COPY submodules/media_player_react/ /app/submodules/media_player_react/
COPY submodules/collection_manager_react/ /app/submodules/collection_manager_react/
COPY submodules/dashboard_analytics_react/ /app/submodules/dashboard_analytics_react/
COPY submodules/auth_context_react/ /app/submodules/auth_context_react/
COPY submodules/catalogizer_api_client_ts/ /app/submodules/catalogizer_api_client_ts/

# Build the modules from their sources, in dependency order (media_types_ts first); a failing module build fails the image
RUN cd /app/submodules/media_types_ts && npm run build
RUN cd /app/submodules/websocket_client_ts && npm run build
RUN cd /app/submodules/catalogizer_api_client_ts && npm run build
RUN cd /app/submodules/auth_context_react && npm run build
RUN cd /app/submodules/ui_components_react && npm run build
RUN cd /app/submodules/media_browser_react && npm run build
RUN cd /app/submodules/media_player_react && npm run build
RUN cd /app/submodules/collection_manager_react && npm run build
RUN cd /app/submodules/dashboard_analytics_react && npm run build

# Build main app
COPY catalog-web/ /app/catalog-web/
WORKDIR /app/catalog-web
RUN npm ci --legacy-peer-deps
# No masking fallback: a type or build error MUST fail the image build (no `|| ... --skipLibCheck`).
RUN npm run build

FROM docker.io/nginx:alpine

# Copy built assets and nginx config with proper ownership
COPY --from=builder --chown=nginx:nginx /app/catalog-web/dist /usr/share/nginx/html
COPY --chown=nginx:nginx catalog-web/nginx.conf /etc/nginx/conf.d/default.conf

# Create required directories with nginx user ownership for non-root operation
RUN chown -R nginx:nginx /var/cache/nginx /var/log/nginx && \
    touch /var/run/nginx.pid && chown nginx:nginx /var/run/nginx.pid

EXPOSE 3000

# Health check: nginx serves the SPA on 3000 (127.0.0.1: the server block listens on IPv4 only)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:3000/ || exit 1

# Switch to non-root user
USER nginx

CMD ["nginx", "-g", "daemon off;"]
