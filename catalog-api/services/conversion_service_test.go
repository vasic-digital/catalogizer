package services

import (
	"fmt"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
)

func TestConversionService_GetSupportedFormats(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	formats := service.GetSupportedFormats()

	assert.NotNil(t, formats)
	// Verify that supported formats struct has entries in each category
	assert.Greater(t, len(formats.Video.Input), 0)
	assert.Greater(t, len(formats.Audio.Input), 0)
	assert.Greater(t, len(formats.Document.Input), 0)
	assert.Greater(t, len(formats.Image.Input), 0)
}

func TestConversionService_ValidateConversionRequest(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name   string
		input  *models.ConversionRequest
		expect bool
	}{
		{
			name: "empty request",
			input: &models.ConversionRequest{
				SourcePath:     "",
				TargetPath:     "",
				SourceFormat:   "",
				TargetFormat:   "",
				ConversionType: "",
			},
			expect: false,
		},
		{
			name: "missing source path",
			input: &models.ConversionRequest{
				SourcePath:     "",
				TargetPath:     "/output/video.mp4",
				SourceFormat:   "avi",
				TargetFormat:   "mp4",
				ConversionType: "video",
			},
			expect: false,
		},
		{
			name: "missing target path",
			input: &models.ConversionRequest{
				SourcePath:     "/input/video.avi",
				TargetPath:     "",
				SourceFormat:   "avi",
				TargetFormat:   "mp4",
				ConversionType: "video",
			},
			expect: false,
		},
		{
			name: "missing source format",
			input: &models.ConversionRequest{
				SourcePath:     "/input/video.avi",
				TargetPath:     "/output/video.mp4",
				SourceFormat:   "",
				TargetFormat:   "mp4",
				ConversionType: "video",
			},
			expect: false,
		},
		{
			name: "missing target format",
			input: &models.ConversionRequest{
				SourcePath:     "/input/video.avi",
				TargetPath:     "/output/video.mp4",
				SourceFormat:   "avi",
				TargetFormat:   "",
				ConversionType: "video",
			},
			expect: false,
		},
		{
			name: "missing conversion type",
			input: &models.ConversionRequest{
				SourcePath:     "/input/video.avi",
				TargetPath:     "/output/video.mp4",
				SourceFormat:   "avi",
				TargetFormat:   "mp4",
				ConversionType: "",
			},
			expect: false,
		},
		{
			name: "invalid conversion type",
			input: &models.ConversionRequest{
				SourcePath:     "/input/video.avi",
				TargetPath:     "/output/video.mp4",
				SourceFormat:   "avi",
				TargetFormat:   "mp4",
				ConversionType: "unknown",
			},
			expect: false,
		},
		{
			name: "unsupported format",
			input: &models.ConversionRequest{
				SourcePath:     "/input/video.xyz",
				TargetPath:     "/output/video.abc",
				SourceFormat:   "xyz",
				TargetFormat:   "abc",
				ConversionType: "video",
			},
			expect: false,
		},
		{
			name: "valid video conversion",
			input: &models.ConversionRequest{
				SourcePath:     "/input/video.avi",
				TargetPath:     "/output/video.mp4",
				SourceFormat:   "avi",
				TargetFormat:   "mp4",
				ConversionType: "video",
			},
			expect: true,
		},
		{
			name: "valid audio conversion",
			input: &models.ConversionRequest{
				SourcePath:     "/input/audio.wav",
				TargetPath:     "/output/audio.mp3",
				SourceFormat:   "wav",
				TargetFormat:   "mp3",
				ConversionType: "audio",
			},
			expect: true,
		},
		{
			name: "valid image conversion",
			input: &models.ConversionRequest{
				SourcePath:     "/input/image.png",
				TargetPath:     "/output/image.jpg",
				SourceFormat:   "png",
				TargetFormat:   "jpg",
				ConversionType: "image",
			},
			expect: true,
		},
		{
			name: "valid document conversion",
			input: &models.ConversionRequest{
				SourcePath:     "/input/doc.pdf",
				TargetPath:     "/output/doc.txt",
				SourceFormat:   "pdf",
				TargetFormat:   "txt",
				ConversionType: "document",
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.validateConversionRequest(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestConversionService_IsValidConversionType(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name           string
		conversionType string
		expected       bool
	}{
		{
			name:           "valid video conversion",
			conversionType: "video",
			expected:       true,
		},
		{
			name:           "valid audio conversion",
			conversionType: "audio",
			expected:       true,
		},
		{
			name:           "valid image conversion",
			conversionType: "image",
			expected:       true,
		},
		{
			name:           "valid document conversion",
			conversionType: "document",
			expected:       true,
		},
		{
			name:           "invalid conversion type",
			conversionType: "unknown",
			expected:       false,
		},
		{
			name:           "empty conversion type",
			conversionType: "",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isValidConversionType(tt.conversionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConversionService_IsSupportedFormat(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name           string
		sourceFormat   string
		targetFormat   string
		conversionType string
		expected       bool
	}{
		{
			name:           "mp4 video format",
			sourceFormat:   "mp4",
			targetFormat:   "mp4",
			conversionType: "video",
			expected:       true,
		},
		{
			name:           "mp3 audio format",
			sourceFormat:   "mp3",
			targetFormat:   "mp3",
			conversionType: "audio",
			expected:       true,
		},
		{
			name:           "unsupported format",
			sourceFormat:   "xyz",
			targetFormat:   "xyz",
			conversionType: "video",
			expected:       false,
		},
		{
			name:           "jpg image format",
			sourceFormat:   "jpg",
			targetFormat:   "png",
			conversionType: "image",
			expected:       true,
		},
		{
			name:           "png image format uppercase",
			sourceFormat:   "PNG",
			targetFormat:   "JPG",
			conversionType: "image",
			expected:       true,
		},
		{
			name:           "epub document format (needs calibre on the server)",
			sourceFormat:   "epub",
			targetFormat:   "pdf",
			conversionType: "document",
			expected:       converterPresent("ebook-convert"),
		},
		{
			name:           "source supported target unsupported video",
			sourceFormat:   "mp4",
			targetFormat:   "xyz",
			conversionType: "video",
			expected:       false,
		},
		{
			name:           "source unsupported target supported video",
			sourceFormat:   "xyz",
			targetFormat:   "mp4",
			conversionType: "video",
			expected:       false,
		},
		{
			name:           "invalid conversion type",
			sourceFormat:   "mp4",
			targetFormat:   "mp4",
			conversionType: "invalid",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isSupportedFormat(tt.conversionType, tt.sourceFormat, tt.targetFormat)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConversionService_BuildFFmpegVideoArgs(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name     string
		job      *models.ConversionJob
		expected []string
	}{
		{
			name: "basic video conversion medium quality",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "medium",
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "/output/video.mp4"},
		},
		{
			name: "low quality video",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "low",
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "28", "-preset", "fast", "/output/video.mp4"},
		},
		{
			name: "high quality video",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "high",
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "18", "-preset", "slow", "/output/video.mp4"},
		},
		{
			name: "lossless quality video",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "lossless",
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "0", "-preset", "veryslow", "/output/video.mp4"},
		},
		{
			name: "default quality when unknown",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "unknown",
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "/output/video.mp4"},
		},
		{
			name: "with resolution setting",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "medium",
				Settings:     ptr(`{"resolution": "1920x1080"}`),
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "-s", "1920x1080", "/output/video.mp4"},
		},
		{
			name: "with framerate setting",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "medium",
				Settings:     ptr(`{"framerate": "30"}`),
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "-r", "30", "/output/video.mp4"},
		},
		{
			name: "with bitrate setting",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "medium",
				Settings:     ptr(`{"bitrate": "2M"}`),
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "-b:v", "2M", "/output/video.mp4"},
		},
		{
			name: "with all settings",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "medium",
				Settings:     ptr(`{"resolution": "1280x720", "framerate": "24", "bitrate": "1M"}`),
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "-s", "1280x720", "-r", "24", "-b:v", "1M", "/output/video.mp4"},
		},
		{
			name: "invalid settings json",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "medium",
				Settings:     ptr(`invalid json`),
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "/output/video.mp4"},
		},
		{
			name: "nil settings",
			job: &models.ConversionJob{
				SourcePath:   "/input/video.avi",
				TargetPath:   "/output/video.mp4",
				TargetFormat: "mp4",
				Quality:      "medium",
				Settings:     nil,
			},
			expected: []string{"-i", "/input/video.avi", "-y", "-crf", "23", "-preset", "medium", "/output/video.mp4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := service.buildFFmpegVideoArgs(tt.job)
			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestConversionService_BuildFFmpegAudioArgs(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name     string
		job      *models.ConversionJob
		expected []string
	}{
		{
			name: "basic audio conversion medium quality",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "medium",
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "192k", "/output/audio.mp3"},
		},
		{
			name: "low quality audio",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "low",
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "96k", "/output/audio.mp3"},
		},
		{
			name: "high quality audio",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "high",
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "320k", "/output/audio.mp3"},
		},
		{
			name: "lossless quality audio (flac)",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.flac",
				TargetFormat: "flac",
				Quality:      "lossless",
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-c:a", "flac", "/output/audio.flac"},
		},
		{
			name: "default quality when unknown",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "unknown",
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "192k", "/output/audio.mp3"},
		},
		{
			name: "with sample rate setting",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "medium",
				Settings:     ptr(`{"sample_rate": "44100"}`),
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "192k", "-ar", "44100", "/output/audio.mp3"},
		},
		{
			name: "with channels setting",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "medium",
				Settings:     ptr(`{"channels": "2"}`),
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "192k", "-ac", "2", "/output/audio.mp3"},
		},
		{
			name: "with both settings",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "medium",
				Settings:     ptr(`{"sample_rate": "48000", "channels": "1"}`),
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "192k", "-ar", "48000", "-ac", "1", "/output/audio.mp3"},
		},
		{
			name: "invalid settings json",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "medium",
				Settings:     ptr(`invalid json`),
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "192k", "/output/audio.mp3"},
		},
		{
			name: "nil settings",
			job: &models.ConversionJob{
				SourcePath:   "/input/audio.wav",
				TargetPath:   "/output/audio.mp3",
				TargetFormat: "mp3",
				Quality:      "medium",
				Settings:     nil,
			},
			expected: []string{"-i", "/input/audio.wav", "-y", "-ab", "192k", "/output/audio.mp3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := service.buildFFmpegAudioArgs(tt.job)
			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestConversionService_BuildImageMagickArgs(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name           string
		job            *models.ConversionJob
		expectedArgLen int
	}{
		{
			name: "basic image conversion",
			job: &models.ConversionJob{
				SourcePath: "/input/image.png",
				TargetPath: "/output/image.jpg",
			},
			expectedArgLen: 2,
		},
		{
			name: "with resize setting",
			job: &models.ConversionJob{
				SourcePath: "/input/image.png",
				TargetPath: "/output/image.jpg",
				Settings:   ptr(`{"resize": "1920x1080"}`),
			},
			expectedArgLen: 4,
		},
		{
			name: "with quality setting",
			job: &models.ConversionJob{
				SourcePath: "/input/image.png",
				TargetPath: "/output/image.jpg",
				Settings:   ptr(`{"quality": "90"}`),
			},
			expectedArgLen: 4,
		},
		{
			name: "with compress setting",
			job: &models.ConversionJob{
				SourcePath: "/input/image.png",
				TargetPath: "/output/image.jpg",
				Settings:   ptr(`{"compress": true}`),
			},
			expectedArgLen: 4,
		},
		{
			name: "with all settings",
			job: &models.ConversionJob{
				SourcePath: "/input/image.png",
				TargetPath: "/output/image.jpg",
				Settings:   ptr(`{"resize": "800x600", "quality": "85", "compress": true}`),
			},
			expectedArgLen: 8,
		},
		{
			name: "invalid settings json",
			job: &models.ConversionJob{
				SourcePath: "/input/image.png",
				TargetPath: "/output/image.jpg",
				Settings:   ptr(`invalid json`),
			},
			expectedArgLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := service.buildImageMagickArgs(tt.job)
			assert.Len(t, args, tt.expectedArgLen)
		})
	}
}

func TestConversionService_IsEbookConversion(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name         string
		job          *models.ConversionJob
		expectedBool bool
	}{
		{"epub to mobi", &models.ConversionJob{SourceFormat: "epub", TargetFormat: "mobi"}, true},
		{"mobi to epub", &models.ConversionJob{SourceFormat: "mobi", TargetFormat: "epub"}, true},
		{"pdf to epub", &models.ConversionJob{SourceFormat: "pdf", TargetFormat: "epub"}, true},
		{"epub to pdf", &models.ConversionJob{SourceFormat: "epub", TargetFormat: "pdf"}, true},
		{"epub to txt", &models.ConversionJob{SourceFormat: "epub", TargetFormat: "txt"}, true},
		{"txt to epub", &models.ConversionJob{SourceFormat: "txt", TargetFormat: "epub"}, true},
		{"pdf to jpg", &models.ConversionJob{SourceFormat: "pdf", TargetFormat: "jpg"}, false},
		{"mp4 to webm", &models.ConversionJob{SourceFormat: "mp4", TargetFormat: "webm"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isEbookConversion(tt.job)
			assert.Equal(t, tt.expectedBool, result)
		})
	}
}

func TestConversionService_IsPDFConversion(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name         string
		job          *models.ConversionJob
		expectedBool bool
	}{
		{"pdf to jpg", &models.ConversionJob{SourceFormat: "pdf", TargetFormat: "jpg"}, true},
		{"pdf to png", &models.ConversionJob{SourceFormat: "pdf", TargetFormat: "png"}, true},
		{"pdf to txt", &models.ConversionJob{SourceFormat: "pdf", TargetFormat: "txt"}, true},
		{"pdf to html", &models.ConversionJob{SourceFormat: "pdf", TargetFormat: "html"}, true},
		{"jpg to pdf", &models.ConversionJob{SourceFormat: "jpg", TargetFormat: "pdf"}, true},
		{"epub to pdf", &models.ConversionJob{SourceFormat: "epub", TargetFormat: "pdf"}, true},
		{"doc to pdf", &models.ConversionJob{SourceFormat: "doc", TargetFormat: "pdf"}, true},
		{"mp4 to webm", &models.ConversionJob{SourceFormat: "mp4", TargetFormat: "webm"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isPDFConversion(tt.job)
			assert.Equal(t, tt.expectedBool, result)
		})
	}
}

// ---------------------------------------------------------------------------
// convertDocument routing logic tests
// ---------------------------------------------------------------------------

func TestConversionService_ConvertDocument_EbookRoute(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	// convertDocument routes to convertEbook when isEbookConversion returns true.
	// Since ebook-convert is not installed in test, the exec.Command will fail,
	// but we verify it attempted the ebook route (not the PDF route or default).
	job := &models.ConversionJob{
		SourcePath:   "/nonexistent/book.epub",
		TargetPath:   "/nonexistent/book.mobi",
		SourceFormat: "epub",
		TargetFormat: "mobi",
	}

	err := service.convertDocument(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ebook conversion failed")
}

func TestConversionService_ConvertDocument_PDFRoute(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	// When source or target is pdf and it's not an ebook conversion,
	// convertDocument routes to convertPDF.
	// pdf -> jpg is isPDFConversion=true but isEbookConversion=false
	job := &models.ConversionJob{
		SourcePath:   "/nonexistent/doc.pdf",
		TargetPath:   "/nonexistent/doc.jpg",
		SourceFormat: "pdf",
		TargetFormat: "jpg",
	}

	err := service.convertDocument(job)
	assert.Error(t, err)
	// This goes to convertPDFToImage which uses go-fitz, and will fail opening nonexistent file
	assert.Contains(t, err.Error(), "failed to open PDF")
}

func TestConversionService_ConvertDocument_UnsupportedRoute(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	// When neither ebook nor PDF conversion conditions are met,
	// convertDocument returns "unsupported document conversion"
	job := &models.ConversionJob{
		SourcePath:   "/nonexistent/doc.docx",
		TargetPath:   "/nonexistent/doc.odt",
		SourceFormat: "docx",
		TargetFormat: "odt",
	}

	err := service.convertDocument(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported document conversion")
}

// ---------------------------------------------------------------------------
// convertPDF routing tests
// ---------------------------------------------------------------------------

func TestConversionService_ConvertPDF_UnsupportedTarget(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	// convertPDF determines target format from file extension.
	// An unsupported extension returns an error.
	job := &models.ConversionJob{
		SourcePath:   "/nonexistent/doc.pdf",
		TargetPath:   "/nonexistent/doc.xyz",
		SourceFormat: "pdf",
		TargetFormat: "xyz",
	}

	err := service.convertPDF(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported PDF conversion target format")
}

func TestConversionService_ConvertPDF_TextRoute(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		SourcePath:   "/nonexistent/doc.pdf",
		TargetPath:   "/nonexistent/doc.txt",
		SourceFormat: "pdf",
		TargetFormat: "txt",
	}

	err := service.convertPDF(job)
	assert.Error(t, err)
	// Goes through convertPDFToText which opens file
	assert.Contains(t, err.Error(), "failed to open PDF")
}

func TestConversionService_ConvertPDF_HTMLRoute(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		SourcePath:   "/nonexistent/doc.pdf",
		TargetPath:   "/nonexistent/doc.html",
		SourceFormat: "pdf",
		TargetFormat: "html",
	}

	err := service.convertPDF(job)
	// convertPDFToHTML tries pandoc, then libreoffice, then text fallback.
	// All will fail but it may return an error from the text conversion fallback.
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// processConversion dispatch tests
// ---------------------------------------------------------------------------

func TestConversionService_ProcessConversion_VideoDispatch(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		ConversionType: models.ConversionTypeVideo,
		SourcePath:     "/nonexistent/video.avi",
		TargetPath:     "/nonexistent/video.mp4",
		TargetFormat:   "mp4",
		Quality:        "medium",
	}

	// processConversion calls convertVideo which exec.Command("ffmpeg"...).
	// Without ffmpeg, it panics or returns error. Since handleConversionError
	// tries to use conversionRepo (nil), it will panic, but processConversion
	// has a recover() that also calls handleConversionError, creating a double panic.
	// We test this through direct convertVideo call instead.
	err := service.convertVideo(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ffmpeg video conversion failed")
}

func TestConversionService_ProcessConversion_AudioDispatch(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		ConversionType: models.ConversionTypeAudio,
		SourcePath:     "/nonexistent/audio.wav",
		TargetPath:     "/nonexistent/audio.mp3",
		TargetFormat:   "mp3",
		Quality:        "medium",
	}

	err := service.convertAudio(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ffmpeg audio conversion failed")
}

func TestConversionService_ProcessConversion_ImageDispatch(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		ConversionType: models.ConversionTypeImage,
		SourcePath:     "/nonexistent/image.png",
		TargetPath:     "/nonexistent/image.jpg",
	}

	err := service.convertImage(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "imagemagick conversion failed")
}

func TestConversionService_ProcessConversion_DocumentDispatch(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		ConversionType: models.ConversionTypeDocument,
		SourcePath:     "/nonexistent/book.epub",
		TargetPath:     "/nonexistent/book.mobi",
		SourceFormat:   "epub",
		TargetFormat:   "mobi",
	}

	err := service.convertDocument(job)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// convertEbook settings tests
// ---------------------------------------------------------------------------

func TestConversionService_ConvertEbook_WithSettings(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	tests := []struct {
		name     string
		settings *string
	}{
		{
			name:     "with preserve_cover and preserve_metadata",
			settings: ptr(`{"preserve_cover": true, "preserve_metadata": true}`),
		},
		{
			name:     "with invalid json settings",
			settings: ptr(`invalid json`),
		},
		{
			name:     "with nil settings",
			settings: nil,
		},
		{
			name:     "with empty settings",
			settings: ptr(`{}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.ConversionJob{
				SourcePath:   "/nonexistent/book.epub",
				TargetPath:   "/nonexistent/book.mobi",
				SourceFormat: "epub",
				TargetFormat: "mobi",
				Settings:     tt.settings,
			}

			err := service.convertEbook(job)
			// All fail because ebook-convert is not installed
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "ebook conversion failed")
		})
	}
}

// ---------------------------------------------------------------------------
// handleConversionSuccess and handleConversionError field manipulation tests
// ---------------------------------------------------------------------------

func TestConversionService_HandleConversionSuccess_FieldUpdates(t *testing.T) {
	// handleConversionSuccess sets Status, CompletedAt, Duration, then calls
	// conversionRepo.UpdateJob (which will panic with nil repo). We use recover
	// to verify the field updates were made before the panic.
	service := NewConversionService(nil, nil, nil)

	startTime := time.Now().Add(-5 * time.Minute)
	job := &models.ConversionJob{
		ID:        1,
		UserID:    1,
		Status:    models.ConversionStatusRunning,
		StartedAt: &startTime,
	}

	// Expect panic from nil conversionRepo.UpdateJob
	assert.Panics(t, func() {
		service.handleConversionSuccess(job)
	})

	// Verify field updates happened before the panic
	assert.Equal(t, models.ConversionStatusCompleted, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.NotNil(t, job.Duration)
	assert.True(t, *job.Duration > 0)
}

func TestConversionService_HandleConversionSuccess_NoStartedAt(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		ID:        1,
		UserID:    1,
		Status:    models.ConversionStatusRunning,
		StartedAt: nil, // no start time
	}

	assert.Panics(t, func() {
		service.handleConversionSuccess(job)
	})

	assert.Equal(t, models.ConversionStatusCompleted, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.Nil(t, job.Duration) // Duration not set when StartedAt is nil
}

func TestConversionService_HandleConversionError_FieldUpdates(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	startTime := time.Now().Add(-3 * time.Minute)
	job := &models.ConversionJob{
		ID:        2,
		UserID:    1,
		Status:    models.ConversionStatusRunning,
		StartedAt: &startTime,
	}

	convErr := fmt.Errorf("test conversion error")

	assert.Panics(t, func() {
		service.handleConversionError(job, convErr)
	})

	assert.Equal(t, models.ConversionStatusFailed, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.NotNil(t, job.ErrorMessage)
	assert.Equal(t, "test conversion error", *job.ErrorMessage)
	assert.NotNil(t, job.Duration)
	assert.True(t, *job.Duration > 0)
}

func TestConversionService_HandleConversionError_NoStartedAt(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	job := &models.ConversionJob{
		ID:        3,
		UserID:    1,
		Status:    models.ConversionStatusRunning,
		StartedAt: nil,
	}

	convErr := fmt.Errorf("another error")

	assert.Panics(t, func() {
		service.handleConversionError(job, convErr)
	})

	assert.Equal(t, models.ConversionStatusFailed, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.NotNil(t, job.ErrorMessage)
	assert.Equal(t, "another error", *job.ErrorMessage)
	assert.Nil(t, job.Duration) // Duration not set when StartedAt is nil
}

// ---------------------------------------------------------------------------
// notifyUser test (pure output, no side effects beyond Printf)
// ---------------------------------------------------------------------------

func TestConversionService_NotifyUser(t *testing.T) {
	service := NewConversionService(nil, nil, nil)

	// notifyUser just prints to stdout. Verify it does not panic.
	job := &models.ConversionJob{
		ID:     1,
		UserID: 42,
	}

	assert.NotPanics(t, func() {
		service.notifyUser(job, "Test notification")
	})
}

func TestConversionService_ConvertPDFToImage_InvalidPath(t *testing.T) {
	service := NewConversionService(nil, nil, nil)
	job := &models.ConversionJob{
		SourcePath: "/nonexistent/file.pdf",
		TargetPath: "/tmp/output.png",
	}
	err := service.convertPDFToImage(job, "png")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open PDF")
}

func TestConversionService_ConvertPDFToText_InvalidPath(t *testing.T) {
	service := NewConversionService(nil, nil, nil)
	job := &models.ConversionJob{
		SourcePath: "/nonexistent/file.pdf",
		TargetPath: "/tmp/output.txt",
	}
	err := service.convertPDFToText(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open PDF")
}

func TestConversionService_ConvertPDFWithImageMagick_InvalidPath(t *testing.T) {
	service := NewConversionService(nil, nil, nil)
	job := &models.ConversionJob{
		SourcePath: "/nonexistent/file.pdf",
		TargetPath: "/tmp/output.bmp",
	}
	err := service.convertPDFWithImageMagick(job, "bmp")
	// Either error (no ImageMagick) or exec error — both are non-nil
	assert.Error(t, err)
}

func TestConversionService_ConvertPDFToImage_WithSettings(t *testing.T) {
	service := NewConversionService(nil, nil, nil)
	settings := `{"page": 0, "dpi": 300}`
	job := &models.ConversionJob{
		SourcePath: "/nonexistent/file.pdf",
		TargetPath: "/tmp/output.png",
		Settings:   &settings,
	}
	err := service.convertPDFToImage(job, "png")
	assert.Error(t, err) // file doesn't exist, but settings parsing path is covered
}

func ptr(s string) *string {
	return &s
}
