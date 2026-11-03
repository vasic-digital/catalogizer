import api from './api'
import { asList } from './apiShapes'

export interface SmbShare {
  name: string
  path: string
  host: string
  comment?: string
}

export interface SmbTestResult {
  success: boolean
  message: string
  latency_ms?: number
}

export interface SmbBrowseEntry {
  name: string
  path: string
  is_directory: boolean
  size: number
  modified_at?: string
}

export interface SmbTestRequest {
  host: string
  share: string
  username?: string
  password?: string
  domain?: string
}

export interface SmbBrowseRequest {
  host: string
  share: string
  path?: string
  username?: string
  password?: string
  domain?: string
}

export const smbApi = {
  discover: (): Promise<SmbShare[]> =>
    api.post('/smb/discover').then((res) => asList<SmbShare>(res.data, 'shares')),

  discoverGet: (): Promise<SmbShare[]> =>
    api.get('/smb/discover').then((res) => asList<SmbShare>(res.data, 'shares')),

  testConnection: (body: SmbTestRequest): Promise<SmbTestResult> =>
    api.post('/smb/test', body).then((res) => res.data),

  browse: (body: SmbBrowseRequest): Promise<SmbBrowseEntry[]> =>
    api.post('/smb/browse', body).then((res) => asList<SmbBrowseEntry>(res.data, 'entries')),
}
