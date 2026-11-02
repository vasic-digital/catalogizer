package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"catalogizer/internal/models"

	"go.uber.org/zap"
)

// Book and publication recognition provider with OCR and metadata APIs
type BookRecognitionProvider struct {
	logger      *zap.Logger
	httpClient  *http.Client
	baseURLs    map[string]string
	apiKeys     map[string]string
	rateLimiter map[string]*time.Ticker
}

// Google Books API structures
type GoogleBooksResponse struct {
	Kind       string           `json:"kind"`
	TotalItems int              `json:"totalItems"`
	Items      []GoogleBookItem `json:"items"`
}

type GoogleBookItem struct {
	Kind       string               `json:"kind"`
	ID         string               `json:"id"`
	ETag       string               `json:"etag"`
	SelfLink   string               `json:"selfLink"`
	VolumeInfo GoogleBookVolumeInfo `json:"volumeInfo"`
	SaleInfo   GoogleBookSaleInfo   `json:"saleInfo"`
	AccessInfo GoogleBookAccessInfo `json:"accessInfo"`
	SearchInfo GoogleBookSearchInfo `json:"searchInfo,omitempty"`
}

type GoogleBookVolumeInfo struct {
	Title               string                 `json:"title"`
	Subtitle            string                 `json:"subtitle,omitempty"`
	Authors             []string               `json:"authors,omitempty"`
	Publisher           string                 `json:"publisher,omitempty"`
	PublishedDate       string                 `json:"publishedDate,omitempty"`
	Description         string                 `json:"description,omitempty"`
	IndustryIdentifiers []GoogleBookIdentifier `json:"industryIdentifiers,omitempty"`
	ReadingModes        GoogleBookReadingModes `json:"readingModes"`
	PageCount           int                    `json:"pageCount,omitempty"`
	PrintType           string                 `json:"printType"`
	Categories          []string               `json:"categories,omitempty"`
	AverageRating       float64                `json:"averageRating,omitempty"`
	RatingsCount        int                    `json:"ratingsCount,omitempty"`
	MaturityRating      string                 `json:"maturityRating"`
	AllowAnonLogging    bool                   `json:"allowAnonLogging"`
	ContentVersion      string                 `json:"contentVersion"`
	PanelizationSummary GoogleBookPanelization `json:"panelizationSummary,omitempty"`
	ImageLinks          GoogleBookImageLinks   `json:"imageLinks,omitempty"`
	Language            string                 `json:"language"`
	PreviewLink         string                 `json:"previewLink"`
	InfoLink            string                 `json:"infoLink"`
	CanonicalVolumeLink string                 `json:"canonicalVolumeLink"`
	SeriesInfo          GoogleBookSeriesInfo   `json:"seriesInfo,omitempty"`
}

type GoogleBookIdentifier struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

type GoogleBookReadingModes struct {
	Text  bool `json:"text"`
	Image bool `json:"image"`
}

type GoogleBookPanelization struct {
	ContainsEpubBubbles  bool `json:"containsEpubBubbles"`
	ContainsImageBubbles bool `json:"containsImageBubbles"`
}

type GoogleBookImageLinks struct {
	SmallThumbnail string `json:"smallThumbnail,omitempty"`
	Thumbnail      string `json:"thumbnail,omitempty"`
	Small          string `json:"small,omitempty"`
	Medium         string `json:"medium,omitempty"`
	Large          string `json:"large,omitempty"`
	ExtraLarge     string `json:"extraLarge,omitempty"`
}

type GoogleBookSaleInfo struct {
	Country     string            `json:"country"`
	Saleability string            `json:"saleability"`
	IsEbook     bool              `json:"isEbook"`
	ListPrice   GoogleBookPrice   `json:"listPrice,omitempty"`
	RetailPrice GoogleBookPrice   `json:"retailPrice,omitempty"`
	BuyLink     string            `json:"buyLink,omitempty"`
	Offers      []GoogleBookOffer `json:"offers,omitempty"`
}

type GoogleBookPrice struct {
	Amount       float64 `json:"amount"`
	CurrencyCode string  `json:"currencyCode"`
}

type GoogleBookOffer struct {
	FinskyOfferType int             `json:"finskyOfferType"`
	ListPrice       GoogleBookPrice `json:"listPrice"`
	RetailPrice     GoogleBookPrice `json:"retailPrice"`
	GiftablePrice   GoogleBookPrice `json:"giftablePrice,omitempty"`
}

type GoogleBookAccessInfo struct {
	Country                string                       `json:"country"`
	Viewability            string                       `json:"viewability"`
	Embeddable             bool                         `json:"embeddable"`
	PublicDomain           bool                         `json:"publicDomain"`
	TextToSpeechPermission string                       `json:"textToSpeechPermission"`
	Epub                   GoogleBookFormatAvailability `json:"epub"`
	PDF                    GoogleBookFormatAvailability `json:"pdf"`
	WebReaderLink          string                       `json:"webReaderLink,omitempty"`
	AccessViewStatus       string                       `json:"accessViewStatus"`
	QuoteSharingAllowed    bool                         `json:"quoteSharingAllowed"`
}

type GoogleBookFormatAvailability struct {
	IsAvailable  bool   `json:"isAvailable"`
	AcsTokenLink string `json:"acsTokenLink,omitempty"`
}

type GoogleBookSearchInfo struct {
	TextSnippet string `json:"textSnippet"`
}

type GoogleBookSeriesInfo struct {
	Kind                 string `json:"kind"`
	BookDisplayNumber    string `json:"bookDisplayNumber"`
	VolumeDisplayNumber  string `json:"volumeDisplayNumber"`
	ShortSeriesBookTitle string `json:"shortSeriesBookTitle"`
}

// Open Library API structures
type OpenLibrarySearchResponse struct {
	NumFound      int                   `json:"numFound"`
	Start         int                   `json:"start"`
	NumFoundExact bool                  `json:"numFoundExact"`
	Docs          []OpenLibraryDocument `json:"docs"`
}

type OpenLibraryDocument struct {
	Key                   string    `json:"key"`
	Type                  string    `json:"type"`
	Seed                  []string  `json:"seed,omitempty"`
	Title                 string    `json:"title"`
	TitleSuggest          string    `json:"title_suggest,omitempty"`
	TitleSort             string    `json:"title_sort,omitempty"`
	Subtitle              string    `json:"subtitle,omitempty"`
	AlternativeTitle      []string  `json:"alternative_title,omitempty"`
	AlternativeSubtitle   []string  `json:"alternative_subtitle,omitempty"`
	Edition               []string  `json:"edition_name,omitempty"`
	FullTitle             string    `json:"full_title,omitempty"`
	AuthorKey             []string  `json:"author_key,omitempty"`
	AuthorName            []string  `json:"author_name,omitempty"`
	AuthorAlternativeName []string  `json:"author_alternative_name,omitempty"`
	AuthorFacet           []string  `json:"author_facet,omitempty"`
	ContributorName       []string  `json:"contributor,omitempty"`
	Subject               []string  `json:"subject,omitempty"`
	SubjectKey            []string  `json:"subject_key,omitempty"`
	SubjectFacet          []string  `json:"subject_facet,omitempty"`
	Place                 []string  `json:"place,omitempty"`
	PlaceKey              []string  `json:"place_key,omitempty"`
	PlaceFacet            []string  `json:"place_facet,omitempty"`
	Person                []string  `json:"person,omitempty"`
	PersonKey             []string  `json:"person_key,omitempty"`
	PersonFacet           []string  `json:"person_facet,omitempty"`
	Language              []string  `json:"language,omitempty"`
	PublisherName         []string  `json:"publisher,omitempty"`
	PublisherFacet        []string  `json:"publisher_facet,omitempty"`
	PublishDate           []string  `json:"publish_date,omitempty"`
	PublishYear           []int     `json:"publish_year,omitempty"`
	PublishPlace          []string  `json:"publish_place,omitempty"`
	FirstPublishYear      int       `json:"first_publish_year,omitempty"`
	NumberOfPagesMedian   int       `json:"number_of_pages_median,omitempty"`
	LccnSort              string    `json:"lccn_sort,omitempty"`
	EditionCount          int       `json:"edition_count"`
	EditionKey            []string  `json:"edition_key,omitempty"`
	PrintDisabled         []string  `json:"printdisabled,omitempty"`
	LendingEdition        string    `json:"lending_edition,omitempty"`
	LendingIdentifier     string    `json:"lending_identifier,omitempty"`
	ISBN                  []string  `json:"isbn,omitempty"`
	LastModified          time.Time `json:"last_modified_i"`
	EbookCount            int       `json:"ebook_count_i"`
	EbookAccess           string    `json:"ebook_access,omitempty"`
	HasFulltext           bool      `json:"has_fulltext"`
	PublicScan            bool      `json:"public_scan_b,omitempty"`
	CoverID               int       `json:"cover_i,omitempty"`
	CoverEditionKey       string    `json:"cover_edition_key,omitempty"`
	FirstSentence         []string  `json:"first_sentence,omitempty"`
	LCCN                  []string  `json:"lccn,omitempty"`
	OCLC                  []string  `json:"oclc,omitempty"`
	ContributorKey        []string  `json:"contributor_key,omitempty"`
	ID_Amazon             []string  `json:"id_amazon,omitempty"`
	ID_LibraryThing       []string  `json:"id_librarything,omitempty"`
	ID_Goodreads          []string  `json:"id_goodreads,omitempty"`
	ID_DepositoLegal      []string  `json:"id_dnb,omitempty"`
	ID_Wikisource         []string  `json:"id_wikisource,omitempty"`
}

// Crossref API structures (for academic publications)
type CrossrefResponse struct {
	Status  string          `json:"status"`
	Message CrossrefMessage `json:"message"`
}

type CrossrefMessage struct {
	Facets       map[string]interface{} `json:"facets,omitempty"`
	TotalResults int                    `json:"total-results"`
	Items        []CrossrefWork         `json:"items"`
	ItemsPerPage int                    `json:"items-per-page"`
	Query        map[string]interface{} `json:"query,omitempty"`
}

type CrossrefWork struct {
	Indexed             CrossrefDate           `json:"indexed"`
	ReferenceCount      int                    `json:"reference-count"`
	Publisher           string                 `json:"publisher"`
	Issue               string                 `json:"issue,omitempty"`
	License             []CrossrefLicense      `json:"license,omitempty"`
	Funder              []CrossrefFunder       `json:"funder,omitempty"`
	ContentDomain       CrossrefContentDomain  `json:"content-domain"`
	ShortContainerTitle []string               `json:"short-container-title,omitempty"`
	Published           CrossrefPartialDate    `json:"published,omitempty"`
	Abstract            string                 `json:"abstract,omitempty"`
	DOI                 string                 `json:"DOI"`
	Type                string                 `json:"type"`
	Created             CrossrefDate           `json:"created"`
	Page                string                 `json:"page,omitempty"`
	UpdatePolicy        string                 `json:"update-policy,omitempty"`
	Source              string                 `json:"source"`
	IsReferencedByCount int                    `json:"is-referenced-by-count"`
	Title               []string               `json:"title"`
	Prefix              string                 `json:"prefix"`
	Volume              string                 `json:"volume,omitempty"`
	Author              []CrossrefAuthor       `json:"author,omitempty"`
	Member              string                 `json:"member"`
	ContainerTitle      []string               `json:"container-title,omitempty"`
	OriginalTitle       []string               `json:"original-title,omitempty"`
	Language            string                 `json:"language,omitempty"`
	Link                []CrossrefLink         `json:"link,omitempty"`
	Deposited           CrossrefDate           `json:"deposited"`
	Score               float64                `json:"score"`
	Subtitle            []string               `json:"subtitle,omitempty"`
	ShortTitle          []string               `json:"short-title,omitempty"`
	Issued              CrossrefPartialDate    `json:"issued,omitempty"`
	ReferencesCount     int                    `json:"references-count"`
	JournalIssue        CrossrefJournalIssue   `json:"journal-issue,omitempty"`
	AlternativeID       []string               `json:"alternative-id,omitempty"`
	URL                 string                 `json:"URL,omitempty"`
	Relation            map[string]interface{} `json:"relation,omitempty"`
	ISSN                []string               `json:"ISSN,omitempty"`
	IssnType            []CrossrefIssnType     `json:"issn-type,omitempty"`
	Subject             []string               `json:"subject,omitempty"`
	PublishedOnline     CrossrefPartialDate    `json:"published-online,omitempty"`
	PublishedPrint      CrossrefPartialDate    `json:"published-print,omitempty"`
}

type CrossrefDate struct {
	DateParts [][]int   `json:"date-parts"`
	DateTime  time.Time `json:"date-time"`
	Timestamp int64     `json:"timestamp"`
}

type CrossrefPartialDate struct {
	DateParts [][]int `json:"date-parts"`
}

type CrossrefLicense struct {
	Start          CrossrefDate `json:"start"`
	ContentVersion string       `json:"content-version"`
	DelayInDays    int          `json:"delay-in-days"`
	URL            string       `json:"URL"`
}

type CrossrefFunder struct {
	DOI   string   `json:"DOI,omitempty"`
	Name  string   `json:"name"`
	Award []string `json:"award,omitempty"`
}

type CrossrefContentDomain struct {
	Domain               []string `json:"domain"`
	CrossmarkRestriction bool     `json:"crossmark-restriction"`
}

type CrossrefAuthor struct {
	ORCID       string                `json:"ORCID,omitempty"`
	Given       string                `json:"given,omitempty"`
	Family      string                `json:"family,omitempty"`
	Sequence    string                `json:"sequence"`
	Affiliation []CrossrefAffiliation `json:"affiliation,omitempty"`
}

type CrossrefAffiliation struct {
	Name string `json:"name"`
}

type CrossrefLink struct {
	URL                 string `json:"URL"`
	ContentType         string `json:"content-type"`
	ContentVersion      string `json:"content-version"`
	IntendedApplication string `json:"intended-application"`
}

type CrossrefJournalIssue struct {
	Issue           string              `json:"issue,omitempty"`
	PublishedOnline CrossrefPartialDate `json:"published-online,omitempty"`
	PublishedPrint  CrossrefPartialDate `json:"published-print,omitempty"`
}

type CrossrefIssnType struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

// OCR Service structures
type OCRRequest struct {
	ImageData []byte            `json:"image_data"`
	ImageURL  string            `json:"image_url,omitempty"`
	Language  string            `json:"language,omitempty"`
	Options   map[string]string `json:"options,omitempty"`
}

type OCRResponse struct {
	Text       string            `json:"text"`
	Confidence float64           `json:"confidence"`
	Language   string            `json:"language"`
	Blocks     []OCRTextBlock    `json:"blocks"`
	Words      []OCRWord         `json:"words"`
	Lines      []OCRLine         `json:"lines"`
	Metadata   map[string]string `json:"metadata"`
}

type OCRTextBlock struct {
	Text        string         `json:"text"`
	Confidence  float64        `json:"confidence"`
	BoundingBox OCRBoundingBox `json:"bounding_box"`
	Lines       []OCRLine      `json:"lines"`
}

type OCRLine struct {
	Text        string         `json:"text"`
	Confidence  float64        `json:"confidence"`
	BoundingBox OCRBoundingBox `json:"bounding_box"`
	Words       []OCRWord      `json:"words"`
}

type OCRWord struct {
	Text        string         `json:"text"`
	Confidence  float64        `json:"confidence"`
	BoundingBox OCRBoundingBox `json:"bounding_box"`
}

type OCRBoundingBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Text analysis for book content
type BookContentAnalysis struct {
	Language         string        `json:"language"`
	WordCount        int           `json:"word_count"`
	CharacterCount   int           `json:"character_count"`
	ParagraphCount   int           `json:"paragraph_count"`
	SentenceCount    int           `json:"sentence_count"`
	ReadabilityScore float64       `json:"readability_score"`
	Topics           []string      `json:"topics"`
	Keywords         []string      `json:"keywords"`
	Entities         []NamedEntity `json:"entities"`
	Metadata         BookMetadata  `json:"metadata"`
}

type NamedEntity struct {
	Text       string  `json:"text"`
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	StartPos   int     `json:"start_pos"`
	EndPos     int     `json:"end_pos"`
}

type BookMetadata struct {
	Title           string              `json:"title"`
	Author          string              `json:"author"`
	Publisher       string              `json:"publisher"`
	PublicationDate string              `json:"publication_date"`
	ISBN            string              `json:"isbn"`
	Edition         string              `json:"edition"`
	ChapterTitles   []string            `json:"chapter_titles"`
	TableOfContents []TOCEntry          `json:"table_of_contents"`
	Bibliography    []BibliographyEntry `json:"bibliography"`
	Index           []IndexEntry        `json:"index"`
}

type TOCEntry struct {
	Title      string     `json:"title"`
	PageNumber int        `json:"page_number"`
	Level      int        `json:"level"`
	Children   []TOCEntry `json:"children,omitempty"`
}

type BibliographyEntry struct {
	Citation string   `json:"citation"`
	Type     string   `json:"type"`
	Authors  []string `json:"authors"`
	Title    string   `json:"title"`
	Year     int      `json:"year"`
}

type IndexEntry struct {
	Term       string       `json:"term"`
	Pages      []int        `json:"pages"`
	SubEntries []IndexEntry `json:"sub_entries,omitempty"`
}

func NewBookRecognitionProvider(logger *zap.Logger) *BookRecognitionProvider {
	return &BookRecognitionProvider{
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURLs: map[string]string{
			"google_books":  "https://www.googleapis.com/books/v1",
			"open_library":  "https://openlibrary.org",
			"crossref":      "https://api.crossref.org",
			"worldcat":      "https://www.worldcat.org/webservices",
			"libgen":        "https://libgen.is/json.php",
			"archive_org":   "https://archive.org",
			"tesseract":     "https://api.ocr.space/parse",
			"google_vision": "https://vision.googleapis.com/v1",
		},
		apiKeys: map[string]string{
			"google_books":  "free_api_key",
			"google_vision": "free_api_key",
			"ocr_space":     "free_api_key",
		},
		rateLimiter: make(map[string]*time.Ticker),
	}
}

func (p *BookRecognitionProvider) RecognizeMedia(ctx context.Context, req *MediaRecognitionRequest) (*MediaRecognitionResult, error) {
	p.logger.Info("Starting book/publication recognition",
		zap.String("file_path", req.FilePath),
		zap.String("media_type", string(req.MediaType)))

	// Extract metadata from filename
	title, author, isbn := p.extractBookMetadataFromFilename(req.FileName)

	p.logger.Debug("Extracted metadata from filename",
		zap.String("title", title),
		zap.String("author", author),
		zap.String("isbn", isbn))

	// Try OCR if image sample provided
	if len(req.ImageSample) > 0 {
		if ocrResult, err := p.performOCR(ctx, req.ImageSample); err == nil {
			if bookInfo := p.extractBookInfoFromOCR(ocrResult); bookInfo != nil {
				title = bookInfo.Title
				author = bookInfo.Author
				isbn = bookInfo.ISBN
			}
		}
	}

	// Try text analysis if text sample provided
	if req.TextSample != "" {
		if analysis, err := p.analyzeBookContent(ctx, req.TextSample); err == nil {
			if analysis.Metadata.Title != "" {
				title = analysis.Metadata.Title
			}
			if analysis.Metadata.Author != "" {
				author = analysis.Metadata.Author
			}
			if analysis.Metadata.ISBN != "" {
				isbn = analysis.Metadata.ISBN
			}
		}
	}

	// Try different recognition methods based on available metadata
	if isbn != "" {
		if result, err := p.recognizeByISBN(ctx, isbn); err == nil {
			p.logger.Info("Successfully recognized by ISBN",
				zap.String("title", result.Title),
				zap.Float64("confidence", result.Confidence))
			return result, nil
		}
	}

	if title != "" || author != "" {
		if result, err := p.recognizeByMetadata(ctx, title, author); err == nil {
			p.logger.Info("Successfully recognized by metadata",
				zap.String("title", result.Title),
				zap.Float64("confidence", result.Confidence))
			return result, nil
		}
	}

	// Determine publication type
	mediaType := p.determinePublicationType(title, req.FileName, req.MimeType)

	// Fallback to basic recognition
	return p.basicBookRecognition(req, title, author, isbn, mediaType), nil
}

func (p *BookRecognitionProvider) performOCR(ctx context.Context, imageData []byte) (*OCRResponse, error) {
	var errs []error

	// Try OCR.space API first
	result, err := p.performOCRSpace(ctx, imageData)
	if err == nil {
		return result, nil
	}
	errs = append(errs, err)

	// Try Tesseract via local processing
	result, err = p.performTesseract(ctx, imageData)
	if err == nil {
		return result, nil
	}
	errs = append(errs, err)

	// Every engine unconfigured: say so, so callers can tell "not set up" from "tried and failed".
	allUnconfigured := true
	for _, e := range errs {
		if !errors.Is(e, ErrOCRNotConfigured) {
			allUnconfigured = false
		}
	}
	if allUnconfigured {
		return nil, fmt.Errorf("OCR processing failed: %w", ErrOCRNotConfigured)
	}
	return nil, fmt.Errorf("OCR processing failed: %w", errors.Join(errs...))
}

// ErrOCRNotConfigured is returned when an OCR engine has no usable credential or binary. OCR output is never fabricated (CAT-001).
var ErrOCRNotConfigured = errors.New("OCR engine not configured")

// performOCRSpace sends the image to OCR.space. The built-in key value "free_api_key" is a placeholder, not a credential: without a real
// key (set in apiKeys["ocr_space"]) the engine is reported as not configured instead of returning canned text. NOTE: the request/response
// handling below follows the OCR.space documentation and was NOT exercised against the live service (no key available in this session).
func (p *BookRecognitionProvider) performOCRSpace(ctx context.Context, imageData []byte) (*OCRResponse, error) {
	key := p.apiKeys["ocr_space"]
	if key == "" || key == "free_api_key" {
		return nil, fmt.Errorf("OCR.space: %w (no API key)", ErrOCRNotConfigured)
	}
	form := url.Values{}
	form.Set("base64Image", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(imageData))
	form.Set("language", "eng")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURLs["tesseract"], strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", key)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OCR.space request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OCR.space: unexpected status %s", resp.Status)
	}
	var out struct {
		ParsedResults []struct {
			ParsedText string `json:"ParsedText"`
		} `json:"ParsedResults"`
		IsErroredOnProcessing bool `json:"IsErroredOnProcessing"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("OCR.space response: %w", err)
	}
	if out.IsErroredOnProcessing || len(out.ParsedResults) == 0 || strings.TrimSpace(out.ParsedResults[0].ParsedText) == "" {
		return nil, errors.New("OCR.space returned no text")
	}
	return &OCRResponse{Text: out.ParsedResults[0].ParsedText, Language: "en"}, nil
}

// performTesseract runs the local tesseract binary on the image (stdin -> stdout). A missing binary, an unreadable image or empty output is
// an error, never canned text.
func (p *BookRecognitionProvider) performTesseract(ctx context.Context, imageData []byte) (*OCRResponse, error) {
	bin, err := exec.LookPath("tesseract")
	if err != nil {
		return nil, fmt.Errorf("tesseract: %w (binary not found)", ErrOCRNotConfigured)
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "stdin", "stdout", "-l", "eng")
	cmd.Stdin = bytes.NewReader(imageData)
	var stdout, stderr bytes.Buffer
	bounded := &boundedWriter{w: &stdout, remaining: maxTesseractOutputBytes}
	cmd.Stdout, cmd.Stderr = bounded, &limitedStderr{w: &stderr, remaining: 64 << 10}
	if err := cmd.Run(); err != nil {
		if bounded.exceeded {
			return nil, ErrOCROutputTooLarge
		}
		return nil, fmt.Errorf("tesseract failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if bounded.exceeded {
		return nil, ErrOCROutputTooLarge
	}
	text := strings.TrimSpace(stdout.String())
	if text == "" {
		return nil, errors.New("tesseract found no text")
	}
	return &OCRResponse{Text: text, Language: "en"}, nil
}

func (p *BookRecognitionProvider) extractBookInfoFromOCR(ocrResult *OCRResponse) *BookMetadata {
	text := ocrResult.Text

	// Extract title (usually the largest/most prominent text)
	title := p.extractTitleFromOCR(text, ocrResult.Blocks)

	// Extract author (often appears below title)
	author := p.extractAuthorFromOCR(text, ocrResult.Blocks)

	// Extract ISBN (if visible)
	isbn := p.extractISBNFromText(text)

	if title == "" && author == "" && isbn == "" {
		return nil
	}

	return &BookMetadata{
		Title:  title,
		Author: author,
		ISBN:   isbn,
	}
}

func (p *BookRecognitionProvider) analyzeBookContent(ctx context.Context, text string) (*BookContentAnalysis, error) {
	analysis := &BookContentAnalysis{
		Language:       p.detectLanguage(text),
		WordCount:      len(strings.Fields(text)),
		CharacterCount: len(text),
		ParagraphCount: len(strings.Split(text, "\n\n")),
		SentenceCount:  len(regexp.MustCompile(`[.!?]+`).FindAllString(text, -1)),
	}

	// Extract metadata from content
	analysis.Metadata = p.extractMetadataFromContent(text)

	// Extract topics and keywords
	analysis.Topics = p.extractTopics(text)
	analysis.Keywords = p.extractKeywords(text)

	// Calculate readability score
	analysis.ReadabilityScore = p.calculateReadabilityScore(text)

	return analysis, nil
}

func (p *BookRecognitionProvider) recognizeByISBN(ctx context.Context, isbn string) (*MediaRecognitionResult, error) {
	// Clean ISBN
	cleanISBN := p.cleanISBN(isbn)

	// Try Google Books API first
	if result, err := p.searchGoogleBooksByISBN(ctx, cleanISBN); err == nil {
		return result, nil
	}

	// Try Open Library as fallback
	if result, err := p.searchOpenLibraryByISBN(ctx, cleanISBN); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("no results found for ISBN: %s", cleanISBN)
}

func (p *BookRecognitionProvider) recognizeByMetadata(ctx context.Context, title, author string) (*MediaRecognitionResult, error) {
	// Try Google Books API first
	if result, err := p.searchGoogleBooks(ctx, title, author); err == nil {
		return result, nil
	}

	// Try Open Library as fallback
	if result, err := p.searchOpenLibrary(ctx, title, author); err == nil {
		return result, nil
	}

	// Try Crossref for academic publications
	if result, err := p.searchCrossref(ctx, title, author); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("no results found for title/author: %s / %s", title, author)
}

func (p *BookRecognitionProvider) searchGoogleBooks(ctx context.Context, title, author string) (*MediaRecognitionResult, error) {
	params := url.Values{}

	// Build search query
	query := ""
	if title != "" {
		query += fmt.Sprintf("intitle:%s", title)
	}
	if author != "" {
		if query != "" {
			query += "+"
		}
		query += fmt.Sprintf("inauthor:%s", author)
	}

	params.Set("q", query)
	params.Set("maxResults", "10")
	params.Set("printType", "books")
	params.Set("key", p.apiKeys["google_books"])

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/volumes?%s", p.baseURLs["google_books"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var booksResp GoogleBooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&booksResp); err != nil {
		return nil, err
	}

	if len(booksResp.Items) == 0 {
		return nil, fmt.Errorf("no books found in Google Books")
	}

	// Get the best match
	bestMatch := booksResp.Items[0]
	return p.convertGoogleBook(bestMatch), nil
}

func (p *BookRecognitionProvider) searchGoogleBooksByISBN(ctx context.Context, isbn string) (*MediaRecognitionResult, error) {
	params := url.Values{}
	params.Set("q", "isbn:"+isbn)
	params.Set("key", p.apiKeys["google_books"])

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/volumes?%s", p.baseURLs["google_books"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var booksResp GoogleBooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&booksResp); err != nil {
		return nil, err
	}

	if len(booksResp.Items) == 0 {
		return nil, fmt.Errorf("no books found for ISBN in Google Books")
	}

	return p.convertGoogleBook(booksResp.Items[0]), nil
}

func (p *BookRecognitionProvider) convertGoogleBook(book GoogleBookItem) *MediaRecognitionResult {
	volumeInfo := book.VolumeInfo

	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("google_books_%s", book.ID),
		MediaType:         p.determineBookType(volumeInfo),
		Title:             volumeInfo.Title,
		Description:       volumeInfo.Description,
		PageCount:         volumeInfo.PageCount,
		Language:          volumeInfo.Language,
		Rating:            volumeInfo.AverageRating,
		Confidence:        p.calculateGoogleBooksConfidence(volumeInfo.AverageRating, volumeInfo.RatingsCount),
		RecognitionMethod: "google_books_api",
		APIProvider:       "Google Books",
	}

	// Set subtitle if available
	if volumeInfo.Subtitle != "" {
		result.Title = fmt.Sprintf("%s: %s", volumeInfo.Title, volumeInfo.Subtitle)
	}

	// Set authors
	if len(volumeInfo.Authors) > 0 {
		result.Author = volumeInfo.Authors[0]
		for _, author := range volumeInfo.Authors {
			result.Authors = append(result.Authors, Person{
				Name: author,
				Role: "Author",
			})
		}
	}

	// Set publisher
	result.Publisher = volumeInfo.Publisher

	// Parse publication date
	if volumeInfo.PublishedDate != "" {
		result.Year = p.parseYear(volumeInfo.PublishedDate)
		if pubDate, err := p.parseDate(volumeInfo.PublishedDate); err == nil {
			result.ReleaseDate = &pubDate
		}
	}

	// Extract ISBN
	for _, identifier := range volumeInfo.IndustryIdentifiers {
		if identifier.Type == "ISBN_13" {
			result.ISBN13 = identifier.Identifier
			result.ISBN = identifier.Identifier
		} else if identifier.Type == "ISBN_10" {
			result.ISBN10 = identifier.Identifier
			if result.ISBN == "" {
				result.ISBN = identifier.Identifier
			}
		}
	}

	// Set categories as genres
	result.Genres = volumeInfo.Categories

	// Get cover images
	if volumeInfo.ImageLinks.Thumbnail != "" {
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			ID:      "google_books_thumbnail",
			URL:     volumeInfo.ImageLinks.Thumbnail,
			Quality: "medium",
			Source:  "Google Books",
		})
	}
	if volumeInfo.ImageLinks.SmallThumbnail != "" {
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			ID:      "google_books_small",
			URL:     volumeInfo.ImageLinks.SmallThumbnail,
			Quality: "small",
			Source:  "Google Books",
		})
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"google_books_id": book.ID,
		"preview_link":    volumeInfo.PreviewLink,
		"info_link":       volumeInfo.InfoLink,
	}

	return result
}

func (p *BookRecognitionProvider) searchOpenLibrary(ctx context.Context, title, author string) (*MediaRecognitionResult, error) {
	params := url.Values{}

	// Build search query
	query := ""
	if title != "" {
		query += fmt.Sprintf("title:%s", title)
	}
	if author != "" {
		if query != "" {
			query += " AND "
		}
		query += fmt.Sprintf("author:%s", author)
	}

	params.Set("q", query)
	params.Set("limit", "10")

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/search.json?%s", p.baseURLs["open_library"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp OpenLibrarySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	if len(searchResp.Docs) == 0 {
		return nil, fmt.Errorf("no books found in Open Library")
	}

	// Get the best match
	bestMatch := searchResp.Docs[0]
	return p.convertOpenLibraryBook(bestMatch), nil
}

func (p *BookRecognitionProvider) searchOpenLibraryByISBN(ctx context.Context, isbn string) (*MediaRecognitionResult, error) {
	params := url.Values{}
	params.Set("q", "isbn:"+isbn)
	params.Set("limit", "1")

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/search.json?%s", p.baseURLs["open_library"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp OpenLibrarySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	if len(searchResp.Docs) == 0 {
		return nil, fmt.Errorf("no books found for ISBN in Open Library")
	}

	return p.convertOpenLibraryBook(searchResp.Docs[0]), nil
}

func (p *BookRecognitionProvider) convertOpenLibraryBook(doc OpenLibraryDocument) *MediaRecognitionResult {
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("open_library_%s", strings.TrimPrefix(doc.Key, "/works/")),
		MediaType:         MediaTypeBook,
		Title:             doc.Title,
		PageCount:         doc.NumberOfPagesMedian,
		Confidence:        p.calculateOpenLibraryConfidence(doc.EditionCount, doc.EbookCount),
		RecognitionMethod: "open_library_api",
		APIProvider:       "Open Library",
	}

	// Set subtitle
	if doc.Subtitle != "" {
		result.Title = fmt.Sprintf("%s: %s", doc.Title, doc.Subtitle)
	}

	// Set authors
	if len(doc.AuthorName) > 0 {
		result.Author = doc.AuthorName[0]
		for _, author := range doc.AuthorName {
			result.Authors = append(result.Authors, Person{
				Name: author,
				Role: "Author",
			})
		}
	}

	// Set publisher
	if len(doc.PublisherName) > 0 {
		result.Publisher = doc.PublisherName[0]
	}

	// Set publication year
	if doc.FirstPublishYear > 0 {
		result.Year = doc.FirstPublishYear
	}

	// Set ISBN
	if len(doc.ISBN) > 0 {
		result.ISBN = doc.ISBN[0]
		// Determine ISBN-10 vs ISBN-13
		for _, isbn := range doc.ISBN {
			if len(isbn) == 10 {
				result.ISBN10 = isbn
			} else if len(isbn) == 13 {
				result.ISBN13 = isbn
			}
		}
	}

	// Set subjects as genres
	result.Genres = doc.Subject

	// Get cover image
	if doc.CoverID > 0 {
		coverURL := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", doc.CoverID)
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			ID:      fmt.Sprintf("open_library_%d", doc.CoverID),
			URL:     coverURL,
			Quality: "large",
			Source:  "Open Library",
		})
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"open_library_key": doc.Key,
	}
	if len(doc.ID_Goodreads) > 0 {
		result.ExternalIDs["goodreads_id"] = doc.ID_Goodreads[0]
	}

	return result
}

func (p *BookRecognitionProvider) searchCrossref(ctx context.Context, title, author string) (*MediaRecognitionResult, error) {
	params := url.Values{}

	// Build search query for academic publications
	query := ""
	if title != "" {
		query += title
	}
	if author != "" {
		if query != "" {
			query += " "
		}
		query += author
	}

	params.Set("query", query)
	params.Set("rows", "10")

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/works?%s", p.baseURLs["crossref"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var crossrefResp CrossrefResponse
	if err := json.NewDecoder(resp.Body).Decode(&crossrefResp); err != nil {
		return nil, err
	}

	if len(crossrefResp.Message.Items) == 0 {
		return nil, fmt.Errorf("no publications found in Crossref")
	}

	// Get the best match
	bestMatch := crossrefResp.Message.Items[0]
	return p.convertCrossrefWork(bestMatch), nil
}

func (p *BookRecognitionProvider) convertCrossrefWork(work CrossrefWork) *MediaRecognitionResult {
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("crossref_%s", work.DOI),
		MediaType:         p.mapCrossrefType(work.Type),
		DOI:               work.DOI,
		Publisher:         work.Publisher,
		Confidence:        work.Score,
		RecognitionMethod: "crossref_api",
		APIProvider:       "Crossref",
	}

	// Set title
	if len(work.Title) > 0 {
		result.Title = work.Title[0]
	}

	// Set authors
	if len(work.Author) > 0 {
		result.Author = fmt.Sprintf("%s %s", work.Author[0].Given, work.Author[0].Family)
		for _, author := range work.Author {
			fullName := fmt.Sprintf("%s %s", author.Given, author.Family)
			result.Authors = append(result.Authors, Person{
				Name: fullName,
				Role: "Author",
			})
		}
	}

	// Set publication date
	if len(work.Issued.DateParts) > 0 && len(work.Issued.DateParts[0]) > 0 {
		result.Year = work.Issued.DateParts[0][0]
	}

	// Set journal/container title
	if len(work.ContainerTitle) > 0 {
		result.Series = work.ContainerTitle[0]
	}

	// Set volume and issue
	if work.Volume != "" {
		if vol, err := strconv.Atoi(work.Volume); err == nil {
			result.Volume = vol
		}
	}
	if work.Issue != "" {
		if issue, err := strconv.Atoi(work.Issue); err == nil {
			result.Issue = issue
		}
	}

	// Set ISSN
	if len(work.ISSN) > 0 {
		result.ISSN = work.ISSN[0]
	}

	// Set subjects as genres
	result.Genres = work.Subject

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"doi": work.DOI,
	}
	if work.URL != "" {
		result.ExternalIDs["url"] = work.URL
	}

	return result
}

func (p *BookRecognitionProvider) basicBookRecognition(req *MediaRecognitionRequest, title, author, isbn string, mediaType MediaType) *MediaRecognitionResult {
	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("basic_book_%s_%d", strings.ReplaceAll(title, " ", "_"), time.Now().Unix()),
		MediaType:         mediaType,
		Title:             title,
		Author:            author,
		ISBN:              isbn,
		Confidence:        0.3,
		RecognitionMethod: "filename_parsing",
		APIProvider:       "basic",
		ExternalIDs:       make(map[string]string),
	}
}

// Helper methods
func (p *BookRecognitionProvider) extractBookMetadataFromFilename(filename string) (title, author, isbn string) {
	// Remove file extension
	baseName := strings.TrimSuffix(filename, "."+p.getFileExtension(filename))

	// Extract ISBN pattern
	isbnPattern := regexp.MustCompile(`\b(?:ISBN[-\s]*(?:10|13)?[-\s]*[:\s]?)?(?:97[89][-\s]?)?(?:\d[-\s]?){9}[\dXx]\b`)
	if matches := isbnPattern.FindString(baseName); matches != "" {
		isbn = p.cleanISBN(matches)
		// Remove ISBN from filename
		baseName = isbnPattern.ReplaceAllString(baseName, "")
	}

	// Common patterns for books:
	// Author - Title
	// Title - Author
	// Author (Year) Title
	// Title by Author

	// Pattern: "by Author"
	byPattern := regexp.MustCompile(`\s+by\s+(.+)$`)
	if matches := byPattern.FindStringSubmatch(baseName); len(matches) > 1 {
		author = strings.TrimSpace(matches[1])
		title = strings.TrimSpace(byPattern.ReplaceAllString(baseName, ""))
		return title, author, isbn
	}

	// Pattern: Author - Title or Title - Author
	if parts := strings.Split(baseName, " - "); len(parts) >= 2 {
		// Try to determine which is author and which is title
		if p.looksLikeAuthorName(parts[0]) {
			author = strings.TrimSpace(parts[0])
			title = strings.TrimSpace(parts[1])
		} else {
			title = strings.TrimSpace(parts[0])
			author = strings.TrimSpace(parts[1])
		}
		return title, author, isbn
	}

	// Pattern: Author (Year) Title
	yearPattern := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)\s*(.+)$`)
	if matches := yearPattern.FindStringSubmatch(baseName); len(matches) == 4 {
		author = strings.TrimSpace(matches[1])
		title = strings.TrimSpace(matches[3])
		return title, author, isbn
	}

	// Fallback: use entire filename as title
	title = baseName
	return title, author, isbn
}

func (p *BookRecognitionProvider) looksLikeAuthorName(str string) bool {
	// Simple heuristic: author names often have 2-3 words and proper capitalization
	words := strings.Fields(str)
	if len(words) < 2 || len(words) > 4 {
		return false
	}

	// Check if words look like names (capitalized)
	for _, word := range words {
		if len(word) > 0 && strings.ToUpper(word[:1]) != word[:1] {
			return false
		}
	}

	return true
}

func (p *BookRecognitionProvider) extractTitleFromOCR(text string, blocks []OCRTextBlock) string {
	// Find the largest text block (usually the title)
	var largestBlock OCRTextBlock
	maxArea := 0

	for _, block := range blocks {
		area := block.BoundingBox.Width * block.BoundingBox.Height
		if area > maxArea && block.Confidence > 0.8 {
			maxArea = area
			largestBlock = block
		}
	}

	if largestBlock.Text != "" {
		return strings.TrimSpace(largestBlock.Text)
	}

	// Fallback: extract first line that looks like a title
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 5 && len(line) < 100 && !strings.Contains(line, "ISBN") {
			return line
		}
	}

	return ""
}

func (p *BookRecognitionProvider) extractAuthorFromOCR(text string, blocks []OCRTextBlock) string {
	// Look for "by" pattern
	byPattern := regexp.MustCompile(`(?i)by\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)+)`)
	if matches := byPattern.FindStringSubmatch(text); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Look for author-like text blocks (smaller than title, proper case)
	for _, block := range blocks {
		if block.Confidence > 0.7 && p.looksLikeAuthorName(block.Text) {
			return strings.TrimSpace(block.Text)
		}
	}

	return ""
}

func (p *BookRecognitionProvider) extractISBNFromText(text string) string {
	isbnPattern := regexp.MustCompile(`\b(?:ISBN[-\s]*(?:10|13)?[-\s]*[:\s]?)?(?:97[89][-\s]?)?(?:\d[-\s]?){9}[\dXx]\b`)
	if match := isbnPattern.FindString(text); match != "" {
		return p.cleanISBN(match)
	}
	return ""
}

func (p *BookRecognitionProvider) cleanISBN(isbn string) string {
	// Remove all non-digit characters except X
	cleaned := regexp.MustCompile(`[^\dXx]`).ReplaceAllString(isbn, "")
	return strings.ToUpper(cleaned)
}

func (p *BookRecognitionProvider) extractMetadataFromContent(text string) BookMetadata {
	metadata := BookMetadata{}

	// Extract title from first significant line
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 && len(line) < 100 {
			metadata.Title = line
			break
		}
	}

	// Extract chapter titles
	chapterPattern := regexp.MustCompile(`(?i)^(?:chapter|ch\.?)\s+\d+[\.\:\s]+(.+)$`)
	for _, line := range lines {
		if matches := chapterPattern.FindStringSubmatch(strings.TrimSpace(line)); len(matches) > 1 {
			metadata.ChapterTitles = append(metadata.ChapterTitles, matches[1])
		}
	}

	return metadata
}

func (p *BookRecognitionProvider) detectLanguage(text string) string {
	// Simple language detection based on common words
	// This would be replaced with a proper language detection library

	englishWords := []string{"the", "and", "of", "to", "a", "in", "is", "it", "you", "that"}
	spanishWords := []string{"el", "la", "de", "que", "y", "en", "un", "es", "se", "no"}
	frenchWords := []string{"le", "de", "et", "à", "un", "il", "être", "et", "en", "avoir"}

	lowerText := strings.ToLower(text)

	englishCount := 0
	for _, word := range englishWords {
		englishCount += strings.Count(lowerText, " "+word+" ")
	}

	spanishCount := 0
	for _, word := range spanishWords {
		spanishCount += strings.Count(lowerText, " "+word+" ")
	}

	frenchCount := 0
	for _, word := range frenchWords {
		frenchCount += strings.Count(lowerText, " "+word+" ")
	}

	if englishCount >= spanishCount && englishCount >= frenchCount {
		return "en"
	} else if spanishCount >= frenchCount {
		return "es"
	} else {
		return "fr"
	}
}

func (p *BookRecognitionProvider) extractTopics(text string) []string {
	// Simple topic extraction - would be replaced with NLP library
	topics := []string{}

	// Look for capitalized phrases that might be topics
	topicPattern := regexp.MustCompile(`\b[A-Z][a-z]+(?:\s+[A-Z][a-z]+)*\b`)
	matches := topicPattern.FindAllString(text, -1)

	topicCount := make(map[string]int)
	for _, match := range matches {
		if len(match) > 3 { // Filter out short matches
			topicCount[match]++
		}
	}

	// Return most frequent topics
	for topic, count := range topicCount {
		if count > 1 { // Topic appears multiple times
			topics = append(topics, topic)
		}
	}

	return topics
}

func (p *BookRecognitionProvider) extractKeywords(text string) []string {
	// Simple keyword extraction
	words := strings.Fields(strings.ToLower(text))
	wordCount := make(map[string]int)

	// Count word frequency, excluding common words
	stopWords := map[string]bool{
		"the": true, "and": true, "of": true, "to": true, "a": true,
		"in": true, "is": true, "it": true, "you": true, "that": true,
		"he": true, "was": true, "for": true, "on": true, "are": true,
	}

	for _, word := range words {
		word = regexp.MustCompile(`[^\w]`).ReplaceAllString(word, "")
		if len(word) > 3 && !stopWords[word] {
			wordCount[word]++
		}
	}

	// Return most frequent keywords
	var keywords []string
	for word, count := range wordCount {
		if count > 2 { // Word appears multiple times
			keywords = append(keywords, word)
		}
	}

	return keywords
}

func (p *BookRecognitionProvider) calculateReadabilityScore(text string) float64 {
	// Simple Flesch Reading Ease approximation
	sentences := len(regexp.MustCompile(`[.!?]+`).FindAllString(text, -1))
	words := len(strings.Fields(text))
	syllables := p.countSyllables(text)

	if sentences == 0 || words == 0 {
		return 0.0
	}

	avgSentenceLength := float64(words) / float64(sentences)
	avgSyllablesPerWord := float64(syllables) / float64(words)

	score := 206.835 - 1.015*avgSentenceLength - 84.6*avgSyllablesPerWord

	// Normalize to 0-1 scale
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	return score / 100.0
}

func (p *BookRecognitionProvider) countSyllables(text string) int {
	// Simple syllable counting approximation
	words := strings.Fields(strings.ToLower(text))
	syllables := 0

	for _, word := range words {
		wordSyllables := 0
		vowels := "aeiouy"
		lastWasVowel := false

		for _, char := range word {
			isVowel := strings.ContainsRune(vowels, char)
			if isVowel && !lastWasVowel {
				wordSyllables++
			}
			lastWasVowel = isVowel
		}

		// At least one syllable per word
		if wordSyllables == 0 {
			wordSyllables = 1
		}

		syllables += wordSyllables
	}

	return syllables
}

func (p *BookRecognitionProvider) determinePublicationType(title, filename, mimeType string) MediaType {
	filename = strings.ToLower(filename)
	title = strings.ToLower(title)

	// Check for comic book patterns
	comicPatterns := []string{"comic", "manga", "graphic novel", "superhero", "marvel", "dc comics"}
	for _, pattern := range comicPatterns {
		if strings.Contains(filename, pattern) || strings.Contains(title, pattern) {
			return MediaTypeComicBook
		}
	}

	// Check for magazine patterns
	magazinePatterns := []string{"magazine", "issue", "vol.", "monthly", "weekly", "quarterly"}
	for _, pattern := range magazinePatterns {
		if strings.Contains(filename, pattern) || strings.Contains(title, pattern) {
			return MediaTypeMagazine
		}
	}

	// Check for academic journal patterns
	journalPatterns := []string{"journal", "proceedings", "conference", "symposium", "research"}
	for _, pattern := range journalPatterns {
		if strings.Contains(filename, pattern) || strings.Contains(title, pattern) {
			return MediaTypeJournal
		}
	}

	// Check for manual patterns
	manualPatterns := []string{"manual", "guide", "handbook", "reference", "documentation"}
	for _, pattern := range manualPatterns {
		if strings.Contains(filename, pattern) || strings.Contains(title, pattern) {
			return MediaTypeManual
		}
	}

	// Default to book
	return MediaTypeBook
}

func (p *BookRecognitionProvider) determineBookType(volumeInfo GoogleBookVolumeInfo) MediaType {
	// Check print type
	if volumeInfo.PrintType == "MAGAZINE" {
		return MediaTypeMagazine
	}

	// Check categories
	for _, category := range volumeInfo.Categories {
		category = strings.ToLower(category)
		if strings.Contains(category, "comic") || strings.Contains(category, "graphic") {
			return MediaTypeComicBook
		}
		if strings.Contains(category, "magazine") || strings.Contains(category, "periodical") {
			return MediaTypeMagazine
		}
		if strings.Contains(category, "reference") || strings.Contains(category, "manual") {
			return MediaTypeManual
		}
	}

	return MediaTypeBook
}

func (p *BookRecognitionProvider) mapCrossrefType(crossrefType string) MediaType {
	switch strings.ToLower(crossrefType) {
	case "journal-article":
		return MediaTypeJournal
	case "book-chapter", "book":
		return MediaTypeBook
	case "proceedings-article":
		return MediaTypeJournal
	case "reference-entry":
		return MediaTypeManual
	default:
		return MediaTypeBook
	}
}

func (p *BookRecognitionProvider) getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func (p *BookRecognitionProvider) parseYear(dateStr string) int {
	yearPattern := regexp.MustCompile(`(\d{4})`)
	if matches := yearPattern.FindStringSubmatch(dateStr); len(matches) > 1 {
		if year, err := strconv.Atoi(matches[1]); err == nil {
			return year
		}
	}
	return 0
}

func (p *BookRecognitionProvider) parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01",
		"2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2006-01-02T15:04:05Z",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

func (p *BookRecognitionProvider) calculateGoogleBooksConfidence(rating float64, ratingCount int) float64 {
	confidence := 0.5

	if rating > 4.0 && ratingCount > 100 {
		confidence += 0.3
	} else if rating > 3.5 && ratingCount > 50 {
		confidence += 0.2
	}

	return confidence
}

func (p *BookRecognitionProvider) calculateOpenLibraryConfidence(editionCount, ebookCount int) float64 {
	confidence := 0.5

	if editionCount > 5 {
		confidence += 0.2
	}

	if ebookCount > 0 {
		confidence += 0.1
	}

	return confidence
}

func (p *BookRecognitionProvider) generateID(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:12]
}

// RecognitionProvider interface implementation
func (p *BookRecognitionProvider) GetProviderName() string {
	return "book_recognition"
}

func (p *BookRecognitionProvider) SupportsMediaType(mediaType MediaType) bool {
	supportedTypes := []MediaType{
		MediaTypeBook,
		MediaTypeComicBook,
		MediaTypeMagazine,
		MediaTypeNewspaper,
		MediaTypeJournal,
		MediaTypeManual,
		MediaTypeEbook,
	}

	for _, supported := range supportedTypes {
		if mediaType == supported {
			return true
		}
	}

	return false
}

func (p *BookRecognitionProvider) GetConfidenceThreshold() float64 {
	return 0.4
}

// maxTesseractOutputBytes bounds how much recognised text is kept in memory.
const maxTesseractOutputBytes = 4 << 20

// ErrOCROutputTooLarge is returned when the OCR process writes more than maxTesseractOutputBytes.
var ErrOCROutputTooLarge = errors.New("OCR output exceeds size limit")

// boundedWriter stops the child (write error) once the limit is passed instead of buffering without bound.
type boundedWriter struct {
	w         *bytes.Buffer
	remaining int
	exceeded  bool
}

func (b *boundedWriter) Write(p []byte) (int, error) {
	if len(p) > b.remaining {
		b.exceeded = true
		return 0, ErrOCROutputTooLarge
	}
	b.remaining -= len(p)
	return b.w.Write(p)
}

// limitedStderr keeps only the first bytes of stderr but never fails the child.
type limitedStderr struct {
	w         *bytes.Buffer
	remaining int
}

func (l *limitedStderr) Write(p []byte) (int, error) {
	n := len(p)
	if l.remaining > 0 {
		if len(p) > l.remaining {
			p = p[:l.remaining]
		}
		l.remaining -= len(p)
		l.w.Write(p)
	}
	return n, nil
}
