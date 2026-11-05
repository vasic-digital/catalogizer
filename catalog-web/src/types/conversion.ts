export interface ConversionJob {
  id: string;
  sourceFile: {
    path: string;
    name: string;
    format: string;
    size: number;
  };
  targetFormat: string;
  quality: 'low' | 'medium' | 'high' | 'ultra';
  status: 'pending' | 'processing' | 'completed' | 'failed' | 'cancelled' | 'unknown';
  progress: number;
  startTime?: string;
  endTime?: string;
  outputFile?: string;
  error?: string;
  options: {
    resolution?: string;
    bitrate?: number;
    framerate?: number;
    audioCodec?: string;
    videoCodec?: string;
  };
}