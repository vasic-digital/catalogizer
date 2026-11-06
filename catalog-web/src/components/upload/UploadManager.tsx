import React, { useState, useCallback, useEffect, useRef } from 'react';
import { Upload, File, CheckCircle, AlertCircle, Trash2 } from 'lucide-react';
import { Button } from '../ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/Card';
import { Progress } from '../ui/Progress';

interface UploadItem {
  id: string;
  file: File;
  progress: number;
  status: 'pending' | 'uploading' | 'success' | 'error';
  error?: string;
}

interface UploadManagerProps {
  onUpload?: (files: File[]) => Promise<void>;
  onRemove?: (id: string) => void;
  onRetry?: (id: string) => void;
  maxFileSize?: number;
  acceptedTypes?: string[];
  maxConcurrentUploads?: number;
}

/**
 * UploadManager provides drag-and-drop and file-picker upload with per-file
 * progress tracking, size validation, and retry/remove controls.
 */
export const UploadManager: React.FC<UploadManagerProps> = ({
  onUpload,
  onRemove,
  onRetry: _onRetry,
  maxFileSize = 100 * 1024 * 1024, // 100MB
  acceptedTypes = ['video/*', 'audio/*', 'image/*'],
  maxConcurrentUploads: _maxConcurrentUploads = 3
}) => {
  const [uploadQueue, setUploadQueue] = useState<UploadItem[]>([]);
  const [isDragOver, setIsDragOver] = useState(false);
  const pendingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (pendingTimerRef.current) {
        clearTimeout(pendingTimerRef.current);
      }
    };
  }, []);

  const processFiles = useCallback((files: FileList) => {
    const newItems: UploadItem[] = Array.from(files).map(file => ({
      id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      file,
      progress: 0,
      status: 'pending'
    }));

    setUploadQueue(prev => [...prev, ...newItems]);

    if (pendingTimerRef.current) {
      clearTimeout(pendingTimerRef.current);
    }
    pendingTimerRef.current = setTimeout(() => startUploads(newItems), 100);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // The upload is whatever the host's onUpload does. There is no timer-driven progress and
  // success is shown only after onUpload resolves. Without onUpload nothing can be sent, so
  // each file is marked failed with that reason (CAT-001: the old version simulated progress
  // and showed a green success for a file that was never sent).
  const startUploads = useCallback(async (items: UploadItem[]) => {
    for (const item of items) {
      if (!onUpload) {
        setUploadQueue(prev =>
          prev.map(i => i.id === item.id ? {
            ...i,
            status: 'error',
            error: 'Upload is not available: the server has no upload endpoint. The file was not sent.'
          } : i)
        );
        continue;
      }
      try {
        setUploadQueue(prev =>
          prev.map(i => i.id === item.id ? { ...i, status: 'uploading' } : i)
        );
        await onUpload([item.file]);
        setUploadQueue(prev =>
          prev.map(i => i.id === item.id ? { ...i, status: 'success', progress: 100 } : i)
        );
      } catch (error) {
        setUploadQueue(prev =>
          prev.map(i => i.id === item.id ? {
            ...i,
            status: 'error',
            error: error instanceof Error ? error.message : 'Upload failed'
          } : i)
        );
      }
    }
  }, [onUpload]);

  const handleDrop = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragOver(false);
    
    const files = Array.from(e.dataTransfer.files);
    const validFiles = files.filter(file => {
      if (file.size > maxFileSize) {
        return false;
      }
      
      if (acceptedTypes.length > 0 && !acceptedTypes.some(type => {
        if (type.endsWith('/*')) {
          return file.type.startsWith(type.slice(0, -2));
        }
        return file.type === type;
      })) {
        return false;
      }
      
      return true;
    });

    processFiles(validFiles as unknown as FileList);
  }, [processFiles, maxFileSize, acceptedTypes]);

  const handleDragOver = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragOver(false);
  }, []);

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      processFiles(e.target.files);
    }
  }, [processFiles]);

  const removeItem = useCallback((id: string) => {
    setUploadQueue(prev => prev.filter(item => item.id !== id));
    onRemove?.(id);
  }, [onRemove]);

  const retryUpload = useCallback((id: string) => {
    const item = uploadQueue.find(i => i.id === id);
    if (item) {
      startUploads([{ ...item, status: 'pending', progress: 0, error: undefined }]);
    }
  }, [uploadQueue, startUploads]);

  const clearCompleted = useCallback(() => {
    setUploadQueue(prev => prev.filter(item => item.status !== 'success'));
  }, []);

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getStatusIcon = (status: UploadItem['status']) => {
    switch (status) {
      case 'success':
        return <CheckCircle className="w-4 h-4 text-green-500" />;
      case 'error':
        return <AlertCircle className="w-4 h-4 text-red-500" />;
      case 'uploading':
        return <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent animate-spin" />;
      default:
        return <File className="w-4 h-4 text-gray-500 dark:text-gray-400" />;
    }
  };

  return (
    <Card className="w-full max-w-4xl mx-auto">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Upload className="h-5 w-5" />
            Upload Manager
          </CardTitle>
          {uploadQueue.some(item => item.status === 'success') && (
            <Button
              variant="outline"
              size="sm"
              onClick={clearCompleted}
            >
              Clear Completed
            </Button>
          )}
        </div>
      </CardHeader>
      
      <CardContent>
        {/* Drop Zone */}
        <div
          className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
            isDragOver 
              ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20' 
              : 'border-gray-300 hover:border-gray-400'
          }`}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
        >
          <Upload className="w-12 h-12 mx-auto mb-4 text-gray-400" />
          <p className="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">
            Drop files here or click to browse
          </p>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            Max file size: {formatFileSize(maxFileSize)}
            {acceptedTypes.length > 0 && (
              <> • Accepted types: {acceptedTypes.join(', ')}</>
            )}
          </p>
          <input
            type="file"
            multiple
            accept={acceptedTypes.join(',')}
            onChange={handleFileSelect}
            className="hidden"
            id="file-input"
          />
          <Button>
            <label htmlFor="file-input" className="cursor-pointer">
              Select Files
            </label>
          </Button>
        </div>

        {/* Upload Queue */}
        {uploadQueue.length > 0 && (
          <div className="mt-6">
            <h3 className="text-lg font-medium mb-4">
              Upload Queue ({uploadQueue.length} items)
            </h3>
            <div className="space-y-3">
              {uploadQueue.map(item => (
                <div
                  key={item.id}
                  className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
                >
                  {getStatusIcon(item.status)}
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                      {item.file.name}
                    </p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {formatFileSize(item.file.size)}
                    </p>
                    {item.status === 'uploading' && (
                      <Progress value={item.progress} className="mt-2" />
                    )}
                    {item.status === 'error' && (
                      <p className="text-xs text-red-700 dark:text-red-300 mt-1">{item.error}</p>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {item.status === 'error' && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => retryUpload(item.id)}
                      >
                        Retry
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => removeItem(item.id)}
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
};