import api from './api'
import { asList } from './apiShapes'

export interface ScanJob {
  job_id: string
  storage_root_id: number
  storage_root_name?: string
  status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'
  progress: number
  files_found: number
  files_processed: number
  errors: number
  started_at?: string
  completed_at?: string
  error_message?: string
}

export interface QueueScanRequest {
  storage_root_id: number
  full_scan?: boolean
  max_depth?: number
}

export const scansApi = {
  listScans: (): Promise<ScanJob[]> =>
    api.get('/scans').then((res) => asList<ScanJob>(res.data, 'scans')),

  queueScan: (body: QueueScanRequest): Promise<ScanJob> =>
    api.post('/scans', body).then((res) => res.data),

  getScanStatus: (jobId: string): Promise<ScanJob> =>
    api.get(`/scans/${jobId}`).then((res) => res.data),
}
