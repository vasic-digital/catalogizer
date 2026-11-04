package services

import (
	"catalogizer/internal/pathguard"
	"catalogizer/internal/pathsafe"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"catalogizer/internal/auth"
	"catalogizer/internal/logging"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/gen2brain/go-fitz"
	"golang.org/x/sync/semaphore"
)

type ConversionService struct {
	conversionRepo *repository.ConversionRepository
	userRepo       *repository.UserRepository
	authService    *AuthService
	sem            *semaphore.Weighted // Limits concurrent conversion processes

	ctxMu     sync.RWMutex
	workerCtx context.Context // cancelled when the worker is stopping; converter tools are killed with it

	workerRunning atomic.Bool   // guards against a second RunWorker on this service
	jobTimeout    time.Duration // per-job wall-clock limit (CATALOGIZER_CONVERSION_TIMEOUT, default 2h)

	runsMu  sync.Mutex
	runs    map[int]*jobRun // running jobs by id: lets CancelJob kill the tool
	jobCtxs sync.Map        // *models.ConversionJob -> context.Context of that execution
}

// jobRun is the cancellation handle of one running job.
type jobRun struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// DefaultConversionTimeout bounds one conversion; override with CATALOGIZER_CONVERSION_TIMEOUT
// (a Go duration such as "30m" or "6h").
const DefaultConversionTimeout = 2 * time.Hour

func conversionTimeoutFromEnv() time.Duration {
	if v := os.Getenv("CATALOGIZER_CONVERSION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
		if logging.Logger != nil {
			logging.With(logging.String("value", v), logging.String("used", DefaultConversionTimeout.String())).
				Warn("Invalid CATALOGIZER_CONVERSION_TIMEOUT (need a positive Go duration such as 30m); using the default")
		}
	}
	return DefaultConversionTimeout
}

func NewConversionService(conversionRepo *repository.ConversionRepository, userRepo *repository.UserRepository, authService *AuthService) *ConversionService {
	return &ConversionService{
		conversionRepo: conversionRepo,
		userRepo:       userRepo,
		authService:    authService,
		sem:            semaphore.NewWeighted(3), // Max 3 concurrent conversions
		jobTimeout:     conversionTimeoutFromEnv(),
		runs:           map[int]*jobRun{},
	}
}

func (s *ConversionService) CreateConversionJob(userID int, request *models.ConversionRequest) (*models.ConversionJob, error) {
	if reason := s.conversionRequestProblem(request); reason != "" {
		return nil, fmt.Errorf("%w: %s", models.ErrInvalidConversionRequest, reason)
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

// StartConversion claims one pending job and runs it in the background. The claim is atomic, so
// a job can never be started twice. Concurrency is bounded by the service semaphore.
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

	if err := s.claim(job); err != nil {
		return err
	}

	go s.processConversion(job)

	return nil
}

// claim moves the job from pending to running exactly once across all callers.
func (s *ConversionService) claim(job *models.ConversionJob) error {
	now := time.Now()
	won, err := s.conversionRepo.ClaimJob(job.ID, now)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	if !won {
		return fmt.Errorf("job is not in pending status")
	}
	job.Status = models.ConversionStatusRunning
	job.StartedAt = &now
	return nil
}

// processConversion waits for a concurrency slot (bounded) and runs the job.
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
	s.executeJob(job)
}

// executeJob runs a claimed (running) job to a terminal state. The caller holds a semaphore slot.
func (s *ConversionService) executeJob(job *models.ConversionJob) {
	var err error

	// per-job context: worker shutdown, owner cancel (CancelJob) and the per-job timeout all kill
	// the tool's process group through it.
	runCtx, cancelRun := context.WithTimeout(s.toolContext(), s.jobTimeout)
	defer cancelRun()
	run := &jobRun{ctx: runCtx, cancel: cancelRun}
	s.registerRun(job.ID, run)
	defer s.unregisterRun(job.ID, run)
	s.jobCtxs.Store(job, runCtx) // the tools of THIS execution run under this context
	defer s.jobCtxs.Delete(job)

	// cancelled between the claim and now: nothing to run
	if cur, gerr := s.conversionRepo.GetJob(job.ID); gerr == nil && cur.Status != models.ConversionStatusRunning {
		return
	}

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
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("conversion timed out after %s: %w", s.jobTimeout, err)
		}
		s.handleConversionError(job, err)
		return
	}

	s.handleConversionSuccess(job)
}

func (s *ConversionService) registerRun(id int, r *jobRun) {
	s.runsMu.Lock()
	s.runs[id] = r
	s.runsMu.Unlock()
}

// unregisterRun removes the entry only if it is still this run's: a stale run must not
// unregister a newer run of the same job id (retry + re-claim).
func (s *ConversionService) unregisterRun(id int, r *jobRun) {
	s.runsMu.Lock()
	if s.runs[id] == r {
		delete(s.runs, id)
	}
	s.runsMu.Unlock()
}

// killRun stops the tool of a running job, if this process runs it.
func (s *ConversionService) killRun(id int) {
	s.runsMu.Lock()
	r := s.runs[id]
	s.runsMu.Unlock()
	if r != nil {
		r.cancel()
	}
}

// runCtxFor is the context of this execution of the job (per job pointer, so a stale run of the
// same id can never lend or lose its context); outside an execution it is the worker's.
func (s *ConversionService) runCtxFor(job *models.ConversionJob) context.Context {
	if v, ok := s.jobCtxs.Load(job); ok {
		return v.(context.Context)
	}
	return s.toolContext()
}

// RunWorker is the conversion worker: it drains the pending queue every interval until ctx is
// cancelled. Concurrency is bounded by the service semaphore (never more than its capacity of
// jobs run at once, and the worker only claims a job when a slot is free). On cancellation it
// kills running converter tools, returns their jobs to `pending` (they run again after restart),
// and returns only when every job goroutine has finished. Jobs left `running` by a previous
// process are requeued at start (this process is the only worker for the database).
func (s *ConversionService) RunWorker(ctx context.Context, interval time.Duration) {
	if !s.workerRunning.CompareAndSwap(false, true) {
		if logging.Logger != nil {
			logging.Warn("Conversion worker already running; second start ignored")
		}
		return
	}
	defer s.workerRunning.Store(false)
	if interval <= 0 {
		interval = 2 * time.Second
	}
	s.ctxMu.Lock()
	s.workerCtx = ctx
	s.ctxMu.Unlock()

	if n, skipped, err := s.conversionRepo.NormalizeLegacyScheduledFor(); err != nil {
		if logging.Logger != nil {
			logging.With(logging.ErrorField(err)).Warn("Conversion worker could not normalise legacy scheduled_for values")
		}
	} else if (n > 0 || skipped > 0) && logging.Logger != nil {
		logging.With(logging.Int("rewritten", n), logging.Int("unparseable", skipped)).Info("Conversion worker normalised legacy scheduled_for values to UTC")
	}

	if n, err := s.conversionRepo.RequeueRunning(); err != nil {
		if logging.Logger != nil {
			logging.With(logging.ErrorField(err)).Warn("Conversion worker could not requeue interrupted jobs")
		}
	} else if n > 0 && logging.Logger != nil {
		logging.With(logging.Int("requeued", int(n))).Info("Conversion worker requeued interrupted jobs")
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.dispatchPending(ctx, &wg)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// dispatchPending claims pending jobs while concurrency slots are free and runs each in its own
// goroutine registered on wg.
func (s *ConversionService) dispatchPending(ctx context.Context, wg *sync.WaitGroup) {
	jobs, err := s.GetJobQueue()
	if err != nil {
		if logging.Logger != nil {
			logging.With(logging.ErrorField(err)).Error("Conversion worker could not read the job queue")
		}
		return
	}
	for i := range jobs {
		if ctx.Err() != nil {
			return
		}
		job := jobs[i]
		if job.ScheduledFor != nil && job.ScheduledFor.After(time.Now()) {
			continue
		}
		if !s.sem.TryAcquire(1) {
			return // all slots busy; the next tick continues
		}
		// jobs stored before the path confinement existed are re-checked before anything runs
		if cerr := confineConversionPaths(job.SourcePath, job.TargetPath); cerr != nil {
			s.sem.Release(1)
			s.handleConversionError(&job, fmt.Errorf("conversion refused: %w", cerr))
			continue
		}
		if err := s.claim(&job); err != nil {
			s.sem.Release(1)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.sem.Release(1)
			s.executeJob(&job)
		}()
	}
}

// toolContext is the context converter tools run under: the worker's, or Background outside it.
func (s *ConversionService) toolContext() context.Context {
	s.ctxMu.RLock()
	defer s.ctxMu.RUnlock()
	if s.workerCtx != nil {
		return s.workerCtx
	}
	return context.Background()
}

func (s *ConversionService) runTool(job *models.ConversionJob, name string, args []string) error {
	return runConverterToolCtx(s.runCtxFor(job), name, args)
}

func (s *ConversionService) convertVideo(job *models.ConversionJob) error {
	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		j := *job
		j.TargetPath = out
		return s.runTool(job, "ffmpeg", s.buildFFmpegVideoArgs(&j))
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
		return s.runTool(job, "ffmpeg", s.buildFFmpegAudioArgs(&j))
	})
	if err != nil {
		return fmt.Errorf("ffmpeg audio conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) convertDocument(job *models.ConversionJob) error {
	// A PDF source going to a PDF-native target (text / html / image) is served by the isolated
	// PDF path (CAT-002) first: the ebook list contains txt and html, so testing it first sent
	// pdf -> txt to calibre, which is not installed in the image (D1).
	switch s.documentRoute(job) {
	case "pdf":
		return s.convertPDF(job)
	case "ebook":
		return s.convertEbook(job)
	}
	return fmt.Errorf("unsupported document conversion")
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
		return s.runTool(job, "ebook-convert", append([]string{job.SourcePath, out}, opts...))
	})
	if err != nil {
		return fmt.Errorf("ebook conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) convertPDF(job *models.ConversionJob) error {
	// Determine target format and use appropriate conversion method
	// dispatch on the declared (canonical) target format, not the file name's spelling (.htm, .text)
	targetFormat := canonicalFormat(job.TargetFormat)
	if targetFormat == "" {
		targetFormat = canonicalFormat(filepath.Ext(job.TargetPath))
	}

	switch targetFormat {
	case "jpg", "png", "bmp", "tiff", "gif":
		return s.convertPDFToImage(job, targetFormat)
	case "txt":
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

// convertPDFToText converts PDF to plain text through the pdfTextExtractor interface
func (s *ConversionService) convertPDFToText(job *models.ConversionJob) error {
	// CAT-002: text extraction goes through the pdfTextExtractor interface: poppler (pdftotext when
	// installed) first, otherwise / on failure the pure-Go ledongthuc/pdf, both in prlimit-capped
	// child processes (pdf_text_isolation.go); the former unipdf engine needed a commercial licence
	// key at run time.
	ctx, cancel := context.WithTimeout(context.Background(), pdfTextTimeout)
	defer cancel()
	if f, err := os.Open(job.SourcePath); err != nil {
		return fmt.Errorf("failed to open PDF: %w", err)
	} else {
		f.Close()
	}
	ex := defaultPDFTextExtractor()

	totalPages, err := ex.NumPages(ctx, job.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to get total pages: %w", err)
	}

	// Parse settings
	settings := make(map[string]interface{})
	if job.Settings != nil {
		_ = json.Unmarshal([]byte(*job.Settings), &settings)
	}
	startPage, endPage := selectPDFPages(settings, totalPages)
	if totalPages == 0 || startPage > endPage {
		return fmt.Errorf("PDF has no pages in the requested range")
	}

	pages, err := ex.ExtractPages(ctx, job.SourcePath, startPage, endPage)
	if err != nil {
		return err
	}

	// Create output file (also used for the HTML fallback's ".tmp.txt" sibling)
	outputFile, err := createConversionOutput(job.TargetPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	for i, content := range pages {
		if _, err := outputFile.WriteString(content); err != nil {
			return fmt.Errorf("failed to write text to file: %w", err)
		}
		// Add page separator for multi-page PDFs
		if i < len(pages)-1 {
			if _, err := outputFile.WriteString(fmt.Sprintf("\n\n--- Page %d ---\n\n", startPage+i+1)); err != nil {
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
		return s.runTool(job, "pandoc", []string{"-f", "pdf", "-t", "html", "-o", out, job.SourcePath})
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
		if err := s.runTool(job, "libreoffice", []string{"--headless", "--convert-to", "html", "--outdir", priv, job.SourcePath}); err != nil {
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
</html>`, html.EscapeString(filepath.Base(job.SourcePath)), html.EscapeString(filepath.Base(job.SourcePath)), html.EscapeString(string(content)))

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
	// F2: IM rasterises a PDF through Ghostscript. Without `gs` it cannot, so say that instead of
	// failing with an opaque delegate error (the runtime images do not ship Ghostscript: it is a
	// large PostScript interpreter and PDF -> png/jpg does not need it, go-fitz renders those).
	if _, err := lookPath("gs"); err != nil {
		return fmt.Errorf("PDF to %s needs Ghostscript (gs), which is not installed on this server; convert to png or jpg instead", format)
	}
	args := []string{
		"-density", "150", // DPI
		"pdf:" + job.SourcePath,
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
		return s.runTool(job, "convert", append(append([]string(nil), args...), out))
	})
	if err != nil {
		return fmt.Errorf("imagemagick PDF conversion failed: %w", err)
	}
	return nil
}

func (s *ConversionService) convertImage(job *models.ConversionJob) error {
	prefix, perr := imageMagickCoder(job.SourcePath)
	if perr != nil {
		return fmt.Errorf("imagemagick conversion failed: %w", perr)
	}
	source := job.SourcePath
	if prefix == "svg:" { // svg: render a private copy of the re-serialised, checked document
		raw, serr := readCheckedSVG(job.SourcePath)
		if serr != nil {
			return fmt.Errorf("imagemagick conversion failed: %w", serr)
		}
		dir, derr := os.MkdirTemp("", "catalogizer-svg-")
		if derr != nil {
			return fmt.Errorf("imagemagick conversion failed: %w", derr)
		}
		defer os.RemoveAll(dir)
		source = filepath.Join(dir, "in.svg")
		if werr := os.WriteFile(source, raw, 0o600); werr != nil {
			return fmt.Errorf("imagemagick conversion failed: %w", werr)
		}
	}
	err := runConverterInPrivateDir(job.TargetPath, func(out string) error {
		j := *job
		j.TargetPath = out
		j.SourcePath = prefix + source
		return s.runTool(job, "convert", s.buildImageMagickArgs(&j))
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

	// conditional: a job the owner cancelled meanwhile stays cancelled
	written, err := s.conversionRepo.UpdateJobIf(job, models.ConversionStatusRunning)
	if err != nil {
		if logging.Logger != nil {
			logging.With(logging.Int("job_id", job.ID), logging.ErrorField(err)).Warn("Failed to update completed conversion job")
		}
		return
	}
	if !written {
		return
	}

	s.notifyUser(job, "Conversion completed successfully")
}

func (s *ConversionService) handleConversionError(job *models.ConversionJob, conversionError error) {
	// The worker is shutting down and killed the tool: not a conversion failure. Return the job
	// to the queue so it runs again after restart.
	if wc := s.toolContext(); wc.Err() != nil && errors.Is(wc.Err(), context.Canceled) {
		job.Status = models.ConversionStatusPending
		job.StartedAt = nil
		job.CompletedAt = nil
		job.Duration = nil
		job.ErrorMessage = nil
		// only a job that is still running goes back; a cancelled job is never resurrected
		if _, err := s.conversionRepo.UpdateJobIf(job, models.ConversionStatusRunning); err != nil && logging.Logger != nil {
			logging.With(logging.Int("job_id", job.ID), logging.ErrorField(err)).Warn("Failed to requeue interrupted conversion job")
		}
		return
	}

	job.Status = models.ConversionStatusFailed
	job.CompletedAt = &time.Time{}
	*job.CompletedAt = time.Now()
	errorMsg := sanitizeConversionError(conversionError.Error())
	job.ErrorMessage = &errorMsg

	if job.StartedAt != nil {
		duration := job.CompletedAt.Sub(*job.StartedAt)
		job.Duration = &duration
	}

	written, err := s.conversionRepo.UpdateJobIf(job, models.ConversionStatusPending, models.ConversionStatusRunning)
	if err != nil {
		if logging.Logger != nil {
			logging.With(logging.Int("job_id", job.ID), logging.ErrorField(err)).Warn("Failed to update failed conversion job")
		}
		return
	}
	if !written {
		return // cancelled meanwhile: the stored state wins
	}

	s.notifyUser(job, fmt.Sprintf("Conversion failed: %s", errorMsg))
}

// Paths are removed in two passes. Pass 1: a QUOTED path (one that starts right after ' " or a
// backtick and ends at the next quote, so spaces and ": " inside a directory name are covered).
// Pass 2: an UNQUOTED path starts at "/", "file://" or a drive letter after the line start,
// whitespace or one of = ( [ , ; : and runs (spaces included) to a quote, "<>|;", a colon
// followed by whitespace, or the line end. "//" (URLs) never starts a path. Text after a path with
// no delimiter is over-removed, which is the safe direction.
const conversionPathBody = `(?:file://)?/[^/'"` + "`" + `<>|\r\n][^'"` + "`" + `\r\n]*|[A-Za-z]:[\\/][^'"` + "`" + `\r\n]*`

var (
	conversionQuotedPathRE = regexp.MustCompile("(['\"`])(" + conversionPathBody + ")(['\"`])")
	conversionPathRE       = regexp.MustCompile("(^|[\\s=(\\[,;:])((?:file://)?/[^/'\"`<>|\\s][^'\"`<>|;\\r\\n:]*(?::[^\\s'\"`<>|;\\r\\n][^'\"`<>|;\\r\\n:]*)*|[A-Za-z]:[\\\\/][^'\"`<>|;\\r\\n]*)")
)

func pathBase(p string) string {
	p = strings.TrimRight(p, " \t")
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	return "<path>/" + p
}

// sanitizeConversionError removes server filesystem paths (D7) from a tool or service error
// before it is stored on the job or shown to the user; only the base name is kept, no
// directory component is disclosed.
func sanitizeConversionError(msg string) string {
	msg = conversionQuotedPathRE.ReplaceAllStringFunc(msg, func(m string) string {
		sub := conversionQuotedPathRE.FindStringSubmatch(m)
		return sub[1] + pathBase(sub[2]) + sub[3]
	})
	return conversionPathRE.ReplaceAllStringFunc(msg, func(m string) string {
		sub := conversionPathRE.FindStringSubmatch(m)
		return sub[1] + pathBase(sub[2])
	})
}

// conversionPreDeleteHook lets a test move a job between the DeleteJob snapshot and the delete.
var conversionPreDeleteHook func()

var conversionNotifyHook func(job *models.ConversionJob, message string)

// documentRoute names the converter that serves a document job: "pdf" (the isolated PDF path:
// pdf -> txt / html / image) or "ebook" (calibre, for every other advertised pair), "" if none.
func (s *ConversionService) documentRoute(job *models.ConversionJob) string {
	switch {
	case s.isPDFNativeConversion(job):
		return "pdf"
	case documentPairSupported(job.SourceFormat, job.TargetFormat):
		return "ebook"
	}
	return ""
}

func (s *ConversionService) notifyUser(job *models.ConversionJob, message string) {
	if conversionNotifyHook != nil {
		conversionNotifyHook(job, message)
	}
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
			return nil, fmt.Errorf("%w", models.ErrConversionJobNotFound)
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
			return fmt.Errorf("%w", models.ErrConversionJobNotFound)
		}
	}

	if job.Status != models.ConversionStatusPending && job.Status != models.ConversionStatusRunning {
		return fmt.Errorf("%w: cannot cancel job in status: %s", models.ErrConversionJobState, job.Status)
	}

	job.Status = models.ConversionStatusCancelled
	job.CompletedAt = &time.Time{}
	*job.CompletedAt = time.Now()

	// conditional: only a pending or running job can be cancelled; a finish that raced us wins
	written, err := s.conversionRepo.UpdateJobIf(job, models.ConversionStatusPending, models.ConversionStatusRunning)
	if err != nil {
		return err
	}
	if !written {
		return fmt.Errorf("%w: job already finished", models.ErrConversionJobState)
	}
	// the stored state is `cancelled` now (so the finisher's conditional write is a no-op); kill
	// the tool and its process group
	s.killRun(jobID)
	return nil
}

// DeleteJob removes a job that is not running: pending, completed, failed or cancelled (D2). The
// owner or an administrator may delete. A running job gets ErrConversionJobState (409): cancel it
// first. The output file of a completed job is removed too, but only when it is a regular file
// inside the conversion write roots (never a symlink, never the job's own source).
func (s *ConversionService) DeleteJob(jobID int, userID int) error {
	job, err := s.conversionRepo.GetJob(jobID)
	if err != nil {
		return err
	}

	if job.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, auth.PermissionManageUsers)
		if err != nil || !hasPermission {
			return fmt.Errorf("%w", models.ErrConversionJobNotFound)
		}
	}

	if job.Status == models.ConversionStatusRunning {
		return fmt.Errorf("%w: cannot delete a running job; cancel it first", models.ErrConversionJobState)
	}

	if conversionPreDeleteHook != nil {
		conversionPreDeleteHook()
	}
	// atomic and conditional on the status we saw: a claim or finish that raced us leaves the row
	// (and the file) alone and we report 409
	deleted, err := s.conversionRepo.DeleteJobIf(jobID, job.Status)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("%w: job state changed", models.ErrConversionJobState)
	}

	if job.Status == models.ConversionStatusCompleted {
		removeConversionOutput(job)
	}
	return nil
}

// removeConversionOutput deletes a finished job's output file, best effort. It acts only on the
// job's OWN target path, never on what a symlink there points to: the leaf must be a regular file
// (Lstat, no follow), inside the conversion write roots after resolving its directory, and not
// the job's source (same file identity, hard links included). The leaf is re-checked right before
// the remove. Residual: a parent directory swapped for a symlink between that last check and the
// remove is not excluded without unlinkat on a directory fd.
func removeConversionOutput(job *models.ConversionJob) {
	if fi, err := os.Lstat(job.TargetPath); err != nil || !fi.Mode().IsRegular() {
		return
	}
	resolved, err := pathguard.ConfineDataFile(job.TargetPath, pathguard.ConversionWriteRoots())
	if err != nil {
		return
	}
	fi, err := os.Lstat(resolved)
	if err != nil || !fi.Mode().IsRegular() {
		return
	}
	if si, serr := os.Stat(job.SourcePath); serr == nil && os.SameFile(si, fi) {
		return
	}
	if fi2, err := os.Lstat(resolved); err != nil || !fi2.Mode().IsRegular() || !os.SameFile(fi, fi2) {
		return
	}
	_ = os.Remove(resolved)
}

func (s *ConversionService) RetryJob(jobID int, userID int) error {
	job, err := s.conversionRepo.GetJob(jobID)
	if err != nil {
		return err
	}

	if job.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, auth.PermissionManageUsers)
		if err != nil || !hasPermission {
			return fmt.Errorf("%w", models.ErrConversionJobNotFound)
		}
	}

	if job.Status != models.ConversionStatusFailed {
		return fmt.Errorf("%w: can only retry failed jobs", models.ErrConversionJobState)
	}

	job.Status = models.ConversionStatusPending
	job.StartedAt = nil
	job.CompletedAt = nil
	job.Duration = nil
	job.ErrorMessage = nil

	// The job goes back to the queue; the worker (RunWorker) claims and runs it. Only from
	// `failed`: two concurrent retries (double click) must not rewrite a job a worker already
	// claimed.
	written, err := s.conversionRepo.UpdateJobIf(job, models.ConversionStatusFailed)
	if err != nil {
		return err
	}
	if !written {
		return fmt.Errorf("%w: can only retry failed jobs", models.ErrConversionJobState)
	}
	return nil
}

func (s *ConversionService) GetJobStatistics(userID *int, startDate, endDate time.Time) (*models.ConversionStatistics, error) {
	return s.conversionRepo.GetStatistics(userID, startDate, endDate)
}

func (s *ConversionService) CleanupCompletedJobs(olderThan time.Time) error {
	return s.conversionRepo.CleanupJobs(olderThan)
}

// converterLookPath resolves an executable; a variable so tests can model a host without a tool.
var converterLookPath = exec.LookPath

func converterPresent(names ...string) bool {
	for _, n := range names {
		if _, err := converterLookPath(n); err == nil {
			return true
		}
	}
	return false
}

var (
	allVideoInput  = []string{"mp4", "avi", "mkv", "mov", "wmv", "flv", "webm", "m4v", "3gp"}
	allVideoOutput = []string{"mp4", "avi", "mkv", "mov", "webm", "m4v"}
	allAudioInput  = []string{"mp3", "wav", "flac", "aac", "ogg", "wma", "m4a", "opus"}
	allAudioOutput = []string{"mp3", "wav", "flac", "aac", "ogg", "m4a", "opus"}
	allImageInput  = []string{"jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp", "svg"}
	allImageOutput = []string{"jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp"}

	ebookSources = []string{"epub", "mobi", "azw", "azw3", "pdf", "txt", "html", "docx", "odt"}
	ebookTargets = []string{"epub", "mobi", "pdf", "txt", "html"}
)

// documentPairSupported says whether this server can really convert src -> dst (D1): the pairs
// served by the isolated PDF path need no external tool (txt) or pandoc/LibreOffice (html);
// every other pair needs calibre's ebook-convert.
func documentPairSupported(src, dst string) bool {
	src, dst = canonicalFormat(src), canonicalFormat(dst)
	if src == dst {
		return false
	}
	if src == "pdf" {
		switch dst {
		case "txt":
			return true
		case "html":
			return converterPresent("pandoc", "libreoffice", "soffice")
		}
	}
	return converterPresent("ebook-convert") && inList(src, ebookSources) && inList(dst, ebookTargets)
}

func inList(v string, list []string) bool {
	for _, x := range list {
		if v == x {
			return true
		}
	}
	return false
}

// GetSupportedFormats advertises only what the tools present on this server can do.
func (s *ConversionService) GetSupportedFormats() *models.SupportedFormats {
	f := &models.SupportedFormats{}
	f.Video = models.VideoFormats{Input: []string{}, Output: []string{}}
	f.Audio = models.AudioFormats{Input: []string{}, Output: []string{}}
	f.Image = models.ImageFormats{Input: []string{}, Output: []string{}}
	f.Document = models.DocumentFormats{Input: []string{}, Output: []string{}}
	if converterPresent("ffmpeg") {
		f.Video = models.VideoFormats{Input: allVideoInput, Output: allVideoOutput}
		f.Audio = models.AudioFormats{Input: allAudioInput, Output: allAudioOutput}
	}
	if converterPresent("convert") {
		f.Image = models.ImageFormats{Input: allImageInput, Output: allImageOutput}
	}
	for _, src := range ebookSources {
		for _, dst := range ebookTargets {
			if documentPairSupported(src, dst) {
				if !inList(src, f.Document.Input) {
					f.Document.Input = append(f.Document.Input, src)
				}
				if !inList(dst, f.Document.Output) {
					f.Document.Output = append(f.Document.Output, dst)
				}
			}
		}
	}
	return f
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
	return runConverterToolCtx(context.Background(), name, args)
}

// runConverterToolCtx runs a converter tool and kills it when ctx is cancelled. ffmpeg and
// ImageMagick go through the hardened launcher (converter_hardening.go).
func runConverterToolCtx(ctx context.Context, name string, args []string) error {
	if isHardenedTool(name) {
		return runHardenedTool(ctx, name, args)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	setProcessGroup(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			// An honest, actionable reason instead of the bare exec error: the job ends
			// `failed` naming the missing tool (the runtime image must ship it).
			return &converterToolError{fmt.Errorf("converter tool %q is not installed on this server (not found on PATH): %w", name, err)}
		}
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
// entries are checked, and every kept permission is applied inside the private directory, before
// the first is moved, so a refusal or a failed chmod publishes nothing. The moves themselves are
// separate rename(2) calls: if one fails, the outputs renamed before it stay published (their
// previous files are already replaced and cannot be restored); the error names how many were
// published (review round 9, F4).
func publishConverterOutputs(priv, dir string, roots []string) error {
	entries, err := os.ReadDir(priv)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("converter produced no output")
	}
	// keep[name] is the permission of a regular file the output replaces. Rename puts a new
	// inode in place, written by the tool under its own umask; the replaced file's permission
	// bits are copied onto it (owner, ACL and xattr are not, review round 8, item 5).
	keep := map[string]os.FileMode{}
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
		if fi, err := os.Lstat(dst); err == nil && fi.Mode().IsRegular() {
			keep[e.Name()] = fi.Mode().Perm()
		}
	}
	for _, e := range entries {
		if perm, ok := keep[e.Name()]; ok {
			if err := os.Chmod(filepath.Join(priv, e.Name()), perm); err != nil {
				return err
			}
		}
	}
	for i, e := range entries {
		if err := os.Rename(filepath.Join(priv, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return fmt.Errorf("publishing converter output %q failed after %d of %d outputs were published: %w",
				e.Name(), i, len(entries), err)
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
	return s.conversionRequestProblem(request) == ""
}

// conversionExtAliases maps equivalent spellings of one format.
var conversionExtAliases = map[string]string{"jpeg": "jpg", "tif": "tiff", "htm": "html", "text": "txt"}

func canonicalFormat(f string) string {
	f = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(f), "."))
	if a, ok := conversionExtAliases[f]; ok {
		return a
	}
	return f
}

// conversionRequestProblem returns "" for a valid request, else a short client-safe reason (never
// a filesystem path).
func (s *ConversionService) conversionRequestProblem(request *models.ConversionRequest) string {
	if request.SourcePath == "" || request.TargetPath == "" {
		return "source_path and target_path are required"
	}

	// Sanitize paths: reject path traversal and null bytes
	for _, p := range []string{request.SourcePath, request.TargetPath} {
		if strings.Contains(p, "\x00") {
			return "path not permitted"
		}
		cleaned := filepath.Clean(p)
		if strings.Contains(cleaned, "..") {
			return "path not permitted"
		}
	}
	// Absolute paths must stay inside the configured conversion roots; an
	// authenticated caller must not read /etc/passwd or write ~/.ssh (§11.4.115).
	if confineConversionPaths(request.SourcePath, request.TargetPath) != nil {
		return "path not permitted"
	}

	if request.SourceFormat == "" || request.TargetFormat == "" {
		return "source_format and target_format are required"
	}

	if request.ConversionType == "" {
		return "conversion_type is required"
	}

	if !s.isValidConversionType(request.ConversionType) {
		return "unknown conversion_type"
	}

	if !s.isSupportedFormat(request.ConversionType, request.SourceFormat, request.TargetFormat) {
		return fmt.Sprintf("conversion %s -> %s (%s) is not supported on this server", strings.ToLower(request.SourceFormat), strings.ToLower(request.TargetFormat), request.ConversionType)
	}

	// D10: the target file's extension must agree with the declared target format.
	ext := canonicalFormat(filepath.Ext(request.TargetPath))
	if ext != canonicalFormat(request.TargetFormat) {
		return "target_path extension does not match target_format"
	}

	return ""
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
		return s.isFormatSupported(sourceFormat, formats.Document.Input) && s.isFormatSupported(targetFormat, formats.Document.Output) &&
			documentPairSupported(sourceFormat, targetFormat)
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

// isPDFNativeConversion reports a pdf source whose target the PDF path itself can produce.
func (s *ConversionService) isPDFNativeConversion(job *models.ConversionJob) bool {
	if canonicalFormat(job.SourceFormat) != "pdf" {
		return false
	}
	switch canonicalFormat(job.TargetFormat) {
	case "txt", "html", "jpg", "png", "bmp", "tiff", "gif":
		return true
	}
	return false
}

func (s *ConversionService) isPDFConversion(job *models.ConversionJob) bool {
	return job.SourceFormat == "pdf" || job.TargetFormat == "pdf"
}

func (s *ConversionService) GetJobQueue() ([]models.ConversionJob, error) {
	return s.conversionRepo.GetDueJobs(time.Now(), 100)
}

// ProcessJobQueue runs one dispatch pass (what RunWorker does on every tick) and returns once
// the jobs it started have finished.
func (s *ConversionService) ProcessJobQueue() error {
	if _, err := s.GetJobQueue(); err != nil {
		return err
	}
	var wg sync.WaitGroup
	s.dispatchPending(context.Background(), &wg)
	wg.Wait()
	return nil
}
