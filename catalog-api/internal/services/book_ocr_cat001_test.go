package services

// CAT-001: performOCRSpace/performTesseract returned canned text ("Sample OCR text from book cover or page", "Tesseract OCR result") for
// any input, which then fed recognizeByMetadata as if it were the cover's title and author. The OCR must read the actual image
// (the real tesseract binary here) and fail honestly when it cannot.

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func renderText(t *testing.T, text string) []byte {
	t.Helper()
	small := image.NewRGBA(image.Rect(0, 0, 14*len(text)+20, 30))
	draw.Draw(small, small.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	d := &font.Drawer{Dst: small, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13, Dot: fixed.P(10, 20)}
	d.DrawString(text)
	// scale x8 so tesseract can read the 7x13 bitmap font
	big := image.NewRGBA(image.Rect(0, 0, small.Bounds().Dx()*8, small.Bounds().Dy()*8))
	for y := 0; y < big.Bounds().Dy(); y++ {
		for x := 0; x < big.Bounds().Dx(); x++ {
			big.Set(x, y, small.At(x/8, y/8))
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, big))
	return buf.Bytes()
}

func TestCAT001_Tesseract_ReadsTheRealImage(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("hardware/tool not present: tesseract binary (honest SKIP, tool_not_present)")
	}
	p := NewBookRecognitionProvider(zap.NewNop())
	res, err := p.performTesseract(context.Background(), renderText(t, "HELLO WORLD"))
	require.NoError(t, err)
	assert.Contains(t, strings.ToUpper(res.Text), "HELLO")
	assert.NotContains(t, res.Text, "Tesseract OCR result")
}

func TestCAT001_Tesseract_GarbageInputIsAnErrorNotCannedText(t *testing.T) {
	p := NewBookRecognitionProvider(zap.NewNop())
	res, err := p.performTesseract(context.Background(), []byte("not an image"))
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestCAT001_OCRSpace_UnconfiguredIsAnErrorAndNeverCallsOut(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	p := NewBookRecognitionProvider(zap.NewNop())
	p.baseURLs["tesseract"] = srv.URL
	res, err := p.performOCRSpace(context.Background(), []byte("whatever"))
	assert.ErrorIs(t, err, ErrOCRNotConfigured, "the placeholder key 'free_api_key' is not a credential")
	assert.Nil(t, res)
	assert.False(t, called, "no request may be sent with a placeholder key")
}

// Unit-class (the OCR.space service is a test double here): with a configured key the response text is what the service returned.
func TestCAT001_OCRSpace_ConfiguredKeyUsesTheServiceAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "real-key", r.Header.Get("apikey"))
		_, _ = w.Write([]byte(`{"ParsedResults":[{"ParsedText":"MOBY DICK"}],"IsErroredOnProcessing":false}`))
	}))
	defer srv.Close()
	p := NewBookRecognitionProvider(zap.NewNop())
	p.baseURLs["tesseract"] = srv.URL
	p.apiKeys["ocr_space"] = "real-key"
	res, err := p.performOCRSpace(context.Background(), []byte("img"))
	require.NoError(t, err)
	assert.Equal(t, "MOBY DICK", res.Text)
}

func TestCAT001_PerformOCR_NoEngineAvailableIsAnError(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no tesseract
	p := NewBookRecognitionProvider(zap.NewNop())
	res, err := p.performOCR(context.Background(), renderText(t, "HELLO"))
	assert.Error(t, err)
	assert.Nil(t, res)
}
