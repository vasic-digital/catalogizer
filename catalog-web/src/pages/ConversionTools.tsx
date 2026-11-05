import React, { useState, useEffect } from 'react';
import { FormatConverter } from '@/components/conversion/FormatConverter';
import { conversionApi } from '@/lib/conversionApi';
import { useQuery } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import type { ConversionJob } from '@/types/conversion';

export const ConversionTools: React.FC = () => {
  const [jobs, setJobs] = useState<ConversionJob[]>([]);
  const [supportedFormats] = useState(['mp4', 'mkv', 'avi', 'mov', 'webm', 'mp3', 'wav', 'flac']);

  const { data, refetch } = useQuery({
    queryKey: ['conversion-jobs'],
    queryFn: () => conversionApi.getConversionJobs(),
    staleTime: 1000 * 60 * 2,
    refetchInterval: 1000 * 30, // Refresh every 30 seconds for active jobs
  });

  useEffect(() => {
    if (data) {
      setJobs(data);
    }
  }, [data]);

  const handleStartConversion = async (jobData: Omit<ConversionJob, 'id' | 'status' | 'progress'>) => {
    try {
      const newJob = await conversionApi.startConversion(jobData);
      setJobs(prev => [newJob, ...prev]);
      toast.success('Conversion started successfully');
      refetch(); // Refresh jobs list
    } catch (error) {
      toast.error(`Failed to start conversion: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleCancelConversion = async (id: string) => {
    try {
      await conversionApi.cancelConversion(id);
      setJobs(prev => 
        prev.map(job => 
          job.id === id ? { ...job, status: 'cancelled' } : job
        )
      );
      toast.success('Conversion cancelled');
    } catch (error) {
      toast.error(`Failed to cancel conversion: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleRetryConversion = async (id: string) => {
    try {
      await conversionApi.retryConversion(id);
      setJobs(prev => 
        prev.map(job => 
          job.id === id ? { ...job, status: 'pending', progress: 0 } : job
        )
      );
      toast.success('Conversion retry initiated');
    } catch (error) {
      toast.error(`Failed to retry conversion: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleDownloadFile = async (jobId: string) => {
    try {
      await conversionApi.downloadFile(jobId);
      toast.success('File downloaded successfully');
    } catch (error) {
      toast.error(`Failed to download file: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
          Format Converter
        </h1>
        <p className="text-gray-600 dark:text-gray-400">
          Convert media files to different formats with customizable quality settings
        </p>
      </div>
      
      <FormatConverter
        jobs={jobs}
        supportedFormats={supportedFormats}
        onStartConversion={handleStartConversion}
        onCancelConversion={handleCancelConversion}
        onRetryConversion={handleRetryConversion}
        onDownloadFile={handleDownloadFile}
      />
    </div>
  );
};