package services

import (
	"catalogizer/internal/pathguard"
	"catalogizer/internal/pathsafe"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"catalogizer/internal/auth"
	"catalogizer/internal/logging"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/gen2brain/go-fitz"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
	"golang.org/x/sync/semaphore"
)

type ConversionService struct {
	conversionRepo *repository.ConversionRepository
	userRepo       *repository.UserRepository
	authService    *AuthService
	sem            *semaphore.Weighted // Limits concurrent conversion processes
}

func NewConversionService(conversionRepo *repository.ConversionRepository, userRepo *repository.UserRepository, authService *AuthService) *ConversionService {
	return &ConversionService{
		conversionRepo: conversionRepo,
		userRepo:       userRepo,
		authService:    authService,
		sem:            semaphore.NewWeighted(3), // Max 3 concurrent conversions
	}
}

func (s *ConversionService) CreateConversionJob(userID int, request *models.ConversionRequest) (*models.ConversionJob, error) {
	if !s.validateConversionRequest(request) {
		return nil, fmt.Errorf("invalid conversion request")
	}

	job := &models.ConversionJob{
		UserID:         userID,
		SourcePath:     request.SourcePath,
		TargetPath:     request.TargetPath,
		SourceFormat:   request.SourceFormat,
		TargetFormat:   request.TargetFormat,
		ConversionType: request.ConversionType,
		Quality:        request.Quality,
		Settings:       request.Settings,
		Priority:       request.Priority,
		Status:         models.ConversionStatusPending,
		CreatedAt:      time.Now(),
		ScheduledFor:   request.ScheduledFor,
	}

	id, err := s.conversionRepo.CreateJob(job)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversion job: %w", err)
	}

	job.ID = id
	return job, nil
}

func (s *ConversionService) StartConversion(jobID int) error {
	job, err := s.conversionRepo.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get conversion job: %w", err)
	}

	if job.Status != models.ConversionStatusPending {
		return fmt.Errorf("job is not in pending status")
	}

	// jobs stored before the path confinement existed are re-checked before anything runs
	if cerr := confineConversionPaths(job.SourcePath, job.TargetPath); cerr != nil {
		s.handleConversionError(job, fmt.Errorf("conversion refused: %w", cerr))
		return fmt.Errorf("conversion refused: %w", cerr)
	}

	job.Status = models.ConversionStatusRunning
	job.StartedAt = &time.Time{}
	*job.StartedAt = time.Now()

	err = s.conversionRepo.UpdateJob(job)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	go s.processConversion(job)

	return nil
}

func (s *ConversionService) processConversion(job *models.ConversionJob) {
	// Limit concurrent conversion processes (ffmpeg, LibreOffice, etc.)
	// Use a timeout to prevent blocking forever if all semaphore slots are occupied.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := s.sem.Acquire(ctx, 1); err != nil {
		s.handleConversionError(job, fmt.Errorf("failed to acquire conversion semaphore: %w", err))
		return
	}
	defer s.sem.Release(1)

	var err error

	defer func() {
		if r := recover(); r != nil {
			s.handleConversionError(job, fmt.Errorf("conversion panic: %v", r))
		}
	}()

	switch job.ConversionType {
	case models.ConversionTypeVideo:
		err = s.convertVideo(job)
	case models.ConversionTypeAudio:
		err = s.convertAudio(job)
	case models.ConversionTypeDocument:
		err = s.convertDocument(job)
	case models.ConversionTypeImage:
		err = s.convertImage(job)
	default:
		err = fmt.Errorf("unsupported conversion type: %s", job.ConversionType)
	}

	if err != nil {
		s.handleConversionError(job, err)
		return
	}

	s.handleConversionSuccess(job)
}

func (s *ConversionService) convertVideo(job *models.ConversionJob) error {
	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		j := *job
		j.TargetPath = out
		return runConverterTool("ffmpeg", s.buildFFmpegVideoArgs(&j))
	})
	if err != nil {
		return fmt.Errorf("ffmpeg video conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) convertAudio(job *models.ConversionJob) error {
	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		j := *job
		j.TargetPath = out
		return runConverterTool("ffmpeg", s.buildFFmpegAudioArgs(&j))
	})
	if err != nil {
		return fmt.Errorf("ffmpeg audio conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) convertDocument(job *models.ConversionJob) error {
	switch {
	case s.isEbookConversion(job):
		return s.convertEbook(job)
	case s.isPDFConversion(job):
		return s.convertPDF(job)
	default:
		return fmt.Errorf("unsupported document conversion")
	}
}

func (s *ConversionService) convertEbook(job *models.ConversionJob) error {
	var opts []string

	if job.Settings != nil {
		settings := make(map[string]interface{})
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			if cover, ok := settings["preserve_cover"].(bool); ok && cover {
				opts = append(opts, "--preserve-cover")
			}
			if metadata, ok := settings["preserve_metadata"].(bool); ok && metadata {
				opts = append(opts, "--preserve-metadata")
			}
		}
	}

	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		return runConverterTool("ebook-convert", append([]string{job.SourcePath, out}, opts...))
	})
	if err != nil {
		return fmt.Errorf("ebook conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) convertPDF(job *models.ConversionJob) error {
	// Determine target format and use appropriate conversion method
	ext := strings.ToLower(filepath.Ext(job.TargetPath))
	targetFormat := strings.TrimPrefix(ext, ".")

	switch targetFormat {
	case "jpg", "jpeg", "png", "bmp", "tiff", "gif":
		return s.convertPDFToImage(job, targetFormat)
	case "txt", "text":
		return s.convertPDFToText(job)
	case "html":
		return s.convertPDFToHTML(job)
	default:
		return fmt.Errorf("unsupported PDF conversion target format: %s", targetFormat)
	}
}

// convertPDFToImage converts PDF pages to images using go-fitz library
func (s *ConversionService) convertPDFToImage(job *models.ConversionJob, format string) error {
	doc, err := fitz.New(job.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	// Get total page count
	totalPages := doc.NumPage()

	// Parse settings to determine which pages to convert
	settings := make(map[string]interface{})
	if job.Settings != nil {
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			// Settings parsed successfully
		}
	}

	// Determine page range
	startPage := 0
	endPage := totalPages

	if page, ok := settings["page"].(int); ok && page >= 0 && page < totalPages {
		// Single page
		startPage = page
		endPage = page + 1
	} else if start, ok := settings["start_page"].(int); ok && start >= 0 {
		startPage = start
		if end, ok := settings["end_page"].(int); ok && end > start && end <= totalPages {
			endPage = end
		}
	}

	// Determine DPI for quality (default 150)
	dpi := 150
	if dpiVal, ok := settings["dpi"].(int); ok && dpiVal > 0 {
		dpi = dpiVal
	}

	// Convert each page
	for i := startPage; i < endPage; i++ {
		img, err := doc.ImageDPI(i, float64(dpi))
		if err != nil {
			return fmt.Errorf("failed to render page %d: %w", i+1, err)
		}

		// Determine output file path
		outputPath := job.TargetPath
		if totalPages > 1 {
			// Add page number to filename for multi-page PDFs
			dir := filepath.Dir(outputPath)
			name := filepath.Base(outputPath)
			ext := filepath.Ext(outputPath)
			nameWithoutExt := strings.TrimSuffix(name, ext)
			outputPath = filepath.Join(dir, fmt.Sprintf("%s_page_%d%s", nameWithoutExt, i+1, ext))
		}

		// Create output file (the per-page name is derived, so it is confined here, review round 6, F4)
		file, err := createConversionOutput(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		// Encode based on format
		switch format {
		case "jpg", "jpeg":
			err = jpeg.Encode(file, img, &jpeg.Options{Quality: 85})
		case "png":
			err = png.Encode(file, img)
		default:
			// For other formats, use ImageMagick as fallback
			return s.convertPDFWithImageMagick(job, format)
		}

		if err != nil {
			return fmt.Errorf("failed to encode image: %w", err)
		}

		// If this was a single page conversion, break
		if totalPages == 1 {
			break
		}
	}

	return nil
}

// convertPDFToText converts PDF to plain text using unipdf
func (s *ConversionService) convertPDFToText(job *models.ConversionJob) error {
	file, err := os.Open(job.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to open PDF: %w", err)
	}
	defer file.Close()

	pdfReader, err := model.NewPdfReader(file)
	if err != nil {
		return fmt.Errorf("failed to create PDF reader: %w", err)
	}

	// Parse settings
	settings := make(map[string]interface{})
	if job.Settings != nil {
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			// Settings parsed successfully
		}
	}

	// Determine page range
	totalPages, err := pdfReader.GetNumPages()
	if err != nil {
		return fmt.Errorf("failed to get total pages: %w", err)
	}
	startPage := 1
	endPage := totalPages

	if page, ok := settings["page"].(int); ok && page > 0 && page <= totalPages {
		startPage = page
		endPage = page
	} else if start, ok := settings["start_page"].(int); ok && start > 0 {
		startPage = start
		if end, ok := settings["end_page"].(int); ok && end >= start && end <= totalPages {
			endPage = end
		}
	}

	// Create output file (also used for the HTML fallback's ".tmp.txt" sibling)
	outputFile, err := createConversionOutput(job.TargetPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// Extract text from pages
	for i := startPage; i <= endPage; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return fmt.Errorf("failed to get page %d: %w", i, err)
		}

		extractor, err := extractor.New(page)
		if err != nil {
			return fmt.Errorf("failed to create extractor for page %d: %w", i, err)
		}

		content, err := extractor.ExtractText()
		if err != nil {
			return fmt.Errorf("failed to extract text from page %d: %w", i, err)
		}

		if _, err := outputFile.WriteString(content); err != nil {
			return fmt.Errorf("failed to write text to file: %w", err)
		}

		// Add page separator for multi-page PDFs
		if i < endPage {
			if _, err := outputFile.WriteString(fmt.Sprintf("\n\n--- Page %d ---\n\n", i+1)); err != nil {
				return fmt.Errorf("failed to write page separator: %w", err)
			}
		}
	}

	return nil
}

// convertPDFToHTML converts PDF to HTML using external tools
func (s *ConversionService) convertPDFToHTML(job *models.ConversionJob) error {
	// pandoc and LibreOffice each run in their own private directory and only regular files are
	// moved into place (review round 7, S2). A link, FIFO or directory already at the target is
	// refused before any tool runs (review round 6, F4).
	if err := refuseNonRegularLeaf(job.TargetPath); err != nil {
		return err
	}
	// Try pandoc first for HTML conversion
	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		return runConverterTool("pandoc", []string{"-f", "pdf", "-t", "html", "-o", out, job.SourcePath})
	})
	if err == nil {
		return nil
	}
	if !isConverterToolError(err) {
		return err
	}

	// Fallback to LibreOffice if pandoc is not available. It names its output after the source,
	// in --outdir; that file is renamed to the target's name inside the private directory.
	err = runConverterInPrivateDir(job.TargetPath, func(out string) error {
		priv := filepath.Dir(out)
		if err := runConverterTool("libreoffice", []string{"--headless", "--convert-to", "html", "--outdir", priv, job.SourcePath}); err != nil {
			return err
		}
		return moveLibreOfficeOutput(libreOfficeHTMLOutput(job.SourcePath, out), out)
	})
	if err == nil {
		return nil
	}
	if !isConverterToolError(err) {
		return err
	}

	// Last resort: try to convert PDF to text then wrap in HTML
	tempTextFile := job.TargetPath + ".tmp.txt"
	err = s.convertPDFToText(&models.ConversionJob{
		SourcePath: job.SourcePath,
		TargetPath: tempTextFile,
		Settings:   job.Settings,
	})
	if err != nil {
		return fmt.Errorf("failed to convert PDF to text for HTML conversion: %w", err)
	}
	defer os.Remove(tempTextFile)

	// Read the text content
	content, err := os.ReadFile(tempTextFile)
	if err != nil {
		return fmt.Errorf("failed to read temp text file: %w", err)
	}

	// Create basic HTML
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        pre { white-space: pre-wrap; word-wrap: break-word; }
        .page-break { page-break-before: always; border-top: 2px solid #ccc; margin-top: 20px; padding-top: 20px; }
    </style>
</head>
<body>
    <h1>%s</h1>
    <pre>%s</pre>
</body>
</html>`, filepath.Base(job.SourcePath), filepath.Base(job.SourcePath), string(content))

	// Write HTML file
	out, err := createConversionOutput(job.TargetPath)
	if err != nil {
		return err
	}
	if _, err := out.WriteString(htmlContent); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// convertPDFWithImageMagick converts PDF to image formats not directly supported by go-fitz
func (s *ConversionService) convertPDFWithImageMagick(job *models.ConversionJob, format string) error {
	args := []string{
		"-density", "150", // DPI
		job.SourcePath,
	}

	// Parse settings for quality options
	if job.Settings != nil {
		settings := make(map[string]interface{})
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			if dpi, ok := settings["dpi"].(int); ok && dpi > 0 {
				args[1] = fmt.Sprintf("%d", dpi)
			}
			if quality, ok := settings["quality"].(int); ok && quality > 0 {
				args = append(args, "-quality", fmt.Sprintf("%d", quality))
			}
		}
	}

	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		return runConverterTool("convert", append(append([]string(nil), args...), out))
	})
	if err != nil {
		return fmt.Errorf("imagemagick PDF conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) convertImage(job *models.ConversionJob) error {
	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		j := *job
		j.TargetPath = out
		return runConverterTool("convert", s.buildImageMagickArgs(&j))
	})
	if err != nil {
		return fmt.Errorf("imagemagick conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) buildFFmpegVideoArgs(job *models.ConversionJob) []string {
	args := []string{
		"-i", job.SourcePath,
		"-y", // Overwrite output file
	}

	switch job.Quality {
	case "low":
		args = append(args, "-crf", "28", "-preset", "fast")
	case "medium":
		args = append(args, "-crf", "23", "-preset", "medium")
	case "high":
		args = append(args, "-crf", "18", "-preset", "slow")
	case "lossless":
		args = append(args, "-crf", "0", "-preset", "veryslow")
	default:
		args = append(args, "-crf", "23", "-preset", "medium")
	}

	if job.Settings != nil {
		settings := make(map[string]interface{})
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			if resolution, ok := settings["resolution"].(string); ok {
				args = append(args, "-s", resolution)
			}
			if framerate, ok := settings["framerate"].(string); ok {
				args = append(args, "-r", framerate)
			}
			if bitrate, ok := settings["bitrate"].(string); ok {
				args = append(args, "-b:v", bitrate)
			}
		}
	}

	args = append(args, job.TargetPath)
	return args
}

func (s *ConversionService) buildFFmpegAudioArgs(job *models.ConversionJob) []string {
	args := []string{
		"-i", job.SourcePath,
		"-y", // Overwrite output file
	}

	switch job.Quality {
	case "low":
		args = append(args, "-ab", "96k")
	case "medium":
		args = append(args, "-ab", "192k")
	case "high":
		args = append(args, "-ab", "320k")
	case "lossless":
		args = append(args, "-c:a", "flac")
	default:
		args = append(args, "-ab", "192k")
	}

	if job.Settings != nil {
		settings := make(map[string]interface{})
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			if sampleRate, ok := settings["sample_rate"].(string); ok {
				args = append(args, "-ar", sampleRate)
			}
			if channels, ok := settings["channels"].(string); ok {
				args = append(args, "-ac", channels)
			}
		}
	}

	args = append(args, job.TargetPath)
	return args
}

func (s *ConversionService) buildImageMagickArgs(job *models.ConversionJob) []string {
	args := []string{job.SourcePath}

	if job.Settings != nil {
		settings := make(map[string]interface{})
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			if resize, ok := settings["resize"].(string); ok {
				args = append(args, "-resize", resize)
			}
			if quality, ok := settings["quality"].(string); ok {
				args = append(args, "-quality", quality)
			}
			if compress, ok := settings["compress"].(bool); ok && compress {
				args = append(args, "-compress", "JPEG")
			}
		}
	}

	args = append(args, job.TargetPath)
	return args
}

func (s *ConversionService) handleConversionSuccess(job *models.ConversionJob) {
	job.Status = models.ConversionStatusCompleted
	job.CompletedAt = &time.Time{}
	*job.CompletedAt = time.Now()

	if job.StartedAt != nil {
		duration := job.CompletedAt.Sub(*job.StartedAt)
		job.Duration = &duration
	}

	err := s.conversionRepo.UpdateJob(job)
	if err != nil {
		if logging.Logger != nil {
			logging.With(logging.Int("job_id", job.ID), logging.ErrorField(err)).Warn("Failed to update completed conversion job")
		}
	}

	s.notifyUser(job, "Conversion completed successfully")
}

func (s *ConversionService) handleConversionError(job *models.ConversionJob, conversionError error) {
	job.Status = models.ConversionStatusFailed
	job.CompletedAt = &time.Time{}
	*job.CompletedAt = time.Now()
	errorMsg := conversionError.Error()
	job.ErrorMessage = &errorMsg

	if job.StartedAt != nil {
		duration := job.CompletedAt.Sub(*job.StartedAt)
		job.Duration = &duration
	}

	err := s.conversionRepo.UpdateJob(job)
	if err != nil {
		if logging.Logger != nil {
			logging.With(logging.Int("job_id", job.ID), logging.ErrorField(err)).Warn("Failed to update failed conversion job")
		}
	}

	s.notifyUser(job, fmt.Sprintf("Conversion failed: %s", conversionError.Error()))
}

func (s *ConversionService) notifyUser(job *models.ConversionJob, message string) {
	// In a full implementation, this would send notifications via email, push, etc.
	if logging.Logger != nil {
		logging.With(
			logging.Int("user_id", job.UserID),
			logging.Int("job_id", job.ID),
			logging.String("message", message),
		).Debug("Conversion notification")
	}
}

func (s *ConversionService) GetUserJobs(userID int, status *string, limit, offset int) ([]models.ConversionJob, error) {
	return s.conversionRepo.GetUserJobs(userID, status, limit, offset)
}

func (s *ConversionService) GetJob(jobID int, userID int) (*models.ConversionJob, error) {
	job, err := s.conversionRepo.GetJob(jobID)
	if err != nil {
		return nil, err
	}

	if job.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, auth.PermissionViewMedia)
		if err != nil || !hasPermission {
			return nil, fmt.Errorf("unauthorized to view this job")
		}
	}

	return job, nil
}

func (s *ConversionService) CancelJob(jobID int, userID int) error {
	job, err := s.conversionRepo.GetJob(jobID)
	if err != nil {
		return err
	}

	if job.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, auth.PermissionManageUsers)
		if err != nil || !hasPermission {
			return fmt.Errorf("unauthorized to cancel this job")
		}
	}

	if job.Status != models.ConversionStatusPending && job.Status != models.ConversionStatusRunning {
		return fmt.Errorf("cannot cancel job in status: %s", job.Status)
	}

	job.Status = models.ConversionStatusCancelled
	job.CompletedAt = &time.Time{}
	*job.CompletedAt = time.Now()

	return s.conversionRepo.UpdateJob(job)
}

func (s *ConversionService) RetryJob(jobID int, userID int) error {
	job, err := s.conversionRepo.GetJob(jobID)
	if err != nil {
		return err
	}

	if job.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, auth.PermissionManageUsers)
		if err != nil || !hasPermission {
			return fmt.Errorf("unauthorized to retry this job")
		}
	}

	if job.Status != models.ConversionStatusFailed {
		return fmt.Errorf("can only retry failed jobs")
	}

	job.Status = models.ConversionStatusPending
	job.StartedAt = nil
	job.CompletedAt = nil
	job.Duration = nil
	job.ErrorMessage = nil

	err = s.conversionRepo.UpdateJob(job)
	if err != nil {
		return err
	}

	return s.StartConversion(jobID)
}

func (s *ConversionService) GetJobStatistics(userID *int, startDate, endDate time.Time) (*models.ConversionStatistics, error) {
	return s.conversionRepo.GetStatistics(userID, startDate, endDate)
}

func (s *ConversionService) CleanupCompletedJobs(olderThan time.Time) error {
	return s.conversionRepo.CleanupJobs(olderThan)
}

func (s *ConversionService) GetSupportedFormats() *models.SupportedFormats {
	return &models.SupportedFormats{
		Video: models.VideoFormats{
			Input:  []string{"mp4", "avi", "mkv", "mov", "wmv", "flv", "webm", "m4v", "3gp"},
			Output: []string{"mp4", "avi", "mkv", "mov", "webm", "m4v"},
		},
		Audio: models.AudioFormats{
			Input:  []string{"mp3", "wav", "flac", "aac", "ogg", "wma", "m4a", "opus"},
			Output: []string{"mp3", "wav", "flac", "aac", "ogg", "m4a", "opus"},
		},
		Document: models.DocumentFormats{
			Input:  []string{"epub", "mobi", "azw", "azw3", "pdf", "txt", "docx", "odt"},
			Output: []string{"epub", "mobi", "pdf", "txt", "html"},
		},
		Image: models.ImageFormats{
			Input:  []string{"jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp", "svg"},
			Output: []string{"jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp"},
		},
	}
}

// moveLibreOfficeOutput renames the file LibreOffice wrote to the target's name (since review
// round 7 both paths lie in the private converter directory). Only a
// regular file inside the conversion write roots is moved: renaming a planted symlink over the
// target would make the download handler serve the link's target (review round 6, F4).
func moveLibreOfficeOutput(libreOutput, target string) error {
	fi, err := os.Lstat(libreOutput)
	if os.IsNotExist(err) || (err == nil && libreOutput == target) {
		return nil
	}
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("LibreOffice output %q is not a regular file", libreOutput)
	}
	roots := pathguard.ConversionWriteRoots()
	if _, err := pathguard.ConfineDataFile(libreOutput, roots); err != nil {
		return fmt.Errorf("LibreOffice output %q rejected: %w", libreOutput, err)
	}
	if _, err := pathguard.ConfineDataFile(target, roots); err != nil {
		return fmt.Errorf("target path %q rejected: %w", target, err)
	}
	return os.Rename(libreOutput, target)
}

// converterToolError marks a failure of the external tool itself (missing binary, non-zero exit),
// as opposed to a refusal by the path checks: only a tool failure lets convertPDFToHTML fall back
// to the next converter.
type converterToolError struct{ err error }

func (e *converterToolError) Error() string { return e.err.Error() }
func (e *converterToolError) Unwrap() error { return e.err }

func isConverterToolError(err error) bool {
	var te *converterToolError
	return errors.As(err, &te)
}

func runConverterTool(name string, args []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return &converterToolError{fmt.Errorf("%s: %w", name, err)}
	}
	return nil
}

// runConverterInPrivateDir runs an external converter that opens its output by path and follows
// a symlink there: ffmpeg, ebook-convert, ImageMagick (which also writes derived names such as
// out-0.bmp for multi-frame input and expands %d patterns), pandoc and LibreOffice. The tool gets
// <private dir>/<base(target)>, where the private directory is a fresh 0700 directory created
// next to the target (inside the conversion write roots). Afterwards every entry it produced must
// be a regular file, and each is renamed into Dir(target) under its own name. rename(2) replaces
// the destination name and never follows a link there; a link, FIFO or directory already at any
// destination is refused before anything is moved, so a planted link is not replaced either
// (review round 7, S2).
func runConverterInPrivateDir(target string, run func(out string) error) error {
	roots := pathguard.ConversionWriteRoots()
	if _, err := pathguard.ConfineDataFile(target, roots); err != nil {
		return fmt.Errorf("target path %q rejected: %w", target, err)
	}
	if err := refuseNonRegularLeaf(target); err != nil {
		return err
	}
	dir := filepath.Dir(target)
	priv, err := os.MkdirTemp(dir, ".catalogizer-conv-")
	if err != nil {
		return fmt.Errorf("failed to create private converter directory in %q: %w", dir, err)
	}
	if priv == "" {
		return fmt.Errorf("private converter directory has an empty name")
	}
	defer os.RemoveAll(priv)
	if err := run(filepath.Join(priv, filepath.Base(target))); err != nil {
		return err
	}
	return publishConverterOutputs(priv, dir, roots)
}

// publishConverterOutputs moves the converter's outputs from the private directory into dir. All
// entries are checked before the first is moved, so a refusal publishes nothing.
func publishConverterOutputs(priv, dir string, roots []string) error {
	entries, err := os.ReadDir(priv)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("converter produced no output")
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			return fmt.Errorf("converter output %q is not a regular file", e.Name())
		}
		dst := filepath.Join(dir, e.Name())
		if _, err := pathguard.ConfineDataFile(dst, roots); err != nil {
			return fmt.Errorf("output path %q rejected: %w", dst, err)
		}
		if err := refuseNonRegularLeaf(dst); err != nil {
			return err
		}
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(priv, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// createConversionOutput opens a file a converter writes itself: the target or a name derived
// from it (per-page images, the HTML fallback's temp file). The path must lie in the conversion
// write roots, and a symlink in its final component is refused (O_NOFOLLOW).
func createConversionOutput(p string) (*os.File, error) {
	if _, err := pathguard.ConfineDataFile(p, pathguard.ConversionWriteRoots()); err != nil {
		return nil, fmt.Errorf("output path %q rejected: %w", p, err)
	}
	f, err := pathsafe.CreateNoFollow(p, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file %q: %w", p, err)
	}
	return f, nil
}

// refuseNonRegularLeaf refuses a path an external converter will write when something other
// than a regular file already sits there (a symlink the tool would follow). A missing path is
// fine.
func refuseNonRegularLeaf(p string) error {
	fi, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("output path %q exists and is not a regular file", p)
	}
	return nil
}

// libreOfficeHTMLOutput is where "libreoffice --convert-to html --outdir Dir(target) source"
// writes: the source's base name with .html, in the output directory (since review round 7 the
// output directory is the private converter directory). It used to be computed next to the
// SOURCE, so an unrelated <source>.html in the read root was renamed onto the target, i.e. moved
// out of the read root (review round 4).
func libreOfficeHTMLOutput(source, target string) string {
	base := filepath.Base(source)
	return filepath.Join(filepath.Dir(target), strings.TrimSuffix(base, filepath.Ext(base))+".html")
}

// confineConversionPaths: the input may come from the read roots (CATALOGIZER_READ_ROOTS,
// default MEDIA_ROOT_PATH + ./data) or the conversion write roots; the output must lie in the
// conversion write roots (CATALOGIZER_CONVERSION_ROOTS, default ./data). Neither may be the
// application database, a SQLite companion of it, or a directory containing it
// (pathguard.ConfineData, review round 3 N2). Both are checked exactly as the request gives
// them, without filepath.Clean, because that raw string is what the converter opens. Both are
// FILE paths (pathguard.ConfineDataFile): a root itself is refused, because LibreOffice writes
// into Dir(target) and the multi-page writer next to the target, which for the root is its parent
// (review round 5, RV5-B). §11.4.115.
func confineConversionPaths(source, target string) error {
	write := pathguard.ConversionWriteRoots()
	if _, err := pathguard.ConfineDataFile(source, pathguard.Readable(write)); err != nil {
		return fmt.Errorf("source path %q rejected: %w", source, err)
	}
	if _, err := pathguard.ConfineDataFile(target, write); err != nil {
		return fmt.Errorf("target path %q rejected: %w", target, err)
	}
	return nil
}

func (s *ConversionService) validateConversionRequest(request *models.ConversionRequest) bool {
	if request.SourcePath == "" || request.TargetPath == "" {
		return false
	}

	// Sanitize paths: reject path traversal and null bytes
	for _, p := range []string{request.SourcePath, request.TargetPath} {
		if strings.Contains(p, "\x00") {
			return false
		}
		cleaned := filepath.Clean(p)
		if strings.Contains(cleaned, "..") {
			return false
		}
		// Absolute paths must stay inside the configured conversion roots; an
		// authenticated caller must not read /etc/passwd or write ~/.ssh (§11.4.115).
	}
	if confineConversionPaths(request.SourcePath, request.TargetPath) != nil {
		return false
	}

	if request.SourceFormat == "" || request.TargetFormat == "" {
		return false
	}

	if request.ConversionType == "" {
		return false
	}

	if !s.isValidConversionType(request.ConversionType) {
		return false
	}

	if !s.isSupportedFormat(request.ConversionType, request.SourceFormat, request.TargetFormat) {
		return false
	}

	return true
}

func (s *ConversionService) isValidConversionType(conversionType string) bool {
	validTypes := []string{
		models.ConversionTypeVideo,
		models.ConversionTypeAudio,
		models.ConversionTypeDocument,
		models.ConversionTypeImage,
	}

	for _, validType := range validTypes {
		if conversionType == validType {
			return true
		}
	}

	return false
}

func (s *ConversionService) isSupportedFormat(conversionType, sourceFormat, targetFormat string) bool {
	formats := s.GetSupportedFormats()

	switch conversionType {
	case models.ConversionTypeVideo:
		return s.isFormatSupported(sourceFormat, formats.Video.Input) && s.isFormatSupported(targetFormat, formats.Video.Output)
	case models.ConversionTypeAudio:
		return s.isFormatSupported(sourceFormat, formats.Audio.Input) && s.isFormatSupported(targetFormat, formats.Audio.Output)
	case models.ConversionTypeDocument:
		return s.isFormatSupported(sourceFormat, formats.Document.Input) && s.isFormatSupported(targetFormat, formats.Document.Output)
	case models.ConversionTypeImage:
		return s.isFormatSupported(sourceFormat, formats.Image.Input) && s.isFormatSupported(targetFormat, formats.Image.Output)
	}

	return false
}

func (s *ConversionService) isFormatSupported(format string, supportedFormats []string) bool {
	format = strings.ToLower(format)
	for _, supported := range supportedFormats {
		if format == strings.ToLower(supported) {
			return true
		}
	}
	return false
}

func (s *ConversionService) isEbookConversion(job *models.ConversionJob) bool {
	ebookFormats := []string{"epub", "mobi", "azw", "azw3", "txt", "html"}
	return s.isFormatSupported(job.SourceFormat, ebookFormats) || s.isFormatSupported(job.TargetFormat, ebookFormats)
}

func (s *ConversionService) isPDFConversion(job *models.ConversionJob) bool {
	return job.SourceFormat == "pdf" || job.TargetFormat == "pdf"
}

func (s *ConversionService) GetJobQueue() ([]models.ConversionJob, error) {
	return s.conversionRepo.GetJobsByStatus(models.ConversionStatusPending, 100, 0)
}

func (s *ConversionService) ProcessJobQueue() error {
	jobs, err := s.GetJobQueue()
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if job.ScheduledFor != nil && job.ScheduledFor.After(time.Now()) {
			continue
		}

		err := s.StartConversion(job.ID)
		if err != nil {
			if logging.Logger != nil {
				logging.With(logging.Int("job_id", job.ID), logging.ErrorField(err)).Error("Failed to start conversion job")
			}
		}
	}

	return nil
}
