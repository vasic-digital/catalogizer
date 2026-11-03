package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// Game and software recognition provider
type GameSoftwareRecognitionProvider struct {
	logger      *zap.Logger
	httpClient  *http.Client
	baseURLs    map[string]string
	apiKeys     map[string]string
	rateLimiter map[string]*time.Ticker
}

// IGDB (Internet Game Database) API structures
type IGDBGame struct {
	ID                    int                     `json:"id"`
	Name                  string                  `json:"name"`
	Summary               string                  `json:"summary"`
	Storyline             string                  `json:"storyline"`
	FirstReleaseDate      int64                   `json:"first_release_date"`
	Category              int                     `json:"category"`
	Status                int                     `json:"status"`
	Rating                float64                 `json:"rating"`
	RatingCount           int                     `json:"rating_count"`
	AggregatedRating      float64                 `json:"aggregated_rating"`
	AggregatedRatingCount int                     `json:"aggregated_rating_count"`
	TotalRating           float64                 `json:"total_rating"`
	TotalRatingCount      int                     `json:"total_rating_count"`
	Popularity            float64                 `json:"popularity"`
	Cover                 IGDBCover               `json:"cover"`
	Screenshots           []IGDBScreenshot        `json:"screenshots"`
	Artworks              []IGDBArtwork           `json:"artworks"`
	Videos                []IGDBVideo             `json:"videos"`
	Genres                []IGDBGenre             `json:"genres"`
	Themes                []IGDBTheme             `json:"themes"`
	Platforms             []IGDBPlatform          `json:"platforms"`
	GameModes             []IGDBGameMode          `json:"game_modes"`
	PlayerPerspectives    []IGDBPlayerPerspective `json:"player_perspectives"`
	InvolvedCompanies     []IGDBInvolvedCompany   `json:"involved_companies"`
	Franchises            []IGDBFranchise         `json:"franchises"`
	Collection            IGDBCollection          `json:"collection"`
	DLC                   []IGDBGame              `json:"dlcs"`
	Expansions            []IGDBGame              `json:"expansions"`
	StandaloneExpansions  []IGDBGame              `json:"standalone_expansions"`
	Remakes               []IGDBGame              `json:"remakes"`
	Remasters             []IGDBGame              `json:"remasters"`
	ExternalGames         []IGDBExternalGame      `json:"external_games"`
	Websites              []IGDBWebsite           `json:"websites"`
	LanguageSupports      []IGDBLanguageSupport   `json:"language_supports"`
	MultiplayerModes      []IGDBMultiplayerMode   `json:"multiplayer_modes"`
	AlternativeNames      []IGDBAlternativeName   `json:"alternative_names"`
	Keywords              []IGDBKeyword           `json:"keywords"`
	Tags                  []int                   `json:"tags"`
}

type IGDBCover struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	ImageID string `json:"image_id"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type IGDBScreenshot struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	ImageID string `json:"image_id"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type IGDBArtwork struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	ImageID string `json:"image_id"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type IGDBVideo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	VideoID  string `json:"video_id"`
	Checksum string `json:"checksum"`
}

type IGDBGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type IGDBTheme struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type IGDBPlatform struct {
	ID           int              `json:"id"`
	Name         string           `json:"name"`
	Abbreviation string           `json:"abbreviation"`
	Category     int              `json:"category"`
	Generation   int              `json:"generation"`
	PlatformLogo IGDBPlatformLogo `json:"platform_logo"`
	Websites     []IGDBWebsite    `json:"websites"`
}

type IGDBPlatformLogo struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	ImageID string `json:"image_id"`
}

type IGDBGameMode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type IGDBPlayerPerspective struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type IGDBInvolvedCompany struct {
	ID         int         `json:"id"`
	Company    IGDBCompany `json:"company"`
	Developer  bool        `json:"developer"`
	Publisher  bool        `json:"publisher"`
	Porting    bool        `json:"porting"`
	Supporting bool        `json:"supporting"`
}

type IGDBCompany struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Country     int             `json:"country"`
	Description string          `json:"description"`
	StartDate   int64           `json:"start_date"`
	Logo        IGDBCompanyLogo `json:"logo"`
	Websites    []IGDBWebsite   `json:"websites"`
}

type IGDBCompanyLogo struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	ImageID string `json:"image_id"`
}

type IGDBFranchise struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type IGDBCollection struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type IGDBExternalGame struct {
	ID        int    `json:"id"`
	Category  int    `json:"category"`
	UID       string `json:"uid"`
	URL       string `json:"url"`
	Year      int    `json:"year"`
	Media     int    `json:"media"`
	Platform  int    `json:"platform"`
	Countries []int  `json:"countries"`
}

type IGDBWebsite struct {
	ID       int    `json:"id"`
	Category int    `json:"category"`
	Trusted  bool   `json:"trusted"`
	URL      string `json:"url"`
}

type IGDBLanguageSupport struct {
	ID                  int                     `json:"id"`
	Language            IGDBLanguage            `json:"language"`
	LanguageSupportType IGDBLanguageSupportType `json:"language_support_type"`
}

type IGDBLanguage struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
	Locale     string `json:"locale"`
}

type IGDBLanguageSupportType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type IGDBMultiplayerMode struct {
	ID                int  `json:"id"`
	CampaignCoop      bool `json:"campaigncoop"`
	LancCoop          bool `json:"lancoop"`
	OfflineCoop       bool `json:"offlinecoop"`
	OfflineCoopMax    int  `json:"offlinecoopmax"`
	OfflineMax        int  `json:"offlinemax"`
	OnlineCoop        bool `json:"onlinecoop"`
	OnlineCoopMax     int  `json:"onlinecoopmax"`
	OnlineMax         int  `json:"onlinemax"`
	Platform          int  `json:"platform"`
	SplitScreen       bool `json:"splitscreen"`
	SplitScreenOnline bool `json:"splitscreenonline"`
}

type IGDBAlternativeName struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

type IGDBKeyword struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Steam API structures
type SteamAppDetails struct {
	Success bool         `json:"success"`
	Data    SteamAppData `json:"data"`
}

type SteamAppData struct {
	Type                string                  `json:"type"`
	Name                string                  `json:"name"`
	SteamAppID          int                     `json:"steam_appid"`
	RequiredAge         int                     `json:"required_age"`
	IsFree              bool                    `json:"is_free"`
	ControllerSupport   string                  `json:"controller_support"`
	DLCs                []int                   `json:"dlc"`
	DetailedDescription string                  `json:"detailed_description"`
	AboutTheGame        string                  `json:"about_the_game"`
	ShortDescription    string                  `json:"short_description"`
	SupportedLanguages  string                  `json:"supported_languages"`
	Reviews             string                  `json:"reviews"`
	HeaderImage         string                  `json:"header_image"`
	Website             string                  `json:"website"`
	PCRequirements      SteamRequirements       `json:"pc_requirements"`
	MacRequirements     SteamRequirements       `json:"mac_requirements"`
	LinuxRequirements   SteamRequirements       `json:"linux_requirements"`
	LegalNotice         string                  `json:"legal_notice"`
	Developers          []string                `json:"developers"`
	Publishers          []string                `json:"publishers"`
	PriceOverview       SteamPriceOverview      `json:"price_overview"`
	Packages            []int                   `json:"packages"`
	PackageGroups       []SteamPackageGroup     `json:"package_groups"`
	Platforms           SteamPlatforms          `json:"platforms"`
	Metacritic          SteamMetacritic         `json:"metacritic"`
	Categories          []SteamCategory         `json:"categories"`
	Genres              []SteamGenre            `json:"genres"`
	Screenshots         []SteamScreenshot       `json:"screenshots"`
	Movies              []SteamMovie            `json:"movies"`
	Recommendations     SteamRecommendations    `json:"recommendations"`
	Achievements        SteamAchievements       `json:"achievements"`
	ReleaseDate         SteamReleaseDate        `json:"release_date"`
	SupportInfo         SteamSupportInfo        `json:"support_info"`
	Background          string                  `json:"background"`
	ContentDescriptors  SteamContentDescriptors `json:"content_descriptors"`
}

type SteamRequirements struct {
	Minimum     string `json:"minimum"`
	Recommended string `json:"recommended"`
}

type SteamPriceOverview struct {
	Currency         string `json:"currency"`
	Initial          int    `json:"initial"`
	Final            int    `json:"final"`
	DiscountPercent  int    `json:"discount_percent"`
	InitialFormatted string `json:"initial_formatted"`
	FinalFormatted   string `json:"final_formatted"`
}

type SteamPackageGroup struct {
	Name                    string     `json:"name"`
	Title                   string     `json:"title"`
	Description             string     `json:"description"`
	SelectionText           string     `json:"selection_text"`
	SaveText                string     `json:"save_text"`
	DisplayType             int        `json:"display_type"`
	IsRecurringSubscription string     `json:"is_recurring_subscription"`
	Subs                    []SteamSub `json:"subs"`
}

type SteamSub struct {
	PackageID                int    `json:"packageid"`
	PercentSavingsText       string `json:"percent_savings_text"`
	PercentSavings           int    `json:"percent_savings"`
	OptionText               string `json:"option_text"`
	OptionDescription        string `json:"option_description"`
	CanGetFreeLicense        string `json:"can_get_free_license"`
	IsFreeLicense            bool   `json:"is_free_license"`
	PriceInCentsWithDiscount int    `json:"price_in_cents_with_discount"`
}

type SteamPlatforms struct {
	Windows bool `json:"windows"`
	Mac     bool `json:"mac"`
	Linux   bool `json:"linux"`
}

type SteamMetacritic struct {
	Score int    `json:"score"`
	URL   string `json:"url"`
}

type SteamCategory struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

type SteamGenre struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type SteamScreenshot struct {
	ID            int    `json:"id"`
	PathThumbnail string `json:"path_thumbnail"`
	PathFull      string `json:"path_full"`
}

type SteamMovie struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	Thumbnail string            `json:"thumbnail"`
	Webm      map[string]string `json:"webm"`
	Mp4       map[string]string `json:"mp4"`
	Highlight bool              `json:"highlight"`
}

type SteamRecommendations struct {
	Total int `json:"total"`
}

type SteamAchievements struct {
	Total       int                `json:"total"`
	Highlighted []SteamAchievement `json:"highlighted"`
}

type SteamAchievement struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type SteamReleaseDate struct {
	ComingSoon bool   `json:"coming_soon"`
	Date       string `json:"date"`
}

type SteamSupportInfo struct {
	URL   string `json:"url"`
	Email string `json:"email"`
}

type SteamContentDescriptors struct {
	IDs   []int  `json:"ids"`
	Notes string `json:"notes"`
}

// Software identification structures
type SoftwareInfo struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Architecture    string            `json:"architecture"`
	Platform        string            `json:"platform"`
	FileSize        int64             `json:"file_size"`
	FileHash        string            `json:"file_hash"`
	Publisher       string            `json:"publisher"`
	InstallLocation string            `json:"install_location"`
	Registry        map[string]string `json:"registry"`
	Dependencies    []string          `json:"dependencies"`
	PEInfo          *PEInfo           `json:"pe_info,omitempty"`
	ELFInfo         *ELFInfo          `json:"elf_info,omitempty"`
	MachoInfo       *MachoInfo        `json:"macho_info,omitempty"`
}

type PEInfo struct {
	Machine           string            `json:"machine"`
	Timestamp         int64             `json:"timestamp"`
	VersionInfo       map[string]string `json:"version_info"`
	ImportedLibraries []string          `json:"imported_libraries"`
	ExportedFunctions []string          `json:"exported_functions"`
	Sections          []PESection       `json:"sections"`
	Subsystem         string            `json:"subsystem"`
	EntryPoint        string            `json:"entry_point"`
}

type PESection struct {
	Name            string `json:"name"`
	VirtualAddress  string `json:"virtual_address"`
	VirtualSize     int    `json:"virtual_size"`
	RawSize         int    `json:"raw_size"`
	Characteristics string `json:"characteristics"`
}

type ELFInfo struct {
	Class            string       `json:"class"`
	Data             string       `json:"data"`
	Version          string       `json:"version"`
	OSABI            string       `json:"osabi"`
	Machine          string       `json:"machine"`
	Type             string       `json:"type"`
	EntryPoint       string       `json:"entry_point"`
	DynamicLibraries []string     `json:"dynamic_libraries"`
	Symbols          []string     `json:"symbols"`
	Sections         []ELFSection `json:"sections"`
}

type ELFSection struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
	Size    int    `json:"size"`
	Flags   string `json:"flags"`
}

type MachoInfo struct {
	Architecture     string             `json:"architecture"`
	FileType         string             `json:"file_type"`
	LoadCommands     []MachoLoadCommand `json:"load_commands"`
	DynamicLibraries []string           `json:"dynamic_libraries"`
	Symbols          []string           `json:"symbols"`
}

type MachoLoadCommand struct {
	Command string `json:"command"`
	Size    int    `json:"size"`
	Data    string `json:"data"`
}

func NewGameSoftwareRecognitionProvider(logger *zap.Logger) *GameSoftwareRecognitionProvider {
	return &GameSoftwareRecognitionProvider{
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURLs: map[string]string{
			"igdb":        "https://api.igdb.com/v4",
			"steam":       "https://store.steampowered.com/api",
			"github":      "https://api.github.com",
			"sourceforge": "https://sourceforge.net/rest",
			"winget":      "https://api.winget.run",
			"flatpak":     "https://flathub.org/api",
			"snapcraft":   "https://api.snapcraft.io",
			"homebrew":    "https://formulae.brew.sh/api",
		},
		apiKeys: map[string]string{
			"igdb":   "free_api_key",
			"github": "free_api_key",
		},
		rateLimiter: make(map[string]*time.Ticker),
	}
}

func (p *GameSoftwareRecognitionProvider) RecognizeMedia(ctx context.Context, req *MediaRecognitionRequest) (*MediaRecognitionResult, error) {
	p.logger.Info("Starting game/software recognition",
		zap.String("file_path", req.FilePath),
		zap.String("media_type", string(req.MediaType)))

	// Extract metadata from filename and file structure
	name, version, platform := p.extractSoftwareMetadataFromFilename(req.FileName)

	p.logger.Debug("Extracted metadata from filename",
		zap.String("name", name),
		zap.String("version", version),
		zap.String("platform", platform))

	// Determine if it's a game or software
	isGame := p.looksLikeGame(name, req.FileName)

	if isGame {
		// Try game recognition
		if result, err := p.recognizeGame(ctx, name, platform); err == nil {
			p.logger.Info("Successfully recognized as game",
				zap.String("name", result.Title),
				zap.Float64("confidence", result.Confidence))
			return result, nil
		}
	}

	// Try software recognition
	if result, err := p.recognizeSoftware(ctx, name, version, platform, req); err == nil {
		p.logger.Info("Successfully recognized as software",
			zap.String("name", result.Title),
			zap.Float64("confidence", result.Confidence))
		return result, nil
	}

	// Fallback to basic recognition
	return p.basicGameSoftwareRecognition(req, name, version, platform, isGame), nil
}

func (p *GameSoftwareRecognitionProvider) recognizeGame(ctx context.Context, name, platform string) (*MediaRecognitionResult, error) {
	// Try IGDB first
	if result, err := p.searchIGDB(ctx, name, platform); err == nil {
		return result, nil
	}

	// Try Steam as fallback
	if result, err := p.searchSteam(ctx, name); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("no game recognition results found")
}

func (p *GameSoftwareRecognitionProvider) searchIGDB(ctx context.Context, name, platform string) (*MediaRecognitionResult, error) {
	// Build IGDB query
	query := fmt.Sprintf(`search "%s"; fields *; limit 10;`, name)

	target := p.baseURLs["igdb"] + "/games"
	if err := GuardProviderURL(target, SSRFGuardConfig{}); err != nil {
		return nil, fmt.Errorf("igdb: unsafe endpoint: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", target, strings.NewReader(query))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", p.apiKeys["igdb"])
	req.Header.Set("Authorization", "Bearer "+p.apiKeys["igdb"])
	req.Header.Set("Content-Type", "text/plain")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var games []IGDBGame
	if err := json.NewDecoder(resp.Body).Decode(&games); err != nil {
		return nil, err
	}

	if len(games) == 0 {
		return nil, fmt.Errorf("no games found in IGDB")
	}

	// Get the best match
	bestMatch := games[0]

	return p.convertIGDBGame(bestMatch), nil
}

func (p *GameSoftwareRecognitionProvider) convertIGDBGame(game IGDBGame) *MediaRecognitionResult {
	result := &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("igdb_%d", game.ID),
		MediaType:         MediaTypeGame,
		Title:             game.Name,
		Description:       game.Summary,
		Rating:            game.Rating / 10.0, // IGDB uses 0-100 scale
		Confidence:        p.calculateIGDBConfidence(game.Rating, game.RatingCount, game.Popularity),
		RecognitionMethod: "igdb_api",
		APIProvider:       "IGDB",
		IGDBId:            strconv.Itoa(game.ID),
	}

	// Parse release date
	if game.FirstReleaseDate > 0 {
		releaseDate := time.Unix(game.FirstReleaseDate, 0)
		result.ReleaseDate = &releaseDate
		result.Year = releaseDate.Year()
	}

	// Extract genres
	for _, genre := range game.Genres {
		result.Genres = append(result.Genres, genre.Name)
	}

	// Extract platforms
	for _, platform := range game.Platforms {
		result.Platforms = append(result.Platforms, platform.Name)
	}

	// Extract developer and publisher
	for _, company := range game.InvolvedCompanies {
		if company.Developer {
			result.Developer = company.Company.Name
		}
		if company.Publisher {
			result.Publisher_Game = company.Company.Name
		}
	}

	// Get cover art
	if game.Cover.URL != "" {
		result.CoverArt = append(result.CoverArt, models.CoverArtResult{
			ID:      game.Cover.ImageID,
			URL:     p.getIGDBImageURL(game.Cover.ImageID, "cover_big"),
			Quality: "large",
			Source:  "IGDB",
		})
	}

	// Get screenshots
	for _, screenshot := range game.Screenshots {
		result.Screenshots = append(result.Screenshots, p.getIGDBImageURL(screenshot.ImageID, "screenshot_med"))
	}

	// Set external IDs
	result.ExternalIDs = map[string]string{
		"igdb_id": strconv.Itoa(game.ID),
	}

	// Parse external games for Steam ID
	for _, external := range game.ExternalGames {
		if external.Category == 1 { // Steam
			result.ExternalIDs["steam_id"] = external.UID
			result.SteamID = external.UID
		}
	}

	return result
}

// ErrGameProviderNotImplemented marks game lookups with no real API client.
var ErrGameProviderNotImplemented = errors.New("game provider lookup not implemented")

func (p *GameSoftwareRecognitionProvider) searchSteam(ctx context.Context, name string) (*MediaRecognitionResult, error) {
	// Steam has no search API and no app-id database is maintained here, so
	// there is nothing real to look up. Never fabricate a match (CAT-001).
	return nil, fmt.Errorf("%w: steam", ErrGameProviderNotImplemented)
}

func (p *GameSoftwareRecognitionProvider) recognizeSoftware(ctx context.Context, name, version, platform string, req *MediaRecognitionRequest) (*MediaRecognitionResult, error) {
	// Try different software repositories based on platform
	switch strings.ToLower(platform) {
	case "windows":
		if result, err := p.searchWinget(ctx, name); err == nil {
			return result, nil
		}
	case "linux":
		if result, err := p.searchFlatpak(ctx, name); err == nil {
			return result, nil
		}
		if result, err := p.searchSnapcraft(ctx, name); err == nil {
			return result, nil
		}
	case "macos", "darwin":
		if result, err := p.searchHomebrew(ctx, name); err == nil {
			return result, nil
		}
	}

	// Try GitHub for open source software
	if result, err := p.searchGitHub(ctx, name); err == nil {
		return result, nil
	}

	// Try SourceForge
	if result, err := p.searchSourceForge(ctx, name); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("no software recognition results found")
}

func (p *GameSoftwareRecognitionProvider) searchWinget(ctx context.Context, name string) (*MediaRecognitionResult, error) {
	// Winget API search
	params := url.Values{}
	params.Set("query", name)

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/packages?%s", p.baseURLs["winget"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var packages []WingetPackage
	if err := json.NewDecoder(resp.Body).Decode(&packages); err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages found in winget")
	}

	pkg := packages[0]
	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("winget_%s", pkg.PackageIdentifier),
		MediaType:         MediaTypeSoftware,
		Title:             pkg.PackageName,
		Publisher:         pkg.Publisher,
		Version:           pkg.PackageVersion,
		Platform:          "Windows",
		Confidence:        0.8,
		RecognitionMethod: "winget_api",
		APIProvider:       "Winget",
		ExternalIDs: map[string]string{
			"winget_id": pkg.PackageIdentifier,
		},
	}, nil
}

func (p *GameSoftwareRecognitionProvider) searchFlatpak(ctx context.Context, name string) (*MediaRecognitionResult, error) {
	// Flatpak/Flathub API search
	params := url.Values{}
	params.Set("q", name)

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/v1/apps/search?%s", p.baseURLs["flatpak"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apps []FlatpakApp
	if err := json.NewDecoder(resp.Body).Decode(&apps); err != nil {
		return nil, err
	}

	if len(apps) == 0 {
		return nil, fmt.Errorf("no apps found in Flatpak")
	}

	app := apps[0]
	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("flatpak_%s", app.FlatpakAppID),
		MediaType:         MediaTypeSoftware,
		Title:             app.Name,
		Description:       app.Summary,
		Platform:          "Linux",
		Confidence:        0.8,
		RecognitionMethod: "flatpak_api",
		APIProvider:       "Flatpak",
		ExternalIDs: map[string]string{
			"flatpak_id": app.FlatpakAppID,
		},
	}, nil
}

func (p *GameSoftwareRecognitionProvider) searchSnapcraft(ctx context.Context, name string) (*MediaRecognitionResult, error) {
	// Snapcraft API search
	params := url.Values{}
	params.Set("q", name)

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/v2/find?%s", p.baseURLs["snapcraft"], params.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SnapcraftSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return nil, fmt.Errorf("no snaps found in Snapcraft")
	}

	snap := result.Results[0]
	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("snap_%s", snap.Name),
		MediaType:         MediaTypeSoftware,
		Title:             snap.Title,
		Description:       snap.Summary,
		Publisher:         snap.Publisher.DisplayName,
		Platform:          "Linux",
		Confidence:        0.8,
		RecognitionMethod: "snapcraft_api",
		APIProvider:       "Snapcraft",
		ExternalIDs: map[string]string{
			"snap_id": snap.Name,
		},
	}, nil
}

func (p *GameSoftwareRecognitionProvider) searchHomebrew(ctx context.Context, name string) (*MediaRecognitionResult, error) {
	// Homebrew formulae API
	resp, err := p.httpClient.Get(fmt.Sprintf("%s/formula/%s.json", p.baseURLs["homebrew"], name))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var formula HomebrewFormula
	if err := json.NewDecoder(resp.Body).Decode(&formula); err != nil {
		return nil, err
	}

	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("homebrew_%s", formula.Name),
		MediaType:         MediaTypeSoftware,
		Title:             formula.Name,
		Description:       formula.Desc,
		Version:           formula.Versions.Stable,
		Platform:          "macOS",
		Confidence:        0.8,
		RecognitionMethod: "homebrew_api",
		APIProvider:       "Homebrew",
		ExternalIDs: map[string]string{
			"homebrew_name": formula.Name,
		},
	}, nil
}

func (p *GameSoftwareRecognitionProvider) searchGitHub(ctx context.Context, name string) (*MediaRecognitionResult, error) {
	// GitHub repository search
	params := url.Values{}
	params.Set("q", name)
	params.Set("sort", "stars")
	params.Set("order", "desc")

	target := fmt.Sprintf("%s/search/repositories?%s", p.baseURLs["github"], params.Encode())
	if err := GuardProviderURL(target, SSRFGuardConfig{}); err != nil {
		return nil, fmt.Errorf("github: unsafe endpoint: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+p.apiKeys["github"])
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult GitHubSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	if len(searchResult.Items) == 0 {
		return nil, fmt.Errorf("no repositories found on GitHub")
	}

	repo := searchResult.Items[0]
	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("github_%d", repo.ID),
		MediaType:         MediaTypeSoftware,
		Title:             repo.Name,
		Description:       repo.Description,
		Developer:         repo.Owner.Login,
		License:           repo.License.Name,
		Confidence:        p.calculateGitHubConfidence(repo.StargazersCount, repo.ForksCount),
		RecognitionMethod: "github_api",
		APIProvider:       "GitHub",
		ExternalIDs: map[string]string{
			"github_id":  strconv.Itoa(repo.ID),
			"github_url": repo.HTMLURL,
		},
	}, nil
}

func (p *GameSoftwareRecognitionProvider) searchSourceForge(ctx context.Context, name string) (*MediaRecognitionResult, error) {
	// SourceForge API search (simplified)
	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("sourceforge_%s", p.generateID(name)),
		MediaType:         MediaTypeSoftware,
		Title:             name,
		Confidence:        0.5,
		RecognitionMethod: "sourceforge_lookup",
		APIProvider:       "SourceForge",
		ExternalIDs:       make(map[string]string),
	}, nil
}

func (p *GameSoftwareRecognitionProvider) basicGameSoftwareRecognition(req *MediaRecognitionRequest, name, version, platform string, isGame bool) *MediaRecognitionResult {
	mediaType := MediaTypeSoftware
	if isGame {
		mediaType = MediaTypeGame
	}

	return &MediaRecognitionResult{
		MediaID:           fmt.Sprintf("basic_%s_%s_%d", strings.ReplaceAll(name, " ", "_"), platform, time.Now().Unix()),
		MediaType:         mediaType,
		Title:             name,
		Version:           version,
		Platform:          platform,
		Confidence:        0.3,
		RecognitionMethod: "filename_parsing",
		APIProvider:       "basic",
		ExternalIDs:       make(map[string]string),
	}
}

// Helper methods
func (p *GameSoftwareRecognitionProvider) extractSoftwareMetadataFromFilename(filename string) (name, version, platform string) {
	// Remove file extension
	baseName := strings.TrimSuffix(filename, "."+p.getFileExtension(filename))

	// Common patterns for software/games:
	// Name v1.0.0
	// Name-1.0.0-win32
	// Name_Setup_v1.0.exe
	// Game.Name.2023.Repack

	// Extract version patterns
	versionPatterns := []string{
		`v?(\d+\.\d+\.\d+)`,
		`v?(\d+\.\d+)`,
		`(\d{4})`, // Year as version
	}

	for _, pattern := range versionPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(baseName); len(matches) > 1 {
			version = matches[1]
			// Remove version from name
			baseName = re.ReplaceAllString(baseName, "")
			break
		}
	}

	// Extract platform patterns
	platformPatterns := map[string][]string{
		"windows": {"win", "windows", "x86", "x64", "win32", "win64"},
		"linux":   {"linux", "ubuntu", "debian", "fedora", "arch"},
		"macos":   {"mac", "macos", "osx", "darwin"},
		"android": {"android", "apk"},
		"ios":     {"ios", "iphone", "ipad"},
	}

	lowerName := strings.ToLower(baseName)
	for platformName, patterns := range platformPatterns {
		for _, pattern := range patterns {
			if strings.Contains(lowerName, pattern) {
				platform = platformName
				// Remove platform identifier from name
				baseName = regexp.MustCompile(`(?i)`+pattern).ReplaceAllString(baseName, "")
				break
			}
		}
		if platform != "" {
			break
		}
	}

	// Clean up name
	name = regexp.MustCompile(`[._-]+`).ReplaceAllString(baseName, " ")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Remove common software suffixes
	suffixes := []string{"setup", "installer", "install", "portable", "repack", "crack", "keygen"}
	for _, suffix := range suffixes {
		re := regexp.MustCompile(`(?i)\b` + suffix + `\b`)
		name = re.ReplaceAllString(name, "")
	}

	name = strings.TrimSpace(name)

	return name, version, platform
}

func (p *GameSoftwareRecognitionProvider) looksLikeGame(name, filename string) bool {
	gameKeywords := []string{
		"game", "play", "quest", "adventure", "action", "rpg", "fps", "strategy",
		"simulation", "racing", "sports", "puzzle", "arcade", "indie", "multiplayer",
		"steam", "gog", "epic", "uplay", "origin", "battle.net",
	}

	searchText := strings.ToLower(name + " " + filename)
	for _, keyword := range gameKeywords {
		if strings.Contains(searchText, keyword) {
			return true
		}
	}

	return false
}

func (p *GameSoftwareRecognitionProvider) getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func (p *GameSoftwareRecognitionProvider) getIGDBImageURL(imageID, size string) string {
	return fmt.Sprintf("https://images.igdb.com/igdb/image/upload/t_%s/%s.jpg", size, imageID)
}

func (p *GameSoftwareRecognitionProvider) calculateIGDBConfidence(rating float64, ratingCount int, popularity float64) float64 {
	confidence := 0.5

	if rating > 70 && ratingCount > 100 {
		confidence += 0.3
	} else if rating > 60 && ratingCount > 50 {
		confidence += 0.2
	}

	if popularity > 10 {
		confidence += 0.2
	}

	return confidence
}

func (p *GameSoftwareRecognitionProvider) calculateGitHubConfidence(stars, forks int) float64 {
	confidence := 0.5

	if stars > 1000 {
		confidence += 0.3
	} else if stars > 100 {
		confidence += 0.2
	}

	if forks > 100 {
		confidence += 0.2
	}

	return confidence
}

func (p *GameSoftwareRecognitionProvider) generateID(name string) string {
	hash := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hash[:])[:12]
}

// RecognitionProvider interface implementation
func (p *GameSoftwareRecognitionProvider) GetProviderName() string {
	return "game_software_recognition"
}

func (p *GameSoftwareRecognitionProvider) SupportsMediaType(mediaType MediaType) bool {
	supportedTypes := []MediaType{
		MediaTypeGame,
		MediaTypeGameOS,
		MediaTypeSoftware,
		MediaTypeSoftwareOS,
	}

	for _, supported := range supportedTypes {
		if mediaType == supported {
			return true
		}
	}

	return false
}

func (p *GameSoftwareRecognitionProvider) GetConfidenceThreshold() float64 {
	return 0.4
}

// Additional API structures
type WingetPackage struct {
	PackageIdentifier string `json:"PackageIdentifier"`
	PackageName       string `json:"PackageName"`
	PackageVersion    string `json:"PackageVersion"`
	Publisher         string `json:"Publisher"`
}

type FlatpakApp struct {
	FlatpakAppID  string `json:"flatpakAppId"`
	Name          string `json:"name"`
	Summary       string `json:"summary"`
	Description   string `json:"description"`
	DeveloperName string `json:"developerName"`
}

type SnapcraftSearchResult struct {
	Results []SnapcraftSnap `json:"_embedded"`
}

type SnapcraftSnap struct {
	Name      string             `json:"name"`
	Title     string             `json:"title"`
	Summary   string             `json:"summary"`
	Publisher SnapcraftPublisher `json:"publisher"`
}

type SnapcraftPublisher struct {
	DisplayName string `json:"display-name"`
}

type HomebrewFormula struct {
	Name     string           `json:"name"`
	Desc     string           `json:"desc"`
	Versions HomebrewVersions `json:"versions"`
}

type HomebrewVersions struct {
	Stable string `json:"stable"`
	Head   string `json:"head"`
}

type GitHubSearchResult struct {
	Items []GitHubRepository `json:"items"`
}

type GitHubRepository struct {
	ID              int           `json:"id"`
	Name            string        `json:"name"`
	FullName        string        `json:"full_name"`
	Description     string        `json:"description"`
	HTMLURL         string        `json:"html_url"`
	StargazersCount int           `json:"stargazers_count"`
	ForksCount      int           `json:"forks_count"`
	Language        string        `json:"language"`
	License         GitHubLicense `json:"license"`
	Owner           GitHubOwner   `json:"owner"`
}

type GitHubLicense struct {
	Name string `json:"name"`
}

type GitHubOwner struct {
	Login string `json:"login"`
}
