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
// RecognizeMedia — top-level recognition for games and software
// =============================================================================

func TestGameSoftwareRecognitionProvider_RecognizeMedia_GameRecognized(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	igdbResp := []IGDBGame{
		{
			ID:               12345,
			Name:             "The Witcher 3",
			Summary:          "An action RPG",
			FirstReleaseDate: 1431993600, // 2015-05-19
			Rating:           92.5,
			RatingCount:      500,
			Popularity:       80.0,
			Genres:           []IGDBGenre{{ID: 1, Name: "RPG"}},
			Platforms:        []IGDBPlatform{{ID: 1, Name: "PC"}},
			InvolvedCompanies: []IGDBInvolvedCompany{
				{Company: IGDBCompany{Name: "CD Projekt Red"}, Developer: true},
				{Company: IGDBCompany{Name: "CD Projekt"}, Publisher: true},
			},
			Cover: IGDBCover{ImageID: "co1234", URL: "//images.igdb.com/co1234.jpg"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(igdbResp)
	}))
	defer ts.Close()

	provider.baseURLs["igdb"] = ts.URL

	req := &MediaRecognitionRequest{
		FilePath:  "/games/The.Witcher.3.Wild.Hunt.GOG.exe",
		FileName:  "The.Witcher.3.Wild.Hunt.GOG.exe",
		MediaType: MediaTypeGame,
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "The Witcher 3", result.Title)
	assert.Equal(t, "IGDB", result.APIProvider)
	assert.Contains(t, result.Genres, "RPG")
	assert.Equal(t, "CD Projekt Red", result.Developer)
}

func TestGameSoftwareRecognitionProvider_RecognizeMedia_SoftwareFallback(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	// Most APIs fail but searchSourceForge is a mock that always returns a result,
	// so recognizeSoftware succeeds via SourceForge fallback.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["igdb"] = ts.URL
	provider.baseURLs["steam"] = ts.URL
	provider.baseURLs["github"] = ts.URL
	provider.baseURLs["winget"] = ts.URL
	provider.baseURLs["flatpak"] = ts.URL
	provider.baseURLs["snapcraft"] = ts.URL
	provider.baseURLs["homebrew"] = ts.URL

	req := &MediaRecognitionRequest{
		FilePath:  "/software/Visual.Studio.Code.1.85.2.Setup.exe",
		FileName:  "Visual.Studio.Code.1.85.2.Setup.exe",
		MediaType: MediaTypeSoftware,
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, MediaTypeSoftware, result.MediaType)
	// SourceForge mock provides the result
	assert.Equal(t, "SourceForge", result.APIProvider)
}

func TestGameSoftwareRecognitionProvider_RecognizeMedia_IGDBFailNeverClaimsSteam(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	// IGDB fails
	tsIGDB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsIGDB.Close()

	provider.baseURLs["igdb"] = tsIGDB.URL

	req := &MediaRecognitionRequest{
		FilePath:  "/games/Some.Game.Steam.exe",
		FileName:  "Some.Game.Steam.exe",
		MediaType: MediaTypeGame,
	}

	result, err := provider.RecognizeMedia(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	// IGDB failed and Steam has no client: a Steam match must never be invented.
	assert.NotEqual(t, "Steam", result.APIProvider)
}

// =============================================================================
// recognizeGame — game recognition pipeline
// =============================================================================

func TestGameSoftwareRecognitionProvider_RecognizeGame_IGDBSuccess(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	igdbResp := []IGDBGame{
		{
			ID:      99,
			Name:    "Test Game",
			Summary: "A test game",
			Rating:  80.0,
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(igdbResp)
	}))
	defer ts.Close()

	provider.baseURLs["igdb"] = ts.URL

	result, err := provider.recognizeGame(context.Background(), "Test Game", "PC")
	require.NoError(t, err)
	assert.Equal(t, "Test Game", result.Title)
}

func TestGameSoftwareRecognitionProvider_RecognizeGame_IGDBFailReturnsError(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["igdb"] = ts.URL

	result, err := provider.recognizeGame(context.Background(), "Steam Game", "PC")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// =============================================================================
// searchIGDB — IGDB API interaction
// =============================================================================

func TestGameSoftwareRecognitionProvider_SearchIGDB_Success(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	igdbResp := []IGDBGame{
		{
			ID:               1,
			Name:             "IGDB Game",
			Summary:          "Found via IGDB",
			FirstReleaseDate: 1609459200,
			Rating:           85.0,
			RatingCount:      200,
			Genres:           []IGDBGenre{{Name: "Action"}},
			Platforms:        []IGDBPlatform{{Name: "PlayStation 5"}},
			Cover:            IGDBCover{ImageID: "co_test", URL: "//images.igdb.com/co_test.jpg"},
			Screenshots:      []IGDBScreenshot{{ImageID: "ss_001"}},
			ExternalGames:    []IGDBExternalGame{{Category: 1, UID: "steam_123"}},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		json.NewEncoder(w).Encode(igdbResp)
	}))
	defer ts.Close()

	provider.baseURLs["igdb"] = ts.URL

	result, err := provider.searchIGDB(context.Background(), "IGDB Game", "PS5")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "IGDB Game", result.Title)
	assert.Equal(t, "IGDB", result.APIProvider)
	assert.Contains(t, result.Genres, "Action")
	assert.Contains(t, result.Platforms, "PlayStation 5")
	assert.NotEmpty(t, result.CoverArt)
	assert.NotEmpty(t, result.Screenshots)
	assert.Equal(t, "steam_123", result.SteamID)
}

func TestGameSoftwareRecognitionProvider_SearchIGDB_NoResults(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]IGDBGame{})
	}))
	defer ts.Close()

	provider.baseURLs["igdb"] = ts.URL

	_, err := provider.searchIGDB(context.Background(), "Nonexistent Game", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no games found")
}

func TestGameSoftwareRecognitionProvider_SearchIGDB_HTTPError(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	provider.baseURLs["igdb"] = "http://localhost:1"

	_, err := provider.searchIGDB(context.Background(), "Test", "")
	assert.Error(t, err)
}

func TestGameSoftwareRecognitionProvider_SearchIGDB_InvalidJSON(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	provider.baseURLs["igdb"] = ts.URL

	_, err := provider.searchIGDB(context.Background(), "Test", "")
	assert.Error(t, err)
}

// =============================================================================
// convertIGDBGame — conversion of IGDB data to MediaRecognitionResult
// =============================================================================

func TestGameSoftwareRecognitionProvider_ConvertIGDBGame_Full(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	game := IGDBGame{
		ID:               42,
		Name:             "Full Game",
		Summary:          "A full game description",
		FirstReleaseDate: 1609459200,
		Rating:           90.0,
		RatingCount:      1000,
		Popularity:       95.0,
		Genres:           []IGDBGenre{{Name: "Action"}, {Name: "RPG"}},
		Platforms:        []IGDBPlatform{{Name: "PC"}, {Name: "PS5"}},
		InvolvedCompanies: []IGDBInvolvedCompany{
			{Company: IGDBCompany{Name: "Dev Studio"}, Developer: true},
			{Company: IGDBCompany{Name: "Publisher Inc"}, Publisher: true},
		},
		Cover:       IGDBCover{ImageID: "co_full", URL: "//images.igdb.com/co_full.jpg"},
		Screenshots: []IGDBScreenshot{{ImageID: "ss_1"}, {ImageID: "ss_2"}},
		ExternalGames: []IGDBExternalGame{
			{Category: 1, UID: "steam_42"},
			{Category: 5, UID: "gog_42"},
		},
	}

	result := provider.convertIGDBGame(game)

	require.NotNil(t, result)
	assert.Equal(t, "igdb_42", result.MediaID)
	assert.Equal(t, "Full Game", result.Title)
	assert.Equal(t, "A full game description", result.Description)
	assert.Equal(t, MediaTypeGame, result.MediaType)
	assert.Equal(t, 2021, result.Year)
	assert.NotNil(t, result.ReleaseDate)
	assert.Contains(t, result.Genres, "Action")
	assert.Contains(t, result.Genres, "RPG")
	assert.Contains(t, result.Platforms, "PC")
	assert.Contains(t, result.Platforms, "PS5")
	assert.Equal(t, "Dev Studio", result.Developer)
	assert.Equal(t, "Publisher Inc", result.Publisher_Game)
	assert.NotEmpty(t, result.CoverArt)
	assert.Len(t, result.Screenshots, 2)
	assert.Equal(t, "steam_42", result.SteamID)
	assert.Equal(t, "42", result.ExternalIDs["igdb_id"])
}

func TestGameSoftwareRecognitionProvider_ConvertIGDBGame_Minimal(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	game := IGDBGame{
		ID:   1,
		Name: "Minimal Game",
	}

	result := provider.convertIGDBGame(game)

	require.NotNil(t, result)
	assert.Equal(t, "Minimal Game", result.Title)
	assert.Equal(t, MediaTypeGame, result.MediaType)
	assert.Equal(t, 0, result.Year)
	assert.Empty(t, result.Genres)
}

// =============================================================================
// searchSteam — Steam mock results
// =============================================================================

// CAT-001: Steam has no search client; claiming any name is a Steam game with
// 0.6 confidence was fabricated recognition.
func TestGameSoftwareRecognitionProvider_SearchSteam_NotFabricated(t *testing.T) {
	provider := NewGameSoftwareRecognitionProvider(zap.NewNop())

	result, err := provider.searchSteam(context.Background(), "Half-Life 2")
	assert.ErrorIs(t, err, ErrGameProviderNotImplemented)
	assert.Nil(t, result)
}

// =============================================================================
// basicGameSoftwareRecognition — fallback recognition
// =============================================================================

func TestGameSoftwareRecognitionProvider_BasicRecognition_Game(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	req := &MediaRecognitionRequest{
		FilePath: "/games/test.exe",
	}

	result := provider.basicGameSoftwareRecognition(req, "Test Game", "1.0", "windows", true)
	require.NotNil(t, result)
	assert.Equal(t, MediaTypeGame, result.MediaType)
	assert.Equal(t, "Test Game", result.Title)
	assert.Equal(t, "1.0", result.Version)
	assert.Equal(t, "windows", result.Platform)
	assert.Equal(t, 0.3, result.Confidence)
	assert.Equal(t, "filename_parsing", result.RecognitionMethod)
}

func TestGameSoftwareRecognitionProvider_BasicRecognition_Software(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	req := &MediaRecognitionRequest{
		FilePath: "/software/test.dmg",
	}

	result := provider.basicGameSoftwareRecognition(req, "Test App", "2.5", "macos", false)
	require.NotNil(t, result)
	assert.Equal(t, MediaTypeSoftware, result.MediaType)
	assert.Equal(t, "Test App", result.Title)
}

// =============================================================================
// getIGDBImageURL
// =============================================================================

func TestGameSoftwareRecognitionProvider_GetIGDBImageURL_Coverage(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	url := provider.getIGDBImageURL("co1234", "cover_big")
	assert.Equal(t, "https://images.igdb.com/igdb/image/upload/t_cover_big/co1234.jpg", url)

	url2 := provider.getIGDBImageURL("ss_abc", "screenshot_med")
	assert.Equal(t, "https://images.igdb.com/igdb/image/upload/t_screenshot_med/ss_abc.jpg", url2)
}

// =============================================================================
// recognizeSoftware — software recognition pipeline
// =============================================================================

func TestGameSoftwareRecognitionProvider_RecognizeSoftware_WindowsFallsBackToSourceForge(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["winget"] = ts.URL
	provider.baseURLs["github"] = ts.URL
	// searchSourceForge is a mock that always succeeds

	req := &MediaRecognitionRequest{FilePath: "/soft/test.exe"}
	result, err := provider.recognizeSoftware(context.Background(), "Test App", "1.0", "windows", req)
	require.NoError(t, err)
	assert.Equal(t, "SourceForge", result.APIProvider)
}

func TestGameSoftwareRecognitionProvider_RecognizeSoftware_LinuxFallsBackToSourceForge(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["flatpak"] = ts.URL
	provider.baseURLs["snapcraft"] = ts.URL
	provider.baseURLs["github"] = ts.URL
	// searchSourceForge is a mock that always succeeds

	req := &MediaRecognitionRequest{FilePath: "/soft/test.AppImage"}
	result, err := provider.recognizeSoftware(context.Background(), "Test App", "1.0", "linux", req)
	require.NoError(t, err)
	assert.Equal(t, "SourceForge", result.APIProvider)
}

func TestGameSoftwareRecognitionProvider_RecognizeSoftware_MacOSFallsBackToSourceForge(t *testing.T) {
	logger := zap.NewNop()
	provider := NewGameSoftwareRecognitionProvider(logger)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	provider.baseURLs["homebrew"] = ts.URL
	provider.baseURLs["github"] = ts.URL
	// searchSourceForge is a mock that always succeeds

	req := &MediaRecognitionRequest{FilePath: "/soft/test.dmg"}
	result, err := provider.recognizeSoftware(context.Background(), "Test App", "1.0", "macos", req)
	require.NoError(t, err)
	assert.Equal(t, "SourceForge", result.APIProvider)
}
