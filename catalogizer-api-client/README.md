# Catalogizer API Client

A comprehensive, cross-platform TypeScript/JavaScript client library for the Catalogizer media management system.

## Features

- 🔐 **Authentication**: Complete user authentication and session management
- 📺 **Media Management**: Search, stream, and manage your media library
- 🔗 **SMB Integration**: Manage and monitor SMB/CIFS network shares
- 🌐 **WebSocket Support**: Real-time updates for downloads and scanning
- 🔄 **Auto-retry**: Automatic retry logic with exponential backoff
- 📱 **Cross-platform**: Works in Node.js, browsers, React Native, and Electron
- 🎯 **TypeScript**: Full TypeScript support with comprehensive type definitions
- 🏗️ **Modular**: Use individual services or the complete client

## Installation

```bash
npm install @catalogizer/api-client
```

## Quick Start

```typescript
import CatalogizerClient from '@catalogizer/api-client';

// Create client instance
const client = new CatalogizerClient({
  baseURL: 'http://localhost:8080',
  enableWebSocket: true,
  webSocketURL: 'ws://localhost:8080/ws'
});

// Connect and authenticate
try {
  await client.connect({
    username: 'your_username',
    password: 'your_password'
  });

  console.log('Connected to Catalogizer!');
} catch (error) {
  console.error('Failed to connect:', error);
}
```

## Configuration

```typescript
interface ClientConfig {
  baseURL: string;                 // Required: API base URL
  timeout?: number;                // Request timeout (default: 30000ms)
  retryAttempts?: number;          // Max retry attempts (default: 3)
  retryDelay?: number;             // Retry delay (default: 1000ms)
  enableWebSocket?: boolean;       // Enable real-time updates
  webSocketURL?: string;           // WebSocket URL
  headers?: Record<string, string>; // Additional headers
}
```

## Authentication

### Login

```typescript
const loginResponse = await client.auth.login({
  username: 'user',
  password: 'password'
});

console.log('User:', loginResponse.user);
console.log('Token expires in:', loginResponse.expires_in, 'seconds');
```

### Check Authentication Status

```typescript
const status = await client.auth.getStatus();
if (status.authenticated) {
  console.log('User is authenticated:', status.user);
}
```

### User Registration

```typescript
const newUser = await client.auth.register({
  username: 'newuser',
  email: 'user@example.com',
  password: 'securepassword',
  first_name: 'John',
  last_name: 'Doe'
});
```

### Profile Management

```typescript
// Get user profile
const profile = await client.auth.getProfile();

// Update profile
const updatedProfile = await client.auth.updateProfile({
  first_name: 'Jane',
  email: 'jane@example.com'
});

// Change password
await client.auth.changePassword({
  current_password: 'oldpassword',
  new_password: 'newpassword'
});
```

## Media Management

### Search Media

```typescript
const searchResults = await client.media.search({
  query: 'action movies',
  media_type: 'movie',
  year_min: 2020,
  sort_by: 'rating',
  sort_order: 'desc',
  limit: 20
});

console.log(`Found ${searchResults.total} items`);
searchResults.items.forEach(item => {
  console.log(`${item.title} (${item.year})`);
});
```

### Get Media Details

```typescript
const media = await client.media.getById(123);
console.log('Title:', media.title);
console.log('Description:', media.description);
console.log('Rating:', media.rating);
```

### Playback and Progress

```typescript
// Get streaming URL
const streamInfo = await client.media.getStreamUrl(123);
console.log('Stream URL:', streamInfo.url);

// Update watch progress
await client.media.updateProgress(123, {
  media_id: 123,
  position: 1800, // 30 minutes
  duration: 7200, // 2 hours
  timestamp: Date.now()
});

// Mark as watched
await client.media.markAsWatched(123);
```

### Favorites and Recommendations

```typescript
// Toggle favorite
const result = await client.media.toggleFavorite(123);
console.log('Is favorite:', result.is_favorite);

// Get favorites
const favorites = await client.media.getFavorites();

// Get recommendations
const recommendations = await client.media.getRecommendations();
```

### Downloads

```typescript
// Queue download
const downloadJob = await client.media.queueDownload(123);
console.log('Download job ID:', downloadJob.id);

// Get download jobs
const jobs = await client.media.getDownloadJobs();

// Cancel download
await client.media.cancelDownload(downloadJob.id);
```

## SMB Management

### SMB Configuration

```typescript
// Create SMB config
const smbConfig = await client.smb.createConfig({
  name: 'My NAS',
  host: '192.168.1.100',
  port: 445,
  share_name: 'media',
  username: 'nas_user',
  password: 'nas_password',
  mount_point: '/mnt/nas'
});

// Test connection
const testResult = await client.smb.testConnection(smbConfig);
if (testResult.success) {
  console.log('SMB connection successful!');
}
```

### SMB Operations

```typescript
// Connect to SMB share
await client.smb.connect(smbConfig.id);

// Get connection status
const status = await client.smb.getConfigStatus(smbConfig.id);
console.log('Connected:', status.is_connected);

// Scan for media
const scanJob = await client.smb.scan(smbConfig.id, {
  deep_scan: true,
  update_metadata: true
});

console.log('Scan job started:', scanJob.job_id);
```

### Browse SMB Shares

```typescript
const contents = await client.smb.browse(smbConfig.id, '/movies');
console.log('Directories:', contents.directories);
console.log('Files:', contents.files);
```

## Real-time Updates

The client supports WebSocket connections for real-time updates.

**Authentication (SEC-WS-1 F2).** The session token is never placed in the `/ws` URL (URLs leak into proxy and
server logs). `CatalogizerClient` calls `POST /api/v1/ws/ticket` (token in the `Authorization` header) before every
connection attempt, reconnects included, and opens the socket with `?ticket=<single-use ticket>`. A ticket is valid
for 30 seconds and can be redeemed once, so a reconnect always mints a new one. A `401` from the ticket request is
final (no reconnect loop); other failures are retried with backoff. Servers with this change refuse `?token=`, so
the client must be upgraded together with the server. Using `WebSocketClient` directly:

```typescript
import { WebSocketClient } from '@catalogizer/api-client';

const ws = new WebSocketClient('wss://host/ws', {
  ticketProvider: async () => (await http.post('/ws/ticket', {})).ticket,
});
await ws.connect(sessionToken); // sessionToken only gates the ticket path; it is not sent in the URL
// Without a ticketProvider the token is sent as an `Authorization: Bearer` header (Node `ws`).
```

```typescript
// Listen for download progress
client.on('download:progress', (progress) => {
  console.log(`Download ${progress.job_id}: ${progress.progress}%`);
  if (progress.status === 'completed') {
    console.log('Download completed!');
  }
});

// Listen for scan progress
client.on('scan:progress', (progress) => {
  console.log(`Scan ${progress.config_id}: ${progress.progress}%`);
  console.log(`Found ${progress.found_items} items`);
});

// Listen for authentication events
client.on('auth:login', (user) => {
  console.log('User logged in:', user.username);
});

client.on('auth:logout', () => {
  console.log('User logged out');
});
```

## Error Handling

The client provides specific error types for better error handling:

```typescript
import {
  CatalogizerError,
  AuthenticationError,
  NetworkError,
  ValidationError
} from '@catalogizer/api-client';

try {
  await client.media.getById(999);
} catch (error) {
  if (error instanceof AuthenticationError) {
    console.log('Please log in again');
  } else if (error instanceof NetworkError) {
    console.log('Network connection failed');
  } else if (error instanceof ValidationError) {
    console.log('Invalid request data');
  } else if (error instanceof CatalogizerError) {
    console.log('API error:', error.message, 'Status:', error.status);
  }
}
```

## Advanced Usage

### Custom HTTP Configuration

```typescript
const client = new CatalogizerClient({
  baseURL: 'https://api.catalogizer.com',
  timeout: 60000,
  retryAttempts: 5,
  retryDelay: 2000,
  headers: {
    'X-Client-Version': '1.0.0'
  }
});
```

### Manual Token Management

```typescript
// Set token manually (e.g., from storage)
client.setAuthToken('your-stored-token');

// Get current token
const token = client.getAuthToken();

// Clear token
client.clearAuthToken();
```

### Using Individual Services

```typescript
import { HttpClient, MediaService } from '@catalogizer/api-client';

const http = new HttpClient({ baseURL: 'http://localhost:8080' });
const mediaService = new MediaService(http);

const media = await mediaService.search({ query: 'action' });
```

## Platform-Specific Notes

### React Native
```typescript
// Use appropriate WebSocket implementation
import WebSocket from 'ws'; // For Node.js
// or use built-in WebSocket for React Native
```

### Electron
```typescript
// Works out of the box with both main and renderer processes
```

### Browser
```typescript
// Use bundled version or import specific modules
```

## API Reference

For complete API documentation, see the TypeScript definitions included with the package.

## License

MIT License - see LICENSE file for details.