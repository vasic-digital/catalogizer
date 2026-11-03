package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// RecognizeMedia — test the top-level recognition entrypoint
// =============================================================================

func TestBookRecognitionProvider_RecognizeMedia_WithTitleAndAuthor(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	// Point Google Books at a mock server returning valid results
	googleResp := GoogleBooksResponse{
		TotalItems: 1,
		Items: []GoogleBookItem{
			{
				ID: "gb_001",
				VolumeInfo: GoogleBookVolumeInfo{
					Title:         "Clean Code",
					Authors:       []string{"Robert C. Martin"},
					Publisher:     "Prentice Hall",
					PublishedDate: "2008",
					Description:   "A handbook of agile software craftsmanship",
					PrintType:     "BOOK",
					AverageRating: 4.5,
					RatingsCount:  200,
					Language:      "en",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	req := &MediaRecognitionRequest{
		FilePath:  "/books/Robert C. Martin - Clean Code.epub",
		FileName:  "Robert C. Martin - Clean Code.epub",
		MediaType: MediaTypeBook,
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Clean Code", result.Title)
	assert.Equal(t, "Robert C. Martin", result.Author)
	assert.Equal(t, "Google Books", result.APIProvider)
}

func TestBookRecognitionProvider_RecognizeMedia_ByISBN(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	googleResp := GoogleBooksResponse{
		TotalItems: 1,
		Items: []GoogleBookItem{
			{
				ID: "gb_isbn_001",
				VolumeInfo: GoogleBookVolumeInfo{
					Title:     "Test ISBN Book",
					Authors:   []string{"ISBN Author"},
					PrintType: "BOOK",
					IndustryIdentifiers: []GoogleBookIdentifier{
						{Type: "ISBN_13", Identifier: "9780134685991"},
					},
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	req := &MediaRecognitionRequest{
		FilePath:  "/books/9780134685991.pdf",
		FileName:  "ISBN 978-0-13-468599-1 Book.pdf",
		MediaType: MediaTypeBook,
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBookRecognitionProvider_RecognizeMedia_FallbackToBasic(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	// Point all APIs at server that returns empty results
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL
	provider.baseURLs["open_library"] = ts.URL
	provider.baseURLs["crossref"] = ts.URL

	req := &MediaRecognitionRequest{
		FilePath:  "/books/unknown_book.pdf",
		FileName:  "unknown_book.pdf",
		MediaType: MediaTypeBook,
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "filename_parsing", result.RecognitionMethod)
	assert.Equal(t, "basic", result.APIProvider)
}

func TestBookRecognitionProvider_RecognizeMedia_WithImageSample(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	// The OCR mock implementations return mock data, so we can test the image path
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL
	provider.baseURLs["open_library"] = ts.URL
	provider.baseURLs["crossref"] = ts.URL

	req := &MediaRecognitionRequest{
		FilePath:    "/books/scanned_cover.jpg",
		FileName:    "scanned_cover.jpg",
		MediaType:   MediaTypeBook,
		ImageSample: []byte("fake image data"),
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBookRecognitionProvider_RecognizeMedia_WithTextSample(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL
	provider.baseURLs["open_library"] = ts.URL
	provider.baseURLs["crossref"] = ts.URL

	req := &MediaRecognitionRequest{
		FilePath:   "/books/sample.epub",
		FileName:   "sample.epub",
		MediaType:  MediaTypeBook,
		TextSample: "Chapter 1: Introduction\nThis is a sample book about testing. The quick brown fox.",
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
}

// =============================================================================
// performOCR — OCR pipeline coverage
// =============================================================================

// CAT-001: the former tests asserted the canned OCR text. OCR behaviour is now covered by book_ocr_cat001_test.go (real tesseract,
// honest errors); here only the pipeline's refusal on an unreadable image is kept.
func TestBookRecognitionProvider_PerformOCR_UnreadableImageIsAnError(t *testing.T) {
	provider := NewBookRecognitionProvider(zap.NewNop())
	result, err := provider.performOCR(context.Background(), []byte("fake image"))
	require.Error(t, err)
	assert.Nil(t, result)
}

// =============================================================================
// searchGoogleBooks — with mock HTTP server
// =============================================================================

func TestBookRecognitionProvider_SearchGoogleBooks_Success(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	googleResp := GoogleBooksResponse{
		TotalItems: 1,
		Items: []GoogleBookItem{
			{
				ID: "gb_test",
				VolumeInfo: GoogleBookVolumeInfo{
					Title:         "Test Book Title",
					Authors:       []string{"Test Author"},
					PrintType:     "BOOK",
					AverageRating: 4.0,
					RatingsCount:  100,
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params are URL-encoded, so check RawQuery for the decoded form
		q := r.URL.Query().Get("q")
		assert.Contains(t, q, "intitle:")
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	result, err := provider.searchGoogleBooks(context.Background(), "Test Book Title", "Test Author")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Test Book Title", result.Title)
}

func TestBookRecognitionProvider_SearchGoogleBooks_NoResults(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	googleResp := GoogleBooksResponse{
		TotalItems: 0,
		Items:      []GoogleBookItem{},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	_, err := provider.searchGoogleBooks(context.Background(), "Nonexistent", "Nobody")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no books found")
}

func TestBookRecognitionProvider_SearchGoogleBooks_HTTPError(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	provider.baseURLs["google_books"] = "http://localhost:1"

	_, err := provider.searchGoogleBooks(context.Background(), "Test", "Author")
	assert.Error(t, err)
}

func TestBookRecognitionProvider_SearchGoogleBooks_TitleOnly(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	googleResp := GoogleBooksResponse{
		TotalItems: 1,
		Items: []GoogleBookItem{
			{
				ID:         "gb_title_only",
				VolumeInfo: GoogleBookVolumeInfo{Title: "Title Only", PrintType: "BOOK"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	result, err := provider.searchGoogleBooks(context.Background(), "Title Only", "")
	require.NoError(t, err)
	assert.Equal(t, "Title Only", result.Title)
}

// =============================================================================
// searchOpenLibrary — with mock HTTP server
// =============================================================================

func TestBookRecognitionProvider_SearchOpenLibrary_Success(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	olResp := OpenLibrarySearchResponse{
		NumFound: 1,
		Docs: []OpenLibraryDocument{
			{
				Key:              "/works/OL_test",
				Title:            "OpenLib Test Book",
				AuthorName:       []string{"OL Author"},
				FirstPublishYear: 2020,
				EditionCount:     5,
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(olResp)
	}))
	defer ts.Close()

	provider.baseURLs["open_library"] = ts.URL

	result, err := provider.searchOpenLibrary(context.Background(), "OpenLib Test Book", "OL Author")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "OpenLib Test Book", result.Title)
	assert.Equal(t, "Open Library", result.APIProvider)
}

func TestBookRecognitionProvider_SearchOpenLibrary_NoResults(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	olResp := OpenLibrarySearchResponse{
		NumFound: 0,
		Docs:     []OpenLibraryDocument{},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(olResp)
	}))
	defer ts.Close()

	provider.baseURLs["open_library"] = ts.URL

	_, err := provider.searchOpenLibrary(context.Background(), "Nonexistent", "")
	assert.Error(t, err)
}

func TestBookRecognitionProvider_SearchOpenLibrary_HTTPError(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	provider.baseURLs["open_library"] = "http://localhost:1"

	_, err := provider.searchOpenLibrary(context.Background(), "Test", "Author")
	assert.Error(t, err)
}

// =============================================================================
// searchCrossref — with mock HTTP server
// =============================================================================

func TestBookRecognitionProvider_SearchCrossref_Success(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	crossrefResp := CrossrefResponse{
		Status: "ok",
		Message: CrossrefMessage{
			TotalResults: 1,
			Items: []CrossrefWork{
				{
					DOI:       "10.1234/test",
					Type:      "journal-article",
					Title:     []string{"A Research Paper"},
					Publisher: "Test Publisher",
					Author: []CrossrefAuthor{
						{Given: "John", Family: "Doe"},
					},
					Issued: CrossrefPartialDate{
						DateParts: [][]int{{2023, 6}},
					},
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(crossrefResp)
	}))
	defer ts.Close()

	provider.baseURLs["crossref"] = ts.URL

	result, err := provider.searchCrossref(context.Background(), "A Research Paper", "John Doe")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "A Research Paper", result.Title)
	assert.Equal(t, "Crossref", result.APIProvider)
}

func TestBookRecognitionProvider_SearchCrossref_NoResults(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	crossrefResp := CrossrefResponse{
		Status: "ok",
		Message: CrossrefMessage{
			TotalResults: 0,
			Items:        []CrossrefWork{},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(crossrefResp)
	}))
	defer ts.Close()

	provider.baseURLs["crossref"] = ts.URL

	_, err := provider.searchCrossref(context.Background(), "Nonexistent", "")
	assert.Error(t, err)
}

func TestBookRecognitionProvider_SearchCrossref_HTTPError(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	provider.baseURLs["crossref"] = "http://localhost:1"

	_, err := provider.searchCrossref(context.Background(), "Test", "Author")
	assert.Error(t, err)
}

// =============================================================================
// searchGoogleBooksByISBN — with mock HTTP server
// =============================================================================

func TestBookRecognitionProvider_SearchGoogleBooksByISBN_Success(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	googleResp := GoogleBooksResponse{
		TotalItems: 1,
		Items: []GoogleBookItem{
			{
				ID:         "gb_isbn",
				VolumeInfo: GoogleBookVolumeInfo{Title: "ISBN Book", PrintType: "BOOK"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		assert.Contains(t, q, "isbn:")
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	result, err := provider.searchGoogleBooksByISBN(context.Background(), "9780134685991")
	require.NoError(t, err)
	assert.Equal(t, "ISBN Book", result.Title)
}

func TestBookRecognitionProvider_SearchGoogleBooksByISBN_NoResults(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(GoogleBooksResponse{TotalItems: 0, Items: []GoogleBookItem{}})
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	_, err := provider.searchGoogleBooksByISBN(context.Background(), "0000000000")
	assert.Error(t, err)
}

// =============================================================================
// searchOpenLibraryByISBN — with mock HTTP server
// =============================================================================

func TestBookRecognitionProvider_SearchOpenLibraryByISBN_Success(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	olResp := OpenLibrarySearchResponse{
		NumFound: 1,
		Docs: []OpenLibraryDocument{
			{
				Key:   "/works/OL_isbn",
				Title: "ISBN OL Book",
				ISBN:  []string{"9780134685991"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(olResp)
	}))
	defer ts.Close()

	provider.baseURLs["open_library"] = ts.URL

	result, err := provider.searchOpenLibraryByISBN(context.Background(), "9780134685991")
	require.NoError(t, err)
	assert.Equal(t, "ISBN OL Book", result.Title)
}

// =============================================================================
// recognizeByISBN — tests the ISBN search pipeline
// =============================================================================

func TestBookRecognitionProvider_RecognizeByISBN_GoogleBooksSuccess(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	googleResp := GoogleBooksResponse{
		TotalItems: 1,
		Items: []GoogleBookItem{
			{
				ID:         "gb_isbn_rec",
				VolumeInfo: GoogleBookVolumeInfo{Title: "ISBN Recognized", PrintType: "BOOK"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	result, err := provider.recognizeByISBN(context.Background(), "978-0-13-468599-1")
	require.NoError(t, err)
	assert.Equal(t, "ISBN Recognized", result.Title)
}

func TestBookRecognitionProvider_RecognizeByISBN_FallbackToOpenLibrary(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	// Google returns no results
	tsGoogle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(GoogleBooksResponse{TotalItems: 0})
	}))
	defer tsGoogle.Close()

	// OpenLibrary returns a result
	olResp := OpenLibrarySearchResponse{
		NumFound: 1,
		Docs: []OpenLibraryDocument{
			{Key: "/works/OL_fallback", Title: "OL Fallback Book"},
		},
	}
	tsOL := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(olResp)
	}))
	defer tsOL.Close()

	provider.baseURLs["google_books"] = tsGoogle.URL
	provider.baseURLs["open_library"] = tsOL.URL

	result, err := provider.recognizeByISBN(context.Background(), "9780134685991")
	require.NoError(t, err)
	assert.Equal(t, "OL Fallback Book", result.Title)
}

// =============================================================================
// recognizeByMetadata — tests the metadata search pipeline
// =============================================================================

func TestBookRecognitionProvider_RecognizeByMetadata_GoogleBooksSuccess(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	googleResp := GoogleBooksResponse{
		TotalItems: 1,
		Items: []GoogleBookItem{
			{
				ID:         "gb_meta",
				VolumeInfo: GoogleBookVolumeInfo{Title: "Metadata Book", PrintType: "BOOK"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleResp)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL

	result, err := provider.recognizeByMetadata(context.Background(), "Metadata Book", "Author")
	require.NoError(t, err)
	assert.Equal(t, "Metadata Book", result.Title)
}

func TestBookRecognitionProvider_RecognizeByMetadata_AllFail(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["google_books"] = ts.URL
	provider.baseURLs["open_library"] = ts.URL
	provider.baseURLs["crossref"] = ts.URL

	_, err := provider.recognizeByMetadata(context.Background(), "None", "Nobody")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no results found")
}

// =============================================================================
// extractBookInfoFromOCR — tests OCR book info extraction
// =============================================================================

func TestBookRecognitionProvider_ExtractBookInfoFromOCR_WithISBN(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	ocrResult := &OCRResponse{
		Text:       "ISBN 978-3-16-148410-0 The Great Book",
		Confidence: 0.9,
		Blocks: []OCRTextBlock{
			{Text: "The Great Book", Confidence: 0.95, BoundingBox: OCRBoundingBox{Width: 300, Height: 100}},
			{Text: "John Smith", Confidence: 0.85, BoundingBox: OCRBoundingBox{Width: 100, Height: 30}},
		},
	}

	metadata := provider.extractBookInfoFromOCR(ocrResult)
	require.NotNil(t, metadata)
	assert.NotEmpty(t, metadata.ISBN)
}

func TestBookRecognitionProvider_ExtractBookInfoFromOCR_NoData(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	ocrResult := &OCRResponse{
		Text:       "",
		Confidence: 0.1,
		Blocks:     nil,
	}

	metadata := provider.extractBookInfoFromOCR(ocrResult)
	assert.Nil(t, metadata)
}

// =============================================================================
// analyzeBookContent — tests text analysis
// =============================================================================

func TestBookRecognitionProvider_AnalyzeBookContent(t *testing.T) {
	logger := zap.NewNop()
	provider := NewBookRecognitionProvider(logger)

	text := "The Great Book Title That Is Long Enough\nChapter 1: Introduction\nThis is a test. Another sentence here. The quick brown fox jumps over the lazy dog.\n\nChapter 2: Main Content\nMore content follows here. Testing sentences."

	analysis, err := provider.analyzeBookContent(context.Background(), text)
	require.NoError(t, err)
	require.NotNil(t, analysis)
	assert.Greater(t, analysis.WordCount, 0)
	assert.Greater(t, analysis.CharacterCount, 0)
	assert.Greater(t, analysis.ParagraphCount, 0)
	assert.NotEmpty(t, analysis.Language)
}
