package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// fakeTesseract installs a tiny shell script named "tesseract" as the only PATH
// entry. It exercises the wrapper's handling of the process output (empty,
// oversized); the OCR itself is covered by the real-binary tests.
func fakeTesseract(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "tesseract"), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	t.Setenv("PATH", dir+":/usr/bin:/bin")
}

// F7: when no engine is configured performOCR must say so (ErrOCRNotConfigured),
// not the generic "OCR processing failed".
func TestCAT001_PerformOCR_PropagatesNotConfigured(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no tesseract
	p := NewBookRecognitionProvider(zap.NewNop())
	res, err := p.performOCR(context.Background(), []byte("img"))
	assert.ErrorIs(t, err, ErrOCRNotConfigured)
	assert.Nil(t, res)
}

// F11/M12: a tesseract run that succeeds but prints nothing is an error.
func TestCAT001_Tesseract_EmptyOutputIsAnError(t *testing.T) {
	fakeTesseract(t, "exit 0")
	p := NewBookRecognitionProvider(zap.NewNop())
	res, err := p.performTesseract(context.Background(), []byte("img"))
	assert.Error(t, err)
	assert.Nil(t, res)
}

// F8: stdout is bounded; a runaway process cannot exhaust memory.
func TestCAT001_Tesseract_OutputIsBounded(t *testing.T) {
	fakeTesseract(t, "head -c 20000000 /dev/zero | tr '\\0' 'A'")
	p := NewBookRecognitionProvider(zap.NewNop())
	res, err := p.performTesseract(context.Background(), []byte("img"))
	assert.ErrorIs(t, err, ErrOCROutputTooLarge)
	assert.Nil(t, res)
}
