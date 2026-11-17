package services

import (
	"catalogizer/internal/netguard"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"catalogizer/internal/models"
	"go.uber.org/zap"
)

// Movie/TV recognition provider using free APIs
type MovieRecognitionProvider struct {
	logger      *zap.Logger
	httpClient  *http.Client
	baseURLs    map[string]string
	apiKeys     map[string]string
	rateLimiter map[string]*time.Ticker
}

// External API response structures
type TMDbSearchResponse struct {
	Page         int          `json:"page"`
	Results      []TMDbResult `json:"results"`
	TotalPages   int          `json:"total_pages"`
	TotalResults int          `json:"total_results"`
}

type TMDbResult struct {
	ID               int      `json:"id"`
	Title            string   `json:"title,omitempty"`
	Name             string   `json:"name,omitempty"`
	OriginalTitle    string   `json:"original_title,omitempty"`
	OriginalName     string   `json:"original_name,omitempty"`
	Overview         string   `json:"overview"`
	ReleaseDate      string   `json:"release_date,omitempty"`
	FirstAirDate     string   `json:"first_air_date,omitempty"`
	GenreIDs         []int    `json:"genre_ids"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	PosterPath       string   `json:"poster_path,omitempty"`
	BackdropPath     string   `json:"backdrop_path,omitempty"`
	Popularity       float64  `json:"popularity"`
	Adult            bool     `json:"adult"`
	Video            bool     `json:"video,omitempty"`
	MediaType        string   `json:"media_type,omitempty"`
	OriginCountry    []string `json:"origin_country,omitempty"`
	OriginalLanguage string   `json:"original_language"`
}

type TMDbMovieDetails struct {
	ID                  int                     `json:"id"`
	Title               string                  `json:"title"`
	OriginalTitle       string                  `json:"original_title"`
	Overview            string                  `json:"overview"`
	ReleaseDate         string                  `json:"release_date"`
	Runtime             int                     `json:"runtime"`
	Genres              []TMDbGenre             `json:"genres"`
	ProductionCompanies []TMDbProductionCompany `json:"production_companies"`
	ProductionCountries []TMDbCountry           `json:"production_countries"`
	SpokenLanguages     []TMDbLanguage          `json:"spoken_languages"`
	VoteAverage         float64                 `json:"vote_average"`
	VoteCount           int                     `json:"vote_count"`
	Popularity          float64                 `json:"popularity"`
	PosterPath          string                  `json:"poster_path"`
	BackdropPath        string                  `json:"backdrop_path"`
	Adult               bool                    `json:"adult"`
	Homepage            string                  `json:"homepage"`
	IMDbID              string                  `json:"imdb_id"`
	Budget              int64                   `json:"budget"`
	Revenue             int64                   `json:"revenue"`
	Status              string                  `json:"status"`
	Tagline             string                  `json:"tagline"`
}

type TMDbTVDetails struct {
	ID                  int                     `json:"id"`
	Name                string                  `json:"name"`
	OriginalName        string                  `json:"original_name"`
	Overview            string                  `json:"overview"`
	FirstAirDate        string                  `json:"first_air_date"`
	LastAirDate         string                  `json:"last_air_date"`
	Genres              []TMDbGenre             `json:"genres"`
	CreatedBy           []TMDbCreator           `json:"created_by"`
	Networks            []TMDbNetwork           `json:"networks"`
	ProductionCompanies []TMDbProductionCompany `json:"production_companies"`
	ProductionCountries []TMDbCountry           `json:"production_countries"`
	SpokenLanguages     []TMDbLanguage          `json:"spoken_languages"`
	VoteAverage         float64                 `json:"vote_average"`
	VoteCount           int                     `json:"vote_count"`
	Popularity          float64                 `json:"popularity"`
	PosterPath          string                  `json:"poster_path"`
	BackdropPath        string                  `json:"backdrop_path"`
	Homepage            string                  `json:"homepage"`
	InProduction        bool                    `json:"in_production"`
	NumberOfEpisodes    int                     `json:"number_of_episodes"`
	NumberOfSeasons     int                     `json:"number_of_seasons"`
	Status              string                  `json:"status"`
	Type                string                  `json:"type"`
	Tagline             string                  `json:"tagline"`
	ExternalIDs         TMDbExternalIDs         `json:"external_ids,omitempty"`
}

type TMDbGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type TMDbProductionCompany struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type TMDbCountry struct {
	ISO31661 string `json:"iso_3166_1"`
	Name     string `json:"name"`
}

type TMDbLanguage struct {
	ISO6391     string `json:"iso_639_1"`
	EnglishName string `json:"english_name"`
	Name        string `json:"name"`
}

type TMDbCreator struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Gender      int    `json:"gender"`
	ProfilePath string `json:"profile_path"`
}

type TMDbNetwork struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type TMDbExternalIDs struct {
	IMDbID      string `json:"imdb_id"`
	TVDBID      int    `json:"tvdb_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

type TMDbCredits struct {
	ID   int              `json:"id"`
	Cast []TMDbCastMember `json:"cast"`
	Crew []TMDbCrewMember `json:"crew"`
}

type TMDbCastMember struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Character   string  `json:"character"`
	Order       int     `json:"order"`
	Gender      int     `json:"gender"`
	ProfilePath string  `json:"profile_path"`
	CastID      int     `json:"cast_id"`
	CreditID    string  `json:"credit_id"`
	Popularity  float64 `json:"popularity"`
}

type TMDbCrewMember struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Job         string  `json:"job"`
	Department  string  `json:"department"`
	Gender      int     `json:"gender"`
	ProfilePath string  `json:"profile_path"`
	CreditID    string  `json:"credit_id"`
	Popularity  float64 `json:"popularity"`
}

// OMDb API structures (fallback)
type OMDbResponse struct {
	Title      string       `json:"Title"`
	Year       string       `json:"Year"`
	Rated      string       `json:"Rated"`
	Released   string       `json:"Released"`
	Runtime    string       `json:"Runtime"`
	Genre      string       `json:"Genre"`
	Director   string       `json:"Director"`
	Writer     string       `json:"Writer"`
	Actors     string       `json:"Actors"`
	Plot       string       `json:"Plot"`
	Language   string       `json:"Language"`
	Country    string       `json:"Country"`
	Awards     string       `json:"Awards"`
	Poster     string       `json:"Poster"`
	Ratings    []OMDbRating `json:"Ratings"`
	Metascore  string       `json:"Metascore"`
	IMDbRating string       `json:"imdbRating"`
	IMDbVotes  string       `json:"imdbVotes"`
	IMDbID     string       `json:"imdbID"`
	Type       string       `json:"Type"`
	DVD        string       `json:"DVD,omitempty"`
	BoxOffice  string       `json:"BoxOffice,omitempty"`
	Production string       `json:"Production,omitempty"`
	Website    string       `json:"Website,omitempty"`
	Response   string       `json:"Response"`
	Error      string       `json:"Error,omitempty"`
}

type OMDbRating struct {
	Source string `json:"Source"`
	Value  string `json:"Value"`
}

func NewMovieRecognitionProvider(logger *zap.Logger) *MovieRecognitionProvider {
	return &MovieRecognitionProvider{
		logger:     logger,
		httpClient: netguard.NewProviderClient(netguard.ProviderClientConfig{Timeout: 30 * time.Second, Strict: true}),
		baseURLs: map[string]string{
			"tmdb": "https://api.themoviedb.org/3",
			"omdb": "http://www.omdbapi.com",
			"tvdb": "https://api4.thetvdb.com/v4",
			"imdb": "https://imdb-api.com",
		},
		apiKeys: map[string]string{
			"tmdb": "free_api_key", // Using free tier
			"omdb": "free_api_key", // Using free tier
		},
		rateLimiter: make(map[string]*time.Ticker),
	}
}

func (p *MovieRecognitionProvider) RecognizeMedia(ctx context.Context, req *MediaRecognitionRequest) (*MediaRecognitionResult, error) {
	p.logger.Info("Starting movie/TV recognition",
		zap.String("file_path", req.FilePath),
		zap.String("media_type", string(req.MediaType)))

	// Extract title from filename
	title := p.extractTitleFromFilename(req.FileName)
	year := p.extractYearFromFilename(req.FileName)

	// Extract season/episode info for TV shows
	season, episode := p.extractSeasonEpisode(req.FileName)

	p.logger.Debug("Extracted metadata from filename",
		zap.String("title", title),
		zap.Int("year", year),
		zap.Int("season", season),
		zap.Int("episode", episode))

	// Try TMDb first (best free API)
	if result, err := p.searchTMDb(ctx, title, year, req.MediaType, season, episode); err == nil {
		p.logger.Info("Successfully recognized via TMDb",
			zap.String("title", result.Title),
			zap.Float64("confidence", result.Confidence))
		return result, nil
	}

	// Fallback to OMDb
	if result, err := p.searchOMDb(ctx, title, year, req.MediaType); err == nil {
		p.logger.Info("Successfully recognized via OMDb",
			zap.String("title", result.Title),
			zap.Float64("confidence", result.Confidence))
		return result, nil
	}

	// Fallback to basic pattern matching
	return p.basicRecognition(req, title, year, season, episode), nil
}

func (p *MovieRecognitionProvider) searchTMDb(ctx context.Context, title string, year int, mediaType MediaType, season, episode int) (*MediaRecognitionResult, error) {
	// Search for the media
	searchURL := fmt.Sprintf("%s/search/multi", p.baseURLs["tmdb"])
	params := url.Values{}
	params.Set("api_key", p.apiKeys["tmdb"])
	params.Set("query", title)
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}

	resp, err := p.httpClient.Get(fmt.Sprintf("%s?%s", searchURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp TMDbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	if len(searchResp.Results) == 0 {
		return nil, fmt.Errorf("no results found in TMDb")
	}

	// Get the best match
	bestMatch := searchResp.Results[0]

	// Get detailed information
	if bestMatch.MediaType == "movie" || (bestMatch.Title != "" && bestMatch.MediaType == "") {
		return p.getTMDbMovieDetails(ctx, bestMatch.ID)
	} else if bestMatch.MediaType == "tv" || bestMatch.Name != "" {
		return p.getTMDbTVDetails(ctx, bestMatch.ID, season, episode)
	}

	return nil, fmt.Errorf("unsupported media type from TMDb")
}

func (p *MovieRecognitionProvider) getTMDbMovieDetails(ctx context.Context, movieID int) (*MediaRecognitionResult, error) {
	// Get movie details
	detailsURL := fmt.Sprintf("%s/movie/%d", p.baseURLs["tmdb"], movieID)
	params := url.Values{}
	params.Set("api_key", p.apiKeys["tmdb"])
	params.Set("append_to_response", "credits,external_ids,images")

	resp, err := p.httpClient.Get(fmt.Sprintf("%s?%s", detailsURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var movie TMDbMovieDetails
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, err
	}

	// Convert to MediaRecognitionResult
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("tmdb_movie_%d", movie.ID),
		MediaType:         MediaTypeMovie,
		Title:             movie.Title,
		OriginalTitle:     movie.OriginalTitle,
		Description:       movie.Overview,
		Year:              p.parseYear(movie.ReleaseDate),
		Duration:          int64(movie.Runtime * 60), // Convert minutes to seconds
		IMDbID:            movie.IMDbID,
		TMDbID:            strconv.Itoa(movie.ID),
		Rating:            movie.VoteAverage,
		Confidence:        p.calculateConfidence(movie.Title, movie.VoteAverage, movie.VoteCount),
		RecognitionMethod: "tmdb_api",
		APIProvider:       "TMDb",
	}

	// Parse release date
	if releaseDate, err := time.Parse("2006-01-02", movie.ReleaseDate); err == nil {
		result.ReleaseDate = &releaseDate
	}

	// Extract genres
	for _, genre := range movie.Genres {
		result.Genres = append(result.Genres, genre.Name)
	}

	// Get cover art
	if movie.PosterPath != "" {
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			URL:     fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", movie.PosterPath),
			Width:   500,
			Quality: "medium",
			Source:  "TMDb",
		})
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			URL:     fmt.Sprintf("https://image.tmdb.org/t/p/original%s", movie.PosterPath),
			Quality: "high",
			Source:  "TMDb",
		})
	}

	if movie.BackdropPath != "" {
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			URL:     fmt.Sprintf("https://image.tmdb.org/t/p/w1280%s", movie.BackdropPath),
			Width:   1280,
			Quality: "high",
			Source:  "TMDb",
		})
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"tmdb_id": strconv.Itoa(movie.ID),
	}
	if movie.IMDbID != "" {
		result.ExternalIDs["imdb_id"] = movie.IMDbID
	}

	return result, nil
}

func (p *MovieRecognitionProvider) getTMDbTVDetails(ctx context.Context, tvID, season, episode int) (*MediaRecognitionResult, error) {
	// Get TV series details
	detailsURL := fmt.Sprintf("%s/tv/%d", p.baseURLs["tmdb"], tvID)
	params := url.Values{}
	params.Set("api_key", p.apiKeys["tmdb"])
	params.Set("append_to_response", "credits,external_ids,images")

	resp, err := p.httpClient.Get(fmt.Sprintf("%s?%s", detailsURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tv TMDbTVDetails
	if err := json.NewDecoder(resp.Body).Decode(&tv); err != nil {
		return nil, err
	}

	// Determine media type
	mediaType := MediaTypeTVSeries
	title := tv.Name
	if season > 0 && episode > 0 {
		mediaType = MediaTypeTVEpisode
		// Try to get episode details
		if episodeDetails, err := p.getTMDbEpisodeDetails(ctx, tvID, season, episode); err == nil {
			title = episodeDetails.Name
		}
	}

	// Convert to MediaRecognitionResult
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("tmdb_tv_%d", tv.ID),
		MediaType:         mediaType,
		Title:             title,
		OriginalTitle:     tv.OriginalName,
		SeriesTitle:       tv.Name,
		Description:       tv.Overview,
		Year:              p.parseYear(tv.FirstAirDate),
		Season:            season,
		Episode:           episode,
		TMDbID:            strconv.Itoa(tv.ID),
		Rating:            tv.VoteAverage,
		Confidence:        p.calculateConfidence(tv.Name, tv.VoteAverage, tv.VoteCount),
		RecognitionMethod: "tmdb_api",
		APIProvider:       "TMDb",
	}

	// Parse first air date
	if firstAirDate, err := time.Parse("2006-01-02", tv.FirstAirDate); err == nil {
		result.ReleaseDate = &firstAirDate
	}

	// Extract genres
	for _, genre := range tv.Genres {
		result.Genres = append(result.Genres, genre.Name)
	}

	// Get cover art
	if tv.PosterPath != "" {
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			URL:     fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", tv.PosterPath),
			Width:   500,
			Quality: "medium",
			Source:  "TMDb",
		})
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"tmdb_id": strconv.Itoa(tv.ID),
	}
	if tv.ExternalIDs.IMDbID != "" {
		result.ExternalIDs["imdb_id"] = tv.ExternalIDs.IMDbID
		result.IMDbID = tv.ExternalIDs.IMDbID
	}
	if tv.ExternalIDs.TVDBID > 0 {
		result.ExternalIDs["tvdb_id"] = strconv.Itoa(tv.ExternalIDs.TVDBID)
		result.TVDBId = strconv.Itoa(tv.ExternalIDs.TVDBID)
	}

	return result, nil
}

func (p *MovieRecognitionProvider) getTMDbEpisodeDetails(ctx context.Context, tvID, season, episode int) (*TMDbEpisodeDetails, error) {
	detailsURL := fmt.Sprintf("%s/tv/%d/season/%d/episode/%d", p.baseURLs["tmdb"], tvID, season, episode)
	params := url.Values{}
	params.Set("api_key", p.apiKeys["tmdb"])

	resp, err := p.httpClient.Get(fmt.Sprintf("%s?%s", detailsURL, params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var episode_details TMDbEpisodeDetails
	if err := json.NewDecoder(resp.Body).Decode(&episode_details); err != nil {
		return nil, err
	}

	return &episode_details, nil
}

type TMDbEpisodeDetails struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	EpisodeNumber int     `json:"episode_number"`
	SeasonNumber  int     `json:"season_number"`
	Runtime       int     `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	StillPath     string  `json:"still_path"`
}

func (p *MovieRecognitionProvider) searchOMDb(ctx context.Context, title string, year int, mediaType MediaType) (*MediaRecognitionResult, error) {
	params := url.Values{}
	params.Set("apikey", p.apiKeys["omdb"])
	params.Set("t", title)
	if year > 0 {
		params.Set("y", strconv.Itoa(year))
	}

	// Set type based on media type
	switch mediaType {
	case MediaTypeMovie, MediaTypeConcert, MediaTypeDocumentary:
		params.Set("type", "movie")
	case MediaTypeTVSeries, MediaTypeTVEpisode:
		params.Set("type", "series")
	}

	resp, err := p.httpClient.Get(fmt.Sprintf("%s?%s", p.baseURLs["omdb"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var omdbResp OMDbResponse
	if err := json.NewDecoder(resp.Body).Decode(&omdbResp); err != nil {
		return nil, err
	}

	if omdbResp.Response == "False" {
		return nil, fmt.Errorf("OMDb error: %s", omdbResp.Error)
	}

	// Convert OMDb response to MediaRecognitionResult
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("omdb_%s", omdbResp.IMDbID),
		MediaType:         p.mapOMDbType(omdbResp.Type),
		Title:             omdbResp.Title,
		Description:       omdbResp.Plot,
		Year:              p.parseYear(omdbResp.Year),
		Director:          omdbResp.Director,
		IMDbID:            omdbResp.IMDbID,
		Confidence:        p.calculateOMDbConfidence(omdbResp.IMDbRating, omdbResp.IMDbVotes),
		RecognitionMethod: "omdb_api",
		APIProvider:       "OMDb",
	}

	// Parse release date
	if releaseDate, err := time.Parse("02 Jan 2006", omdbResp.Released); err == nil {
		result.ReleaseDate = &releaseDate
	}

	// Parse genres
	if omdbResp.Genre != "" {
		result.Genres = strings.Split(omdbResp.Genre, ", ")
	}

	// Parse runtime
	if runtime := p.parseRuntime(omdbResp.Runtime); runtime > 0 {
		result.Duration = runtime
	}

	// Parse rating
	if rating, err := strconv.ParseFloat(omdbResp.IMDbRating, 64); err == nil {
		result.Rating = rating
	}

	// Parse cast
	if omdbResp.Actors != "" {
		actors := strings.Split(omdbResp.Actors, ", ")
		for _, actor := range actors {
			result.Cast = append(result.Cast, Person{
				Name: actor,
				Role: "Actor",
			})
		}
	}

	// Add cover art
	if omdbResp.Poster != "" && omdbResp.Poster != "N/A" {
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			URL:     omdbResp.Poster,
			Quality: "medium",
			Source:  "OMDb",
		})
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"imdb_id": omdbResp.IMDbID,
	}

	return result, nil
}

func (p *MovieRecognitionProvider) basicRecognition(req *MediaRecognitionRequest, title string, year, season, episode int) *MediaRecognitionResult {
	// Basic fallback recognition
	mediaType := req.MediaType
	if mediaType == "" {
		if season > 0 && episode > 0 {
			mediaType = MediaTypeTVEpisode
		} else if season > 0 {
			mediaType = MediaTypeTVSeries
		} else {
			mediaType = MediaTypeMovie
		}
	}

	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("basic_%s_%d", strings.ReplaceAll(title, " ", "_"), time.Now().Unix()),
		MediaType:         mediaType,
		Title:             title,
		Year:              year,
		Season:            season,
		Episode:           episode,
		Confidence:        0.3, // Low confidence for basic recognition
		RecognitionMethod: "filename_parsing",
		APIProvider:       "basic",
		ExternalIDs:       make(map[string]string),
	}
}

// Helper methods
func (p *MovieRecognitionProvider) extractTitleFromFilename(filename string) string {
	// Remove file extension
	name := strings.TrimSuffix(filename, "."+p.getFileExtension(filename))

	// Remove common patterns
	patterns := []string{
		`\d{4}`,           // Year
		`S\d{2}E\d{2}`,    // Season/Episode
		`\d{1,2}x\d{1,2}`, // Alternative season/episode
		`(?i)(720p|1080p|4k|hdtv|webrip|bluray|dvdrip|cam|ts|r5)`, // Quality
		`(?i)(xvid|x264|h264|h265|hevc)`,                          // Codec
		`(?i)(aac|ac3|dts|mp3)`,                                   // Audio
		`\[.*?\]`,                                                 // Brackets
		`\(.*?\)`,                                                 // Parentheses
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		name = re.ReplaceAllString(name, "")
	}

	// Clean up
	name = regexp.MustCompile(`[._-]+`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	return name
}

func (p *MovieRecognitionProvider) extractYearFromFilename(filename string) int {
	re := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	matches := re.FindAllString(filename, -1)

	if len(matches) > 0 {
		if year, err := strconv.Atoi(matches[len(matches)-1]); err == nil {
			return year
		}
	}

	return 0
}

func (p *MovieRecognitionProvider) extractSeasonEpisode(filename string) (int, int) {
	// Pattern: S01E01
	re := regexp.MustCompile(`S(\d{1,2})E(\d{1,2})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) == 3 {
		season, _ := strconv.Atoi(matches[1])
		episode, _ := strconv.Atoi(matches[2])
		return season, episode
	}

	// Pattern: 1x01
	re = regexp.MustCompile(`(\d{1,2})x(\d{1,2})`)
	matches = re.FindStringSubmatch(filename)
	if len(matches) == 3 {
		season, _ := strconv.Atoi(matches[1])
		episode, _ := strconv.Atoi(matches[2])
		return season, episode
	}

	return 0, 0
}

func (p *MovieRecognitionProvider) getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func (p *MovieRecognitionProvider) parseYear(dateStr string) int {
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}
	return 0
}

func (p *MovieRecognitionProvider) parseRuntime(runtime string) int64 {
	re := regexp.MustCompile(`(\d+)\s*min`)
	matches := re.FindStringSubmatch(runtime)
	if len(matches) == 2 {
		if minutes, err := strconv.Atoi(matches[1]); err == nil {
			return int64(minutes * 60) // Convert to seconds
		}
	}
	return 0
}

func (p *MovieRecognitionProvider) calculateConfidence(title string, rating float64, voteCount int) float64 {
	confidence := 0.5 // Base confidence

	// Boost confidence based on rating and vote count
	if rating > 7.0 && voteCount > 1000 {
		confidence += 0.3
	} else if rating > 6.0 && voteCount > 100 {
		confidence += 0.2
	} else if voteCount > 50 {
		confidence += 0.1
	}

	// Boost confidence if title is not empty
	if title != "" {
		confidence += 0.2
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func (p *MovieRecognitionProvider) calculateOMDbConfidence(rating, votes string) float64 {
	confidence := 0.5

	if r, err := strconv.ParseFloat(rating, 64); err == nil && r > 6.0 {
		confidence += 0.2
	}

	// Parse vote count (remove commas)
	cleanVotes := strings.ReplaceAll(votes, ",", "")
	if v, err := strconv.Atoi(cleanVotes); err == nil && v > 1000 {
		confidence += 0.2
	}

	return confidence
}

func (p *MovieRecognitionProvider) mapOMDbType(omdbType string) MediaType {
	switch strings.ToLower(omdbType) {
	case "movie":
		return MediaTypeMovie
	case "series":
		return MediaTypeTVSeries
	case "episode":
		return MediaTypeTVEpisode
	default:
		return MediaTypeMovie
	}
}

// RecognitionProvider interface implementation
func (p *MovieRecognitionProvider) GetProviderName() string {
	return "movie_recognition"
}

func (p *MovieRecognitionProvider) SupportsMediaType(mediaType MediaType) bool {
	supportedTypes := []MediaType{
		MediaTypeMovie,
		MediaTypeTVSeries,
		MediaTypeTVEpisode,
		MediaTypeConcert,
		MediaTypeDocumentary,
		MediaTypeCourse,
		MediaTypeTraining,
	}

	for _, supported := range supportedTypes {
		if mediaType == supported {
			return true
		}
	}

	return false
}

func (p *MovieRecognitionProvider) GetConfidenceThreshold() float64 {
	return 0.4 // Minimum 40% confidence required
}
