import { describe, it, expect } from 'vitest'
import type {
  NetworkHost,
  SMBShare,
  FileEntry,
  ConfigurationAccess,
  ConfigurationSource,
  Configuration,
  WizardState,
  SMBConnectionConfig,
  FTPConnectionConfig,
  NFSConnectionConfig,
  WebDAVConnectionConfig,
  LocalConnectionConfig,
  ValidationError,
  ApiResponse,
  StepProps,
  WizardStep,
  ConfigValidationResult,
} from '../index'

describe('Installer Wizard type definitions', () => {
  describe('NetworkHost', () => {
    it('represents a discovered network host', () => {
      const host: NetworkHost = {
        ip: '192.168.1.100',
        hostname: 'media-server',
        mac_address: '00:11:22:33:44:55',
        vendor: 'ExampleNAS',
        open_ports: [445, 139, 80],
        smb_shares: ['media', 'backups'],
      }

      expect(host.ip).toBe('192.168.1.100')
      expect(host.hostname).toBe('media-server')
      expect(host.open_ports).toContain(445)
      expect(host.smb_shares).toHaveLength(2)
    })

    it('works with minimal fields', () => {
      const host: NetworkHost = {
        ip: '10.0.0.1',
        open_ports: [],
        smb_shares: [],
      }

      expect(host.ip).toBe('10.0.0.1')
      expect(host.hostname).toBeUndefined()
      expect(host.mac_address).toBeUndefined()
      expect(host.vendor).toBeUndefined()
      expect(host.open_ports).toHaveLength(0)
    })
  })

  describe('SMBShare', () => {
    it('represents a discovered SMB share', () => {
      const share: SMBShare = {
        host: '192.168.1.100',
        share_name: 'media',
        path: '/volume1/media',
        writable: true,
        description: 'Media files share',
      }

      expect(share.host).toBe('192.168.1.100')
      expect(share.share_name).toBe('media')
      expect(share.writable).toBe(true)
    })

    it('works without description', () => {
      const share: SMBShare = {
        host: '192.168.1.100',
        share_name: 'public',
        path: '/public',
        writable: false,
      }

      expect(share.description).toBeUndefined()
      expect(share.writable).toBe(false)
    })
  })

  describe('FileEntry', () => {
    it('represents a file', () => {
      const file: FileEntry = {
        name: 'movie.mkv',
        path: '/media/movies/movie.mkv',
        is_directory: false,
        size: 4294967296,
        modified: '2023-06-15T12:00:00Z',
      }

      expect(file.name).toBe('movie.mkv')
      expect(file.is_directory).toBe(false)
      expect(file.size).toBe(4294967296)
    })

    it('represents a directory', () => {
      const dir: FileEntry = {
        name: 'movies',
        path: '/media/movies',
        is_directory: true,
      }

      expect(dir.is_directory).toBe(true)
      expect(dir.size).toBeUndefined()
      expect(dir.modified).toBeUndefined()
    })
  })

  describe('Configuration', () => {
    it('represents a complete configuration', () => {
      const config: Configuration = {
        accesses: [
          {
            name: 'nas_credentials',
            type: 'credentials',
            account: 'admin',
            secret: 'password123',
          },
        ],
        sources: [
          {
            type: 'samba',
            url: 'smb://192.168.1.100:445/media',
            access: 'nas_credentials',
          },
        ],
      }

      expect(config.accesses).toHaveLength(1)
      expect(config.sources).toHaveLength(1)
      expect(config.accesses[0].name).toBe('nas_credentials')
      expect(config.sources[0].type).toBe('samba')
    })

    it('represents an empty configuration', () => {
      const config: Configuration = {
        accesses: [],
        sources: [],
      }

      expect(config.accesses).toHaveLength(0)
      expect(config.sources).toHaveLength(0)
    })

    it('supports multiple accesses and sources', () => {
      const config: Configuration = {
        accesses: [
          { name: 'user1', type: 'credentials', account: 'u1', secret: 's1' },
          { name: 'user2', type: 'credentials', account: 'u2', secret: 's2' },
          { name: 'user3', type: 'credentials', account: 'u3', secret: 's3' },
        ],
        sources: [
          { type: 'samba', url: 'smb://host1/share', access: 'user1' },
          { type: 'ftp', url: 'ftp://host2:21/data', access: 'user2' },
          { type: 'samba', url: 'smb://host3/media', access: 'user3' },
        ],
      }

      expect(config.accesses).toHaveLength(3)
      expect(config.sources).toHaveLength(3)
    })
  })

  describe('WizardState', () => {
    it('represents initial wizard state', () => {
      const state: WizardState = {
        currentStep: 0,
        totalSteps: 5,
        canGoNext: true,
        canGoPrevious: false,
        isComplete: false,
      }

      expect(state.currentStep).toBe(0)
      expect(state.canGoPrevious).toBe(false)
      expect(state.isComplete).toBe(false)
    })

    it('represents final wizard state', () => {
      const state: WizardState = {
        currentStep: 4,
        totalSteps: 5,
        canGoNext: false,
        canGoPrevious: true,
        isComplete: true,
      }

      expect(state.currentStep).toBe(4)
      expect(state.isComplete).toBe(true)
    })

    it('represents middle wizard state', () => {
      const state: WizardState = {
        currentStep: 2,
        totalSteps: 5,
        canGoNext: true,
        canGoPrevious: true,
        isComplete: false,
      }

      expect(state.canGoNext).toBe(true)
      expect(state.canGoPrevious).toBe(true)
    })
  })

  describe('Protocol connection configs', () => {
    it('represents SMB connection config', () => {
      const config: SMBConnectionConfig = {
        name: 'Media Server',
        host: '192.168.1.100',
        port: 445,
        share_name: 'media',
        username: 'admin',
        password: 'secret',
        domain: 'WORKGROUP',
        path: '/movies',
        enabled: true,
      }

      expect(config.port).toBe(445)
      expect(config.domain).toBe('WORKGROUP')
      expect(config.enabled).toBe(true)
    })

    it('represents FTP connection config', () => {
      const config: FTPConnectionConfig = {
        name: 'FTP Server',
        host: '192.168.1.200',
        port: 21,
        username: 'ftpuser',
        password: 'ftppass',
        path: '/data',
        enabled: true,
      }

      expect(config.port).toBe(21)
      expect(config.path).toBe('/data')
    })

    it('represents NFS connection config', () => {
      const config: NFSConnectionConfig = {
        name: 'NFS Export',
        host: '192.168.1.150',
        path: '/export/media',
        mount_point: '/mnt/nfs-media',
        options: 'vers=3,nolock',
        enabled: true,
      }

      expect(config.mount_point).toBe('/mnt/nfs-media')
      expect(config.options).toBe('vers=3,nolock')
    })

    it('represents WebDAV connection config', () => {
      const config: WebDAVConnectionConfig = {
        name: 'WebDAV Server',
        url: 'https://webdav.example.com/files',
        username: 'webuser',
        password: 'webpass',
        path: '/media',
        enabled: true,
      }

      expect(config.url).toContain('https')
      expect(config.path).toBe('/media')
    })

    it('represents Local connection config', () => {
      const config: LocalConnectionConfig = {
        name: 'Local Media',
        base_path: '/home/user/media',
        enabled: true,
      }

      expect(config.base_path).toBe('/home/user/media')
      expect(config.enabled).toBe(true)
    })
  })

  describe('ValidationError', () => {
    it('represents a field validation error', () => {
      const error: ValidationError = {
        field: 'host',
        message: 'Host is required',
      }

      expect(error.field).toBe('host')
      expect(error.message).toBe('Host is required')
    })
  })

  describe('ApiResponse', () => {
    it('represents a successful response', () => {
      const response: ApiResponse<{ id: number }> = {
        success: true,
        data: { id: 1 },
      }

      expect(response.success).toBe(true)
      expect(response.data?.id).toBe(1)
    })

    it('represents an error response', () => {
      const response: ApiResponse<never> = {
        success: false,
        error: 'Connection failed',
        message: 'Unable to reach server',
      }

      expect(response.success).toBe(false)
      expect(response.error).toBe('Connection failed')
    })
  })

  describe('ConfigValidationResult', () => {
    it('represents a valid configuration', () => {
      const result: ConfigValidationResult = {
        isValid: true,
        errors: [],
        warnings: [],
      }

      expect(result.isValid).toBe(true)
      expect(result.errors).toHaveLength(0)
      expect(result.warnings).toHaveLength(0)
    })

    it('represents an invalid configuration with errors', () => {
      const result: ConfigValidationResult = {
        isValid: false,
        errors: [
          { field: 'host', message: 'Host is required' },
          { field: 'username', message: 'Username is required' },
        ],
        warnings: ['Password is weak'],
      }

      expect(result.isValid).toBe(false)
      expect(result.errors).toHaveLength(2)
      expect(result.warnings).toHaveLength(1)
      expect(result.errors[0].field).toBe('host')
      expect(result.warnings[0]).toBe('Password is weak')
    })
  })
})
