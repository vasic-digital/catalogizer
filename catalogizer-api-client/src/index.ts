import { EventEmitter } from 'events';
import { HttpClient } from './utils/http';
import { WebSocketClient } from './utils/websocket';
import { AuthService } from './services/AuthService';
import { MediaService } from './services/MediaService';
import { SMBService } from './services/SMBService';
import {
  ClientConfig,
  ClientEvents,
  User,
  LoginRequest,
  LoginResponse,
} from './types';

export class CatalogizerClient extends EventEmitter {
  private http: HttpClient;
  private ws?: WebSocketClient;
  private config: ClientConfig;

  // Service instances
  public readonly auth: AuthService;
  public readonly media: MediaService;
  public readonly smb: SMBService;

  constructor(config: ClientConfig) {
    super();

    this.config = config;
    this.http = new HttpClient(config);

    // Initialize services
    this.auth = new AuthService(this.http);
    this.media = new MediaService(this.http);
    this.smb = new SMBService(this.http);

    // Set up HTTP client callbacks
    this.http.onTokenRefresh = this.handleTokenRefresh.bind(this);
    this.http.onAuthenticationError = this.handleAuthenticationError.bind(this);

    // Initialize WebSocket if enabled
    if (config.enableWebSocket) {
      this.initializeWebSocket();
    }
  }

  /**
   * Initialize WebSocket connection
   */
  private initializeWebSocket(): void {
    if (!this.config.webSocketURL) {
      console.warn('WebSocket enabled but no WebSocket URL provided');
      return;
    }

    this.ws = new WebSocketClient(this.config.webSocketURL, {
      // SEC-WS-1 F2: a fresh single-use ticket per connection attempt; the session token never goes in the URL.
      ticketProvider: async () => {
        const res = await this.http.post<{ ticket?: string }>('/ws/ticket', {});
        if (!res?.ticket) throw new Error('websocket ticket missing from response');
        return res.ticket;
      },
    });

    // Proxy WebSocket events
    this.ws.on('connection:open', () => {
      this.emit('connection:open');
    });

    this.ws.on('connection:close', () => {
      this.emit('connection:close');
    });

    this.ws.on('connection:error', (error: Error) => {
      this.emit('connection:error', error);
    });

    this.ws.on('download:progress', (progress) => {
      this.emit('download:progress', progress);
    });

    this.ws.on('scan:progress', (progress) => {
      this.emit('scan:progress', progress);
    });
  }

  /**
   * Connect to the Catalogizer server
   */
  public async connect(credentials?: LoginRequest): Promise<LoginResponse | null> {
    try {
      // Test server connection
      await this.http.get('/health');

      // If credentials provided, login
      if (credentials) {
        const loginResponse = await this.auth.login(credentials);
        this.emit('auth:login', loginResponse.user);

        // Connect WebSocket with auth token (session_token is the
        // canonical name; `token` is a back-compat alias — see
        // LoginResponse type for context).
        const wsToken = loginResponse.session_token ?? loginResponse.token;
        if (this.ws && wsToken) {
          await this.ws.connect(wsToken);
        }

        return loginResponse;
      }

      // If no credentials, try to get current auth status
      try {
        const status = await this.auth.getStatus();
        if (status.authenticated && status.user) {
          this.emit('auth:login', status.user);

          // Connect WebSocket with existing token
          if (this.ws) {
            const token = this.http.getAuthToken();
            if (token) {
              await this.ws.connect(token);
            }
          }
        }
      } catch (error) {
        // Not authenticated, that's okay
      }

      return null;
    } catch (error) {
      throw new Error(`Failed to connect to Catalogizer server: ${error}`);
    }
  }

  /**
   * Disconnect from the server
   */
  public async disconnect(): Promise<void> {
    try {
      await this.auth.logout();
    } catch (error) {
      // Ignore logout errors
    }

    if (this.ws) {
      this.ws.disconnect();
    }

    this.emit('auth:logout');
  }

  /**
   * Check if client is connected and authenticated
   */
  public async isAuthenticated(): Promise<boolean> {
    try {
      const status = await this.auth.getStatus();
      return status.authenticated;
    } catch (error) {
      return false;
    }
  }

  /**
   * Get current user information
   */
  public async getCurrentUser(): Promise<User | null> {
    try {
      return await this.auth.getProfile();
    } catch (error) {
      return null;
    }
  }

  /**
   * Set authentication token manually
   */
  public setAuthToken(token: string): void {
    this.http.setAuthToken(token);

    if (this.ws) {
      this.ws.setAuthToken(token);
    }
  }

  /**
   * Clear authentication token
   */
  public clearAuthToken(): void {
    this.http.clearAuthToken();
  }

  /**
   * Get current authentication token
   */
  public getAuthToken(): string | undefined {
    return this.http.getAuthToken();
  }

  /**
   * Update client configuration
   */
  public updateConfig(newConfig: Partial<ClientConfig>): void {
    this.config = { ...this.config, ...newConfig };
    this.http.updateConfig(newConfig);

    // Reinitialize WebSocket if configuration changed
    if (newConfig.enableWebSocket !== undefined || newConfig.webSocketURL) {
      if (this.ws) {
        this.ws.disconnect();
        this.ws = undefined;
      }

      if (this.config.enableWebSocket) {
        this.initializeWebSocket();
      }
    }
  }

  /**
   * Get current configuration
   */
  public getConfig(): ClientConfig {
    return { ...this.config };
  }

  /**
   * Check server health
   */
  public async healthCheck(): Promise<{
    status: string;
    version: string;
    timestamp: string;
  }> {
    return this.http.get<{
      status: string;
      version: string;
      timestamp: string;
    }>('/health');
  }

  /**
   * Get server information
   */
  public async getServerInfo(): Promise<{
    name: string;
    version: string;
    description: string;
    features: string[];
  }> {
    return this.http.get<{
      name: string;
      version: string;
      description: string;
      features: string[];
    }>('/info');
  }

  /**
   * Handle token refresh
   */
  private async handleTokenRefresh(): Promise<string | null> {
    try {
      const response = await this.auth.refreshToken();
      this.emit('auth:token_refresh', response.token);
      return response.token;
    } catch (error) {
      return null;
    }
  }

  /**
   * Handle authentication errors
   */
  private handleAuthenticationError(): void {
    this.emit('auth:logout');
  }

  // Typed event methods
  public on<K extends keyof ClientEvents>(event: K, listener: ClientEvents[K]): this;
  public on(event: string | symbol, listener: (...args: any[]) => void): this;
  public on(event: any, listener: any): this {
    return super.on(event, listener);
  }

  public off<K extends keyof ClientEvents>(event: K, listener: ClientEvents[K]): this;
  public off(event: string | symbol, listener: (...args: any[]) => void): this;
  public off(event: any, listener: any): this {
    return super.off(event, listener);
  }

  public emit<K extends keyof ClientEvents>(event: K, ...args: Parameters<ClientEvents[K]>): boolean;
  public emit(event: string | symbol, ...args: any[]): boolean;
  public emit(event: any, ...args: any[]): boolean {
    return super.emit(event, ...args);
  }
}

// Export types and classes
export * from './types';
export { HttpClient } from './utils/http';
export { WebSocketClient } from './utils/websocket';
export { AuthService } from './services/AuthService';
export { MediaService } from './services/MediaService';
export { SMBService } from './services/SMBService';

// Default export
export default CatalogizerClient;