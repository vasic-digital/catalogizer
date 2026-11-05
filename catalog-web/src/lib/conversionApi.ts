import type { ConversionJob } from '@/types/conversion';
import { api } from './api';

const STATUS_MAP: Record<string, ConversionJob['status']> = {
  pending: 'pending',
  running: 'processing',
  processing: 'processing',
  completed: 'completed',
  failed: 'failed',
  cancelled: 'cancelled',
};

const basename = (p: unknown): string => {
  if (typeof p !== 'string' || p === '') return '';
  const parts = p.split(/[\\/]/);
  return parts[parts.length - 1] || p;
};

/**
 * Normalizes one job as returned by GET /conversion/jobs (the flat catalog-api
 * model: id, source_path, target_path, source_format, target_format, status,
 * error_message, started_at, completed_at, ...) into the nested ConversionJob
 * the UI renders. Already-nested jobs are accepted and have their missing
 * parts defaulted, so a rendered job never lacks sourceFile / options.
 */
export function mapConversionJob(raw: any): ConversionJob {
  const r = raw ?? {};
  const src = r.sourceFile ?? {};
  const sourcePath: string = src.path ?? r.source_path ?? '';
  const status = STATUS_MAP[String(r.status ?? '').toLowerCase()] ?? 'unknown';
  const quality = ['low', 'medium', 'high', 'ultra'].includes(r.quality) ? r.quality : 'medium';
  return {
    id: String(r.id ?? ''),
    sourceFile: {
      path: sourcePath,
      name: src.name ?? (basename(sourcePath) || 'unknown'),
      format: src.format ?? r.source_format ?? 'unknown',
      size: typeof src.size === 'number' ? src.size : 0,
    },
    targetFormat: r.targetFormat ?? r.target_format ?? 'unknown',
    quality,
    status,
    progress: typeof r.progress === 'number' ? r.progress : status === 'completed' ? 100 : 0,
    startTime: r.startTime ?? r.started_at ?? undefined,
    endTime: r.endTime ?? r.completed_at ?? undefined,
    outputFile: r.outputFile ?? (status === 'completed' ? r.target_path : undefined) ?? undefined,
    error: r.error ?? r.error_message ?? undefined,
    options: r.options ?? {},
  };
}

export const conversionApi = {
  async getConversionJobs(): Promise<ConversionJob[]> {
    const response = await api.get('/conversion/jobs');
    const list = Array.isArray(response.data) ? response.data : response.data?.jobs;
    return Array.isArray(list) ? list.map(mapConversionJob) : [];
  },

  async startConversion(jobData: Omit<ConversionJob, 'id' | 'status' | 'progress'>): Promise<ConversionJob> {
    const response = await api.post('/conversion/jobs', jobData);
    return response.data;
  },

  async cancelConversion(id: string): Promise<void> {
    await api.delete(`/conversion/jobs/${id}`);
  },

  async retryConversion(id: string): Promise<void> {
    await api.post(`/conversion/jobs/${id}/retry`);
  },

  async downloadFile(id: string): Promise<void> {
    const response = await api.get(`/conversion/jobs/${id}/download`, {
      responseType: 'blob'
    });
    const url = window.URL.createObjectURL(response.data);
    const link = document.createElement('a');
    link.href = url;
    link.download = `conversion-${id}`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }
};
