import React, { useState } from 'react';
import { Settings, PlayCircle, Download, Clock, CheckCircle, AlertCircle, X, RefreshCw } from 'lucide-react';
import { Button } from '../ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { Progress } from '../ui/Progress';
import { Input } from '../ui/Input';

interface ConversionJob {
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

interface FormatConverterProps {
  jobs: ConversionJob[];
  supportedFormats: string[];
  onStartConversion?: (job: Omit<ConversionJob, 'id' | 'status' | 'progress'>) => void;
  onCancelConversion?: (id: string) => void;
  onRetryConversion?: (id: string) => void;
  /** Receives the job id; the download endpoint is addressed by job id, not by output path. */
  onDownloadFile?: (jobId: string) => void;
}

/**
 * FormatConverter manages media format conversion jobs with quality/codec
 * settings, progress tracking, and download of completed outputs.
 */
export const FormatConverter: React.FC<FormatConverterProps> = ({
  jobs,
  supportedFormats,
  onStartConversion,
  onCancelConversion,
  onRetryConversion,
  onDownloadFile
}) => {
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [targetFormat, setTargetFormat] = useState('mp4');
  const [quality, setQuality] = useState<'low' | 'medium' | 'high' | 'ultra'>('medium');
  const [customOptions, setCustomOptions] = useState({
    resolution: '',
    bitrate: '',
    framerate: '',
    audioCodec: '',
    videoCodec: ''
  });

  const qualityPresets = {
    low: { resolution: '480p', bitrate: 1000, label: 'Low (Fast)' },
    medium: { resolution: '720p', bitrate: 2500, label: 'Medium (Balanced)' },
    high: { resolution: '1080p', bitrate: 5000, label: 'High (Quality)' },
    ultra: { resolution: '4K', bitrate: 15000, label: 'Ultra (Best)' }
  };

  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (event.target.files && event.target.files[0]) {
      setSelectedFile(event.target.files[0]);
    }
  };

  const handleStartConversion = () => {
    if (!selectedFile) return;

    const preset = qualityPresets[quality];
    const job: Omit<ConversionJob, 'id' | 'status' | 'progress'> = {
      sourceFile: {
        path: URL.createObjectURL(selectedFile),
        name: selectedFile.name,
        format: selectedFile.name.split('.').pop()?.toLowerCase() || 'unknown',
        size: selectedFile.size
      },
      targetFormat,
      quality,
      options: {
        resolution: customOptions.resolution || preset.resolution,
        bitrate: customOptions.bitrate ? parseInt(customOptions.bitrate) : preset.bitrate,
        framerate: customOptions.framerate ? parseInt(customOptions.framerate) : 30,
        audioCodec: customOptions.audioCodec || 'aac',
        videoCodec: customOptions.videoCodec || 'h264'
      }
    };

    onStartConversion?.(job);
    setShowCreateModal(false);
    setSelectedFile(null);
    setCustomOptions({
      resolution: '',
      bitrate: '',
      framerate: '',
      audioCodec: '',
      videoCodec: ''
    });
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDuration = (startTime?: string, endTime?: string) => {
    if (!startTime || !endTime) return '---';
    const start = new Date(startTime);
    const end = new Date(endTime);
    const duration = Math.round((end.getTime() - start.getTime()) / 1000);
    const minutes = Math.floor(duration / 60);
    const seconds = duration % 60;
    return `${minutes}m ${seconds}s`;
  };

  const getStatusIcon = (status: ConversionJob['status']) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="w-5 h-5 text-green-500" />;
      case 'processing':
        return <RefreshCw className="w-5 h-5 text-blue-500 animate-spin" />;
      case 'failed':
        return <AlertCircle className="w-5 h-5 text-red-500" />;
      case 'cancelled':
        return <X className="w-5 h-5 text-gray-500 dark:text-gray-400" />;
      default:
        return <Clock className="w-5 h-5 text-gray-400" />;
    }
  };

  const getStatusBadge = (status: ConversionJob['status']) => {
    const variants = {
      pending: 'secondary',
      processing: 'default',
      completed: 'default',
      failed: 'destructive',
      cancelled: 'outline',
      unknown: 'outline'
    } as const;

    return (
      <Badge variant={variants[status] ?? 'outline'}>
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </Badge>
    );
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">Format Converter</h2>
        <Button onClick={() => setShowCreateModal(true)}>
          <Settings className="w-4 h-4 mr-2" />
          New Conversion
        </Button>
      </div>

      {/* Active Jobs */}
      <div className="space-y-4">
        {jobs.map((job, index) => (
          <Card key={job.id ? job.id : `job-index-${index}`}>
            <CardContent className="p-6">
              <div className="flex items-start gap-4">
                <div className="mt-1">
                  {getStatusIcon(job.status)}
                </div>
                
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between mb-2">
                    <div>
                      <h3 className="font-medium text-gray-900 dark:text-white truncate">
                        {job.sourceFile?.name ?? 'unknown'}
                      </h3>
                      <p className="text-sm text-gray-600 dark:text-gray-300">
                        {(job.sourceFile?.format ?? 'unknown').toUpperCase()} → {job.targetFormat.toUpperCase()} • {job.quality}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      {getStatusBadge(job.status)}
                      {job.status === 'completed' && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => onDownloadFile?.(job.id)}
                        >
                          <Download className="w-4 h-4 mr-1" />
                          Download
                        </Button>
                      )}
                      {job.status === 'failed' && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => onRetryConversion?.(job.id)}
                        >
                          <RefreshCw className="w-4 h-4 mr-1" />
                          Retry
                        </Button>
                      )}
                      {['pending', 'processing'].includes(job.status) && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => onCancelConversion?.(job.id)}
                        >
                          Cancel
                        </Button>
                      )}
                    </div>
                  </div>

                  {/* Progress Bar */}
                  {job.status === 'processing' && (
                    <Progress value={job.progress} className="mb-3" showLabel />
                  )}

                  {/* Details */}
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm text-gray-600 dark:text-gray-300">
                    <div>
                      <span className="block font-medium">File Size</span>
                      {formatFileSize(job.sourceFile?.size ?? 0)}
                    </div>
                    <div>
                      <span className="block font-medium">Duration</span>
                      {formatDuration(job.startTime, job.endTime)}
                    </div>
                    <div>
                      <span className="block font-medium">Resolution</span>
                      {job.options?.resolution}
                    </div>
                    <div>
                      <span className="block font-medium">Bitrate</span>
                      {job.options?.bitrate != null ? `${job.options.bitrate} kbps` : '---'}
                    </div>
                  </div>

                  {/* Error Message */}
                  {job.status === 'failed' && job.error && (
                    <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
                      {job.error}
                    </div>
                  )}

                  {/* Success Message */}
                  {job.status === 'completed' && (
                    <div className="mt-3 p-3 bg-green-50 border border-green-200 rounded-lg text-sm text-green-700">
                      Conversion completed successfully. File is ready for download.
                    </div>
                  )}
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Empty State */}
      {jobs.length === 0 && (
        <Card className="text-center py-12">
          <CardContent>
            <div className="space-y-4">
              <div className="w-16 h-16 bg-gray-100 rounded-full flex items-center justify-center mx-auto">
                <Settings className="w-8 h-8 text-gray-400" />
              </div>
              <div>
                <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
                  No conversion jobs
                </h3>
                <p className="text-gray-600 dark:text-gray-300 mb-4">
                  Start your first media format conversion
                </p>
                <Button onClick={() => setShowCreateModal(true)}>
                  <Settings className="w-4 h-4 mr-2" />
                  Create Conversion Job
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Create Conversion Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-2xl max-h-[90vh] overflow-y-auto">
            <CardHeader>
              <CardTitle>Create Conversion Job</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* File Selection */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Select File
                </label>
                <input
                  type="file"
                  accept="video/*,audio/*"
                  onChange={handleFileSelect}
                  className="block w-full text-sm text-gray-500 dark:text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
                />
                {selectedFile && (
                  <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
                    Selected: {selectedFile.name} ({formatFileSize(selectedFile.size)})
                  </p>
                )}
              </div>

              {/* Target Format */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Target Format
                </label>
                <select
                  value={targetFormat}
                  onChange={(e) => setTargetFormat(e.target.value)}
                  className="w-full p-3 border border-gray-300 rounded-lg"
                >
                  {supportedFormats.map(format => (
                    <option key={format} value={format}>
                      {format.toUpperCase()}
                    </option>
                  ))}
                </select>
              </div>

              {/* Quality Selection */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Quality Preset
                </label>
                <div className="grid grid-cols-2 gap-3">
                  {Object.entries(qualityPresets).map(([key, preset]) => (
                    <label
                      key={key}
                      className={`flex items-center p-3 border rounded-lg cursor-pointer transition-colors ${
                        quality === key 
                          ? 'border-blue-500 bg-blue-50' 
                          : 'border-gray-200 hover:border-gray-300'
                      }`}
                    >
                      <input
                        type="radio"
                        name="quality"
                        value={key}
                        checked={quality === key}
                        onChange={(e) => setQuality(e.target.value as 'low' | 'medium' | 'high' | 'ultra')}
                        className="sr-only"
                      />
                      <div>
                        <div className="font-medium">{preset.label}</div>
                        <div className="text-sm text-gray-600 dark:text-gray-300">
                          {preset.resolution} • {preset.bitrate}kbps
                        </div>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              {/* Custom Options */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Custom Options (Optional)
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <Input
                    placeholder="Resolution (e.g., 1080p)"
                    value={customOptions.resolution}
                    onChange={(e) => setCustomOptions(prev => ({ ...prev, resolution: e.target.value }))}
                  />
                  <Input
                    placeholder="Bitrate (kbps)"
                    type="number"
                    value={customOptions.bitrate}
                    onChange={(e) => setCustomOptions(prev => ({ ...prev, bitrate: e.target.value }))}
                  />
                  <Input
                    placeholder="Framerate (fps)"
                    type="number"
                    value={customOptions.framerate}
                    onChange={(e) => setCustomOptions(prev => ({ ...prev, framerate: e.target.value }))}
                  />
                  <Input
                    placeholder="Audio Codec (e.g., aac)"
                    value={customOptions.audioCodec}
                    onChange={(e) => setCustomOptions(prev => ({ ...prev, audioCodec: e.target.value }))}
                  />
                </div>
              </div>

              {/* Actions */}
              <div className="flex gap-3">
                <Button
                  variant="outline"
                  onClick={() => setShowCreateModal(false)}
                  className="flex-1"
                >
                  Cancel
                </Button>
                <Button
                  onClick={handleStartConversion}
                  disabled={!selectedFile}
                  className="flex-1"
                >
                  <PlayCircle className="w-4 h-4 mr-2" />
                  Start Conversion
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
};