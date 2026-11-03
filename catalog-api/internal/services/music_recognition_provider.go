package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"catalogizer/internal/models"
	"go.uber.org/zap"
)

// Music recognition provider with audio fingerprinting
type MusicRecognitionProvider struct {
	logger      *zap.Logger
	httpClient  *http.Client
	baseURLs    map[string]string
	apiKeys     map[string]string
	rateLimiter map[string]*time.Ticker
}

// Last.fm API structures
type LastFMSearchResponse struct {
	Results LastFMResults `json:"results"`
}

type LastFMResults struct {
	TrackMatches  LastFMTrackMatches  `json:"trackmatches"`
	AlbumMatches  LastFMAlbumMatches  `json:"albummatches"`
	ArtistMatches LastFMArtistMatches `json:"artistmatches"`
}

type LastFMTrackMatches struct {
	Track []LastFMTrack `json:"track"`
}

type LastFMAlbumMatches struct {
	Album []LastFMAlbum `json:"album"`
}

type LastFMArtistMatches struct {
	Artist []LastFMArtist `json:"artist"`
}

type LastFMTrack struct {
	Name       string        `json:"name"`
	Artist     string        `json:"artist"`
	URL        string        `json:"url"`
	Streamable string        `json:"streamable"`
	Listeners  string        `json:"listeners"`
	Image      []LastFMImage `json:"image"`
	MBID       string        `json:"mbid"`
}

type LastFMAlbum struct {
	Name   string        `json:"name"`
	Artist string        `json:"artist"`
	URL    string        `json:"url"`
	Image  []LastFMImage `json:"image"`
	MBID   string        `json:"mbid"`
}

type LastFMArtist struct {
	Name       string        `json:"name"`
	Listeners  string        `json:"listeners"`
	MBID       string        `json:"mbid"`
	URL        string        `json:"url"`
	Streamable string        `json:"streamable"`
	Image      []LastFMImage `json:"image"`
}

type LastFMImage struct {
	Text string `json:"#text"`
	Size string `json:"size"`
}

type LastFMTrackInfo struct {
	Track LastFMTrackDetail `json:"track"`
}

type LastFMTrackDetail struct {
	Name       string             `json:"name"`
	MBID       string             `json:"mbid"`
	URL        string             `json:"url"`
	Duration   string             `json:"duration"`
	Streamable LastFMStreamable   `json:"streamable"`
	Listeners  string             `json:"listeners"`
	Playcount  string             `json:"playcount"`
	Artist     LastFMArtistDetail `json:"artist"`
	Album      LastFMAlbumDetail  `json:"album"`
	TopTags    LastFMTopTags      `json:"toptags"`
	Wiki       LastFMWiki         `json:"wiki"`
}

type LastFMStreamable struct {
	Text      string `json:"#text"`
	Fulltrack string `json:"fulltrack"`
}

type LastFMArtistDetail struct {
	Name string `json:"name"`
	MBID string `json:"mbid"`
	URL  string `json:"url"`
}

type LastFMAlbumDetail struct {
	Artist string        `json:"artist"`
	Title  string        `json:"title"`
	MBID   string        `json:"mbid"`
	URL    string        `json:"url"`
	Image  []LastFMImage `json:"image"`
}

type LastFMTopTags struct {
	Tag []LastFMTag `json:"tag"`
}

type LastFMTag struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type LastFMWiki struct {
	Published string `json:"published"`
	Summary   string `json:"summary"`
	Content   string `json:"content"`
}

// MusicBrainz API structures
type MusicBrainzSearchResponse struct {
	Recordings []MusicBrainzRecording `json:"recordings"`
	Releases   []MusicBrainzRelease   `json:"releases"`
	Artists    []MusicBrainzArtist    `json:"artists"`
}

type MusicBrainzRecording struct {
	ID             string                    `json:"id"`
	Score          int                       `json:"score"`
	Title          string                    `json:"title"`
	Length         int                       `json:"length"`
	Disambiguation string                    `json:"disambiguation"`
	ArtistCredit   []MusicBrainzArtistCredit `json:"artist-credit"`
	Releases       []MusicBrainzReleaseBasic `json:"releases"`
	Tags           []MusicBrainzTag          `json:"tags"`
	Genres         []MusicBrainzGenre        `json:"genres"`
	ISRCs          []string                  `json:"isrcs"`
}

type MusicBrainzRelease struct {
	ID           string                    `json:"id"`
	Score        int                       `json:"score"`
	Title        string                    `json:"title"`
	StatusID     string                    `json:"status-id"`
	Status       string                    `json:"status"`
	Packaging    string                    `json:"packaging"`
	Date         string                    `json:"date"`
	Country      string                    `json:"country"`
	Barcode      string                    `json:"barcode"`
	ArtistCredit []MusicBrainzArtistCredit `json:"artist-credit"`
	ReleaseGroup MusicBrainzReleaseGroup   `json:"release-group"`
	Media        []MusicBrainzMedia        `json:"media"`
	LabelInfo    []MusicBrainzLabelInfo    `json:"label-info"`
}

type MusicBrainzArtist struct {
	ID        string              `json:"id"`
	Score     int                 `json:"score"`
	Name      string              `json:"name"`
	SortName  string              `json:"sort-name"`
	Type      string              `json:"type"`
	Gender    string              `json:"gender"`
	Country   string              `json:"country"`
	Area      MusicBrainzArea     `json:"area"`
	BeginArea MusicBrainzArea     `json:"begin-area"`
	EndArea   MusicBrainzArea     `json:"end-area"`
	LifeSpan  MusicBrainzLifeSpan `json:"life-span"`
	Aliases   []MusicBrainzAlias  `json:"aliases"`
	Tags      []MusicBrainzTag    `json:"tags"`
	Genres    []MusicBrainzGenre  `json:"genres"`
}

type MusicBrainzArtistCredit struct {
	Name   string            `json:"name"`
	Artist MusicBrainzArtist `json:"artist"`
}

type MusicBrainzReleaseBasic struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	StatusID string `json:"status-id"`
	Status   string `json:"status"`
	Date     string `json:"date"`
	Country  string `json:"country"`
}

type MusicBrainzReleaseGroup struct {
	ID               string `json:"id"`
	TypeID           string `json:"type-id"`
	Type             string `json:"type"`
	PrimaryTypeID    string `json:"primary-type-id"`
	PrimaryType      string `json:"primary-type"`
	Title            string `json:"title"`
	FirstReleaseDate string `json:"first-release-date"`
}

type MusicBrainzMedia struct {
	Format     string             `json:"format"`
	DiscCount  int                `json:"disc-count"`
	TrackCount int                `json:"track-count"`
	Tracks     []MusicBrainzTrack `json:"tracks"`
}

type MusicBrainzTrack struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	Title    string `json:"title"`
	Length   int    `json:"length"`
	Position int    `json:"position"`
}

type MusicBrainzLabelInfo struct {
	CatalogNumber string           `json:"catalog-number"`
	Label         MusicBrainzLabel `json:"label"`
}

type MusicBrainzLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MusicBrainzArea struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	SortName string   `json:"sort-name"`
	ISO31661 []string `json:"iso-3166-1-codes"`
}

type MusicBrainzLifeSpan struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
	Ended bool   `json:"ended"`
}

type MusicBrainzAlias struct {
	SortName  string `json:"sort-name"`
	Name      string `json:"name"`
	Locale    string `json:"locale"`
	Type      string `json:"type"`
	Primary   bool   `json:"primary"`
	BeginDate string `json:"begin-date"`
	EndDate   string `json:"end-date"`
}

type MusicBrainzTag struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

type MusicBrainzGenre struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

// Audio fingerprinting structures
type AudioFingerprintAnalysis struct {
	Duration         float64          `json:"duration"`
	SampleRate       int              `json:"sample_rate"`
	Channels         int              `json:"channels"`
	BitRate          int              `json:"bit_rate"`
	Tempo            float64          `json:"tempo"`
	Key              string           `json:"key"`
	Energy           float64          `json:"energy"`
	Valence          float64          `json:"valence"`
	Danceability     float64          `json:"danceability"`
	Acousticness     float64          `json:"acousticness"`
	Instrumentalness float64          `json:"instrumentalness"`
	Speechiness      float64          `json:"speechiness"`
	SpectralFeatures SpectralFeatures `json:"spectral_features"`
	ChromaFeatures   []float64        `json:"chroma_features"`
	MFCCFeatures     []float64        `json:"mfcc_features"`
	RhythmFeatures   RhythmFeatures   `json:"rhythm_features"`
}

type SpectralFeatures struct {
	SpectralCentroid  []float64 `json:"spectral_centroid"`
	SpectralBandwidth []float64 `json:"spectral_bandwidth"`
	SpectralRolloff   []float64 `json:"spectral_rolloff"`
	ZeroCrossingRate  []float64 `json:"zero_crossing_rate"`
	SpectralContrast  []float64 `json:"spectral_contrast"`
}

type RhythmFeatures struct {
	OnsetStrength []float64   `json:"onset_strength"`
	BeatTrack     []float64   `json:"beat_track"`
	Tempoogram    [][]float64 `json:"tempogram"`
}

func NewMusicRecognitionProvider(logger *zap.Logger) *MusicRecognitionProvider {
	return &MusicRecognitionProvider{
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURLs: map[string]string{
			"lastfm":      "http://ws.audioscrobbler.com/2.0/",
			"musicbrainz": "https://musicbrainz.org/ws/2",
			"acoustid":    "https://api.acoustid.org/v2",
			"spotify":     "https://api.spotify.com/v1",
			"deezer":      "https://api.deezer.com",
			"discogs":     "https://api.discogs.com",
		},
		apiKeys: map[string]string{
			"lastfm":   "free_api_key",
			"acoustid": "free_api_key",
			"discogs":  "free_api_key",
		},
		rateLimiter: make(map[string]*time.Ticker),
	}
}

func (p *MusicRecognitionProvider) RecognizeMedia(ctx context.Context, req *MediaRecognitionRequest) (*MediaRecognitionResult, error) {
	p.logger.Info("Starting music recognition",
		zap.String("file_path", req.FilePath),
		zap.String("media_type", string(req.MediaType)))

	// Extract metadata from filename
	title, artist, album := p.extractMusicMetadataFromFilename(req.FileName)
	trackNumber := p.extractTrackNumber(req.FileName)

	p.logger.Debug("Extracted metadata from filename",
		zap.String("title", title),
		zap.String("artist", artist),
		zap.String("album", album),
		zap.Int("track_number", trackNumber))

	// Try audio fingerprinting if audio sample provided
	if len(req.AudioSample) > 0 {
		if result, err := p.recognizeByFingerprint(ctx, req.AudioSample); err == nil {
			p.logger.Info("Successfully recognized via fingerprint",
				zap.String("title", result.Title),
				zap.Float64("confidence", result.Confidence))
			return result, nil
		}
	}

	// Try metadata-based recognition
	if title != "" || artist != "" {
		if result, err := p.recognizeByMetadata(ctx, title, artist, album); err == nil {
			result.TrackNumber = trackNumber
			p.logger.Info("Successfully recognized via metadata",
				zap.String("title", result.Title),
				zap.Float64("confidence", result.Confidence))
			return result, nil
		}
	}

	// Fallback to basic recognition
	return p.basicMusicRecognition(req, title, artist, album, trackNumber), nil
}

func (p *MusicRecognitionProvider) recognizeByFingerprint(ctx context.Context, audioSample []byte) (*MediaRecognitionResult, error) {
	// Generate audio fingerprint
	fingerprint, err := p.generateAudioFingerprint(audioSample)
	if err != nil {
		return nil, err
	}

	// Query AcoustID API
	params := url.Values{}
	params.Set("client", p.apiKeys["acoustid"])
	params.Set("format", "json")
	params.Set("meta", "recordings+recordingids+releases+releaseids+releasegroups+releasegroupids+tracks+compress+usermeta+sources")
	params.Set("duration", fmt.Sprintf("%.2f", fingerprint.Duration))
	params.Set("fingerprint", fingerprint.Hash)

	resp, err := p.httpClient.PostForm(p.baseURLs["acoustid"]+"/lookup", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var acoustIDResp AcoustIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&acoustIDResp); err != nil {
		return nil, err
	}

	if acoustIDResp.Status != "ok" || len(acoustIDResp.Results) == 0 {
		return nil, fmt.Errorf("no results from AcoustID")
	}

	// Get the best match
	bestResult := acoustIDResp.Results[0]
	if len(bestResult.Recordings) == 0 {
		return nil, fmt.Errorf("no recordings found")
	}

	recording := bestResult.Recordings[0]

	// Convert to MediaRecognitionResult
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("acoustid_%s", recording.ID),
		MediaType:         p.determineAudioMediaType(recording.Title),
		Title:             recording.Title,
		Duration:          int64(recording.Duration * 1000), // Convert to milliseconds
		Confidence:        bestResult.Score,
		RecognitionMethod: "audio_fingerprint",
		APIProvider:       "AcoustID",
		Fingerprints: map[string]string{
			"acoustid": fingerprint.Hash,
		},
	}

	// Extract artist information
	if len(recording.Artists) > 0 {
		result.Artist = recording.Artists[0].Name
		result.AlbumArtist = recording.Artists[0].Name
	}

	// Extract release information
	if len(recording.Releases) > 0 {
		release := recording.Releases[0]
		result.Album = release.Title
		if release.Date != "" {
			result.Year = p.parseYear(release.Date)
		}
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"acoustid":    recording.ID,
		"musicbrainz": recording.ID,
	}

	// Get additional metadata from MusicBrainz
	if mbResult, err := p.getMusicBrainzDetails(ctx, recording.ID); err == nil {
		p.enhanceMusicResult(result, mbResult)
	}

	return result, nil
}

func (p *MusicRecognitionProvider) recognizeByMetadata(ctx context.Context, title, artist, album string) (*MediaRecognitionResult, error) {
	// Try Last.fm first
	if result, err := p.searchLastFM(ctx, title, artist, album); err == nil {
		return result, nil
	}

	// Try MusicBrainz as fallback
	if result, err := p.searchMusicBrainz(ctx, title, artist, album); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("no results from metadata APIs")
}

func (p *MusicRecognitionProvider) searchLastFM(ctx context.Context, title, artist, album string) (*MediaRecognitionResult, error) {
	params := url.Values{}
	params.Set("method", "track.search")
	params.Set("api_key", p.apiKeys["lastfm"])
	params.Set("format", "json")
	params.Set("limit", "10")

	// Build search query
	query := title
	if artist != "" {
		query = fmt.Sprintf("%s %s", artist, title)
	}
	params.Set("track", query)

	resp, err := p.httpClient.Get(fmt.Sprintf("%s?%s", p.baseURLs["lastfm"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp LastFMSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	if len(searchResp.Results.TrackMatches.Track) == 0 {
		return nil, fmt.Errorf("no tracks found in Last.fm")
	}

	// Get the best match
	bestMatch := searchResp.Results.TrackMatches.Track[0]

	// Get detailed track information
	return p.getLastFMTrackDetails(ctx, bestMatch.Name, bestMatch.Artist, bestMatch.MBID)
}

func (p *MusicRecognitionProvider) getLastFMTrackDetails(ctx context.Context, track, artist, mbid string) (*MediaRecognitionResult, error) {
	params := url.Values{}
	params.Set("method", "track.getInfo")
	params.Set("api_key", p.apiKeys["lastfm"])
	params.Set("format", "json")
	params.Set("artist", artist)
	params.Set("track", track)
	if mbid != "" {
		params.Set("mbid", mbid)
	}

	resp, err := p.httpClient.Get(fmt.Sprintf("%s?%s", p.baseURLs["lastfm"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var trackInfo LastFMTrackInfo
	if err := json.NewDecoder(resp.Body).Decode(&trackInfo); err != nil {
		return nil, err
	}

	track_detail := trackInfo.Track

	// Convert to MediaRecognitionResult
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("lastfm_%s", p.generateID(track_detail.Name, track_detail.Artist.Name)),
		MediaType:         p.determineAudioMediaType(track_detail.Name),
		Title:             track_detail.Name,
		Artist:            track_detail.Artist.Name,
		AlbumArtist:       track_detail.Artist.Name,
		Album:             track_detail.Album.Title,
		Confidence:        p.calculateLastFMConfidence(track_detail.Listeners, track_detail.Playcount),
		RecognitionMethod: "metadata_search",
		APIProvider:       "Last.fm",
	}

	// Parse duration
	if duration, err := strconv.ParseInt(track_detail.Duration, 10, 64); err == nil {
		result.Duration = duration
	}

	// Extract genres from tags
	for _, tag := range track_detail.TopTags.Tag {
		result.Genres = append(result.Genres, tag.Name)
	}

	// Extract description from wiki
	if track_detail.Wiki.Summary != "" {
		result.Description = track_detail.Wiki.Summary
	}

	// Get cover art from album
	for _, image := range track_detail.Album.Image {
		if image.Text != "" {
			size := "medium"
			if image.Size == "extralarge" {
				size = "large"
			} else if image.Size == "large" {
				size = "medium"
			} else if image.Size == "medium" {
				size = "small"
			}

			result.CoverArt = append(result.CoverArt, models.CoverArtResult{
				URL:     image.Text,
				Quality: size,
				Source:  "Last.fm",
			})
		}
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"lastfm_url": track_detail.URL,
	}
	if track_detail.MBID != "" {
		result.ExternalIDs["musicbrainz"] = track_detail.MBID
		result.MusicBrainzID = track_detail.MBID
	}
	if track_detail.Album.MBID != "" {
		result.ExternalIDs["musicbrainz_album"] = track_detail.Album.MBID
	}

	return result, nil
}

func (p *MusicRecognitionProvider) searchMusicBrainz(ctx context.Context, title, artist, album string) (*MediaRecognitionResult, error) {
	// Build search query
	query := fmt.Sprintf("recording:\"%s\"", title)
	if artist != "" {
		query += fmt.Sprintf(" AND artist:\"%s\"", artist)
	}
	if album != "" {
		query += fmt.Sprintf(" AND release:\"%s\"", album)
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("fmt", "json")
	params.Set("limit", "10")

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/recording?%s", p.baseURLs["musicbrainz"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp MusicBrainzSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	if len(searchResp.Recordings) == 0 {
		return nil, fmt.Errorf("no recordings found in MusicBrainz")
	}

	// Get the best match
	bestMatch := searchResp.Recordings[0]

	return p.convertMusicBrainzRecording(bestMatch), nil
}

func (p *MusicRecognitionProvider) getMusicBrainzDetails(ctx context.Context, recordingID string) (*MusicBrainzRecording, error) {
	params := url.Values{}
	params.Set("fmt", "json")
	params.Set("inc", "artists+releases+genres+tags+isrcs")

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/recording/%s?%s", p.baseURLs["musicbrainz"], recordingID, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var recording MusicBrainzRecording
	if err := json.NewDecoder(resp.Body).Decode(&recording); err != nil {
		return nil, err
	}

	return &recording, nil
}

func (p *MusicRecognitionProvider) convertMusicBrainzRecording(recording MusicBrainzRecording) *MediaRecognitionResult {
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("musicbrainz_%s", recording.ID),
		MediaType:         p.determineAudioMediaType(recording.Title),
		Title:             recording.Title,
		Duration:          int64(recording.Length),
		Confidence:        float64(recording.Score) / 100.0,
		RecognitionMethod: "metadata_search",
		APIProvider:       "MusicBrainz",
		MusicBrainzID:     recording.ID,
	}

	// Extract artist information
	if len(recording.ArtistCredit) > 0 {
		result.Artist = recording.ArtistCredit[0].Artist.Name
		result.AlbumArtist = recording.ArtistCredit[0].Artist.Name
	}

	// Extract release information
	if len(recording.Releases) > 0 {
		release := recording.Releases[0]
		result.Album = release.Title
		if release.Date != "" {
			result.Year = p.parseYear(release.Date)
		}
	}

	// Extract genres
	for _, genre := range recording.Genres {
		result.Genres = append(result.Genres, genre.Name)
	}

	// Extract tags
	for _, tag := range recording.Tags {
		result.Tags = append(result.Tags, tag.Name)
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"musicbrainz": recording.ID,
	}
	if len(recording.ISRCs) > 0 {
		result.ExternalIDs["isrc"] = recording.ISRCs[0]
	}

	return result
}

func (p *MusicRecognitionProvider) enhanceMusicResult(result *MediaRecognitionResult, mbRecording *MusicBrainzRecording) {
	// Add additional metadata from MusicBrainz
	if result.Title == "" {
		result.Title = mbRecording.Title
	}

	// Add genres if not present
	if len(result.Genres) == 0 {
		for _, genre := range mbRecording.Genres {
			result.Genres = append(result.Genres, genre.Name)
		}
	}

	// Add tags if not present
	if len(result.Tags) == 0 {
		for _, tag := range mbRecording.Tags {
			result.Tags = append(result.Tags, tag.Name)
		}
	}

	// Add ISRC if available
	if len(mbRecording.ISRCs) > 0 && result.ExternalIDs != nil {
		result.ExternalIDs["isrc"] = mbRecording.ISRCs[0]
	}
}

func (p *MusicRecognitionProvider) basicMusicRecognition(req *MediaRecognitionRequest, title, artist, album string, trackNumber int) *MediaRecognitionResult {
	// Basic fallback recognition
	mediaType := req.MediaType
	if mediaType == "" {
		mediaType = p.determineAudioMediaType(title)
	}

	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("basic_music_%s_%d", strings.ReplaceAll(title, " ", "_"), time.Now().Unix()),
		MediaType:         mediaType,
		Title:             title,
		Artist:            artist,
		Album:             album,
		TrackNumber:       trackNumber,
		Confidence:        0.3, // Low confidence for basic recognition
		RecognitionMethod: "filename_parsing",
		APIProvider:       "basic",
		ExternalIDs:       make(map[string]string),
	}
}

// Audio fingerprinting implementation
func (p *MusicRecognitionProvider) generateAudioFingerprint(audioSample []byte) (*AudioFingerprint, error) {
	// This is a simplified fingerprinting implementation
	// In production, you would use libraries like chromaprint or similar

	// Generate MD5 hash of audio sample as basic fingerprint
	hash := sha256.Sum256(audioSample)
	fingerprintHash := hex.EncodeToString(hash[:])

	// Extract basic audio features (simplified)
	analysis := p.analyzeAudioFeatures(audioSample)

	fingerprint := &AudioFingerprint{
		Algorithm:  "sha256_basic",
		Hash:       fingerprintHash,
		Duration:   analysis.Duration,
		SampleRate: analysis.SampleRate,
		Channels:   analysis.Channels,
		Features:   map[string]float64{},
		Segments:   p.generateFingerprintSegments(audioSample, analysis),
	}

	return fingerprint, nil
}

func (p *MusicRecognitionProvider) analyzeAudioFeatures(audioSample []byte) *AudioFingerprintAnalysis {
	// Simplified audio analysis
	// In production, use proper audio analysis libraries

	analysis := &AudioFingerprintAnalysis{
		Duration:   float64(len(audioSample)) / 44100.0 / 2.0, // Assume 44.1kHz stereo
		SampleRate: 44100,
		Channels:   2,
		BitRate:    1411, // CD quality
		// Tempo, Energy and Valence need real signal analysis; they stay zero
		// (unmeasured) instead of being derived from len(audioSample).
	}

	// Byte-derived pseudo features (tracked CAT-001 follow-up: replace with
	// real chroma/MFCC extraction).
	analysis.ChromaFeatures = make([]float64, 12)
	analysis.MFCCFeatures = make([]float64, 13)

	for i := range analysis.ChromaFeatures {
		analysis.ChromaFeatures[i] = float64(audioSample[i%len(audioSample)]) / 255.0
	}

	for i := range analysis.MFCCFeatures {
		analysis.MFCCFeatures[i] = float64(audioSample[(i*10)%len(audioSample)]) / 255.0
	}

	return analysis
}

func (p *MusicRecognitionProvider) generateFingerprintSegments(audioSample []byte, analysis *AudioFingerprintAnalysis) []FingerprintSegment {
	segments := make([]FingerprintSegment, 0)
	segmentDuration := 10.0 // 10 second segments

	numSegments := int(math.Ceil(analysis.Duration / segmentDuration))
	for i := 0; i < numSegments; i++ {
		startTime := float64(i) * segmentDuration
		endTime := math.Min(startTime+segmentDuration, analysis.Duration)

		// Generate segment hash
		segmentStart := int(startTime * float64(analysis.SampleRate) * float64(analysis.Channels))
		segmentEnd := int(endTime * float64(analysis.SampleRate) * float64(analysis.Channels))

		if segmentEnd > len(audioSample) {
			segmentEnd = len(audioSample)
		}
		if segmentStart >= segmentEnd {
			break
		}

		segmentData := audioSample[segmentStart:segmentEnd]
		hash := sha256.Sum256(segmentData)
		segmentHash := hex.EncodeToString(hash[:])

		segment := FingerprintSegment{
			StartTime: startTime,
			EndTime:   endTime,
			Hash:      segmentHash,
			Features:  map[string]float64{},
		}

		segments = append(segments, segment)
	}

	return segments
}

// Helper methods
func (p *MusicRecognitionProvider) extractMusicMetadataFromFilename(filename string) (title, artist, album string) {
	// Remove file extension
	name := strings.TrimSuffix(filename, "."+p.getFileExtension(filename))

	// Common patterns for music files:
	// Artist - Title
	// Artist - Album - Track Number - Title
	// Track Number - Artist - Title
	// Album - Track Number - Artist - Title

	// Pattern: Artist - Title
	if parts := strings.Split(name, " - "); len(parts) >= 2 {
		artist = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(parts[1])

		// If there are more parts, try to identify album
		if len(parts) >= 3 {
			// Check if second part looks like album or track number
			if !p.looksLikeTrackNumber(parts[1]) {
				album = strings.TrimSpace(parts[1])
				title = strings.TrimSpace(parts[2])
			}
		}

		return title, artist, album
	}

	// Pattern: Track Number - Title
	trackNumPattern := regexp.MustCompile(`^(\d+)[\s\-\.]+(.+)$`)
	if matches := trackNumPattern.FindStringSubmatch(name); len(matches) == 3 {
		title = strings.TrimSpace(matches[2])
		// Try to extract artist from title if it contains " - "
		if parts := strings.Split(title, " - "); len(parts) >= 2 {
			artist = strings.TrimSpace(parts[0])
			title = strings.TrimSpace(parts[1])
		}
		return title, artist, album
	}

	// Fallback: use filename as title
	title = name
	return title, artist, album
}

func (p *MusicRecognitionProvider) extractTrackNumber(filename string) int {
	// Look for track number at the beginning
	trackNumPattern := regexp.MustCompile(`^(\d+)[\s\-\.]`)
	if matches := trackNumPattern.FindStringSubmatch(filename); len(matches) >= 2 {
		if trackNum, err := strconv.Atoi(matches[1]); err == nil {
			return trackNum
		}
	}

	// Look for track number pattern like "Track 01" or "01 -"
	trackPattern := regexp.MustCompile(`(?i)track\s*(\d+)|(\d+)\s*[-\.]`)
	if matches := trackPattern.FindStringSubmatch(filename); len(matches) >= 2 {
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" {
				if trackNum, err := strconv.Atoi(matches[i]); err == nil {
					return trackNum
				}
			}
		}
	}

	return 0
}

func (p *MusicRecognitionProvider) looksLikeTrackNumber(str string) bool {
	// Check if string looks like a track number
	trackPattern := regexp.MustCompile(`^\d{1,3}$`)
	return trackPattern.MatchString(strings.TrimSpace(str))
}

func (p *MusicRecognitionProvider) determineAudioMediaType(title string) MediaType {
	title = strings.ToLower(title)

	// Check for audiobook patterns
	audiobookPatterns := []string{"audiobook", "narrated", "narrator", "chapter", "audio book"}
	for _, pattern := range audiobookPatterns {
		if strings.Contains(title, pattern) {
			return MediaTypeAudiobook
		}
	}

	// Check for podcast patterns
	podcastPatterns := []string{"podcast", "episode", "ep.", "show"}
	for _, pattern := range podcastPatterns {
		if strings.Contains(title, pattern) {
			return MediaTypePodcast
		}
	}

	// Default to music
	return MediaTypeMusic
}

func (p *MusicRecognitionProvider) getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func (p *MusicRecognitionProvider) parseYear(dateStr string) int {
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}
	return 0
}

func (p *MusicRecognitionProvider) calculateLastFMConfidence(listeners, playcount string) float64 {
	confidence := 0.5

	if l, err := strconv.Atoi(listeners); err == nil && l > 1000 {
		confidence += 0.2
	}

	if p, err := strconv.Atoi(playcount); err == nil && p > 10000 {
		confidence += 0.2
	}

	return confidence
}

func (p *MusicRecognitionProvider) generateID(title, artist string) string {
	combined := fmt.Sprintf("%s_%s", title, artist)
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])[:12]
}

// RecognitionProvider interface implementation
func (p *MusicRecognitionProvider) GetProviderName() string {
	return "music_recognition"
}

func (p *MusicRecognitionProvider) SupportsMediaType(mediaType MediaType) bool {
	supportedTypes := []MediaType{
		MediaTypeMusic,
		MediaTypeAlbum,
		MediaTypeAudiobook,
		MediaTypePodcast,
	}

	for _, supported := range supportedTypes {
		if mediaType == supported {
			return true
		}
	}

	return false
}

func (p *MusicRecognitionProvider) GetConfidenceThreshold() float64 {
	return 0.4 // Minimum 40% confidence required
}

// AcoustID API response structure
type AcoustIDResponse struct {
	Status  string           `json:"status"`
	Results []AcoustIDResult `json:"results"`
}

type AcoustIDResult struct {
	ID         string              `json:"id"`
	Score      float64             `json:"score"`
	Recordings []AcoustIDRecording `json:"recordings"`
}

type AcoustIDRecording struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Duration float64           `json:"duration"`
	Artists  []AcoustIDArtist  `json:"artists"`
	Releases []AcoustIDRelease `json:"releases"`
}

type AcoustIDArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AcoustIDRelease struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Date    string `json:"date"`
	Country string `json:"country"`
}
