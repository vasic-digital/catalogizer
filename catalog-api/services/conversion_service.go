package services

import (
	"catalogizer/internal/pathguard"
	"context"
	"encoding/json"
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
	args := s.buildFFmpegVideoArgs(job)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg video conversion failed: %w", err)
	}

	return nil
}

func (s *ConversionService) convertAudio(job *models.ConversionJob) error {
	args := s.buildFFmpegAudioArgs(job)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
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
	args := []string{
		job.SourcePath,
		job.TargetPath,
	}

	if job.Settings != nil {
		settings := make(map[string]interface{})
		if err := json.Unmarshal([]byte(*job.Settings), &settings); err == nil {
			if cover, ok := settings["preserve_cover"].(bool); ok && cover {
				args = append(args, "--preserve-cover")
			}
			if metadata, ok := settings["preserve_metadata"].(bool); ok && metadata {
				args = append(args, "--preserve-metadata")
			}
		}
	}

	cmd := exec.Command("ebook-convert", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
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

		// Create output file
		file, err := os.Create(outputPath)
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

	// Create output file
	outputFile, err := os.Create(job.TargetPath)
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
	// Try pandoc first for HTML conversion
	args := []string{
		"-f", "pdf",
		"-t", "html",
		"-o", job.TargetPath,
		job.SourcePath,
	}

	cmd := exec.Command("pandoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return nil
	}

	// Fallback to LibreOffice if pandoc is not available
	args = []string{
		"--headless",
		"--convert-to", "html",
		"--outdir", filepath.Dir(job.TargetPath),
		job.SourcePath,
	}

	cmd = exec.Command("libreoffice", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err == nil {
		// LibreOffice might create a different filename, rename if needed
		libreOutput := strings.TrimSuffix(job.SourcePath, filepath.Ext(job.SourcePath)) + ".html"
		if _, err := os.Stat(libreOutput); err == nil {
			if libreOutput != job.TargetPath {
				if err := os.Rename(libreOutput, job.TargetPath); err != nil {
					if logging.Logger != nil {
						logging.With(logging.ErrorField(err)).Warn("Failed to rename LibreOffice output")
					}
				}
			}
		}
		return nil
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
	return os.WriteFile(job.TargetPath, []byte(htmlContent), 0644)
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

	args = append(args, job.TargetPath)

	cmd := exec.Command("convert", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("imagemagick PDF conversion failed: %w", err)
	}

	return nil
}

func (s *ConversionService) convertImage(job *models.ConversionJob) error {
	args := s.buildImageMagickArgs(job)

	cmd := exec.Command("convert", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
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
		if _, err := pathguard.Confine(p, pathguard.RootsFromEnv("CATALOGIZER_CONVERSION_ROOTS", filepath.Join(".", "data"))); err != nil {
			return false
		}
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
