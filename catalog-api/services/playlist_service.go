package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"catalogizer/internal/smartrules"
	"catalogizer/models"
	"catalogizer/repository"
)

// ErrSmartPlaylistItems is returned when a manual item operation targets a smart playlist.
var ErrSmartPlaylistItems = errors.New("smart playlists have no manual items: their members come from the rules")

// ErrPlaylistForbidden is returned when the caller may not change the playlist.
var ErrPlaylistForbidden = errors.New("unauthorized: only the owner can change this playlist")

// ErrPlaylistNotFound is returned when the playlist does not exist OR the caller may not even read it
// (a private playlist of another user): both look the same, so existence is not leaked.
var ErrPlaylistNotFound = errors.New("playlist not found")

// ErrSmartLimit is returned when the user already owns MaxSmartPlaylistsPerUser smart playlists.
var ErrSmartLimit = errors.New("smart playlist limit reached")

// MaxSmartPlaylistsPerUser bounds the rule evaluations a single user can accumulate.
const MaxSmartPlaylistsPerUser = 50

// PlaylistService provides business logic for playlist operations.
type PlaylistService struct {
	playlistRepo *repository.PlaylistRepository
	// smartCreateMu serialises "count this user's smart playlists, then insert one" per user (SECQ-4 / SC7): without it
	// concurrent creates all read the same count and the user ends above MaxSmartPlaylistsPerUser.
	smartCreateMu sync.Map // userID (int) -> *sync.Mutex
}

// lockSmartCreate takes the per-user create lock and returns its release function.
func (s *PlaylistService) lockSmartCreate(userID int) func() {
	m, _ := s.smartCreateMu.LoadOrStore(userID, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// NewPlaylistService creates a new playlist service.
func NewPlaylistService(playlistRepo *repository.PlaylistRepository) *PlaylistService {
	return &PlaylistService{
		playlistRepo: playlistRepo,
	}
}

// CreatePlaylist creates a new playlist for the given user.
func (s *PlaylistService) CreatePlaylist(userID int, req *models.CreatePlaylistRequest) (*models.Playlist, error) {
	return s.CreatePlaylistCtx(context.Background(), userID, req)
}

// CreatePlaylistCtx is CreatePlaylist with the request context (bounds the rule evaluation).
func (s *PlaylistService) CreatePlaylistCtx(ctx context.Context, userID int, req *models.CreatePlaylistRequest) (*models.Playlist, error) {
	if s.playlistRepo == nil {
		return nil, fmt.Errorf("playlist repository not configured")
	}

	if req.Name == "" {
		return nil, fmt.Errorf("playlist name is required")
	}

	if req.IsSmart != nil && !*req.IsSmart && len(req.SmartRules) > 0 {
		return nil, &smartrules.ValidationError{Msg: "smart_rules given but is_smart is false"}
	}
	wantSmart := (req.IsSmart != nil && *req.IsSmart) || len(req.SmartRules) > 0

	now := time.Now()
	playlist := &models.Playlist{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsPublic:    req.IsPublic != nil && *req.IsPublic,
	}
	if wantSmart {
		rs, err := smartrules.Parse(req.SmartRules)
		if err != nil {
			return nil, err
		}
		unlock := s.lockSmartCreate(userID) // held through the insert below
		defer unlock()
		n, err := s.playlistRepo.CountSmartByUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to count smart playlists: %w", err)
		}
		if n >= MaxSmartPlaylistsPerUser {
			return nil, fmt.Errorf("%w (max %d per user)", ErrSmartLimit, MaxSmartPlaylistsPerUser)
		}
		// run the rules once: rules that cannot be evaluated are refused, not stored
		if _, _, err := s.playlistRepo.EvaluateSmart(ctx, 0, rs, int64(userID), 1, 0); err != nil {
			return nil, fmt.Errorf("smart playlist rules could not be evaluated: %w", err)
		}
		playlist.IsSmart = true
		playlist.SmartRules = json.RawMessage(rs.Raw)
	}

	id, err := s.playlistRepo.CreatePlaylist(playlist)
	if err != nil {
		return nil, fmt.Errorf("failed to create playlist: %w", err)
	}

	playlist.ID = id
	return playlist, nil
}

// GetPlaylist returns a playlist by ID, verifying the user has access.
// A smart playlist's members are evaluated now (first default page).
func (s *PlaylistService) GetPlaylist(userID int, playlistID int) (*models.Playlist, error) {
	return s.GetPlaylistPage(userID, playlistID, smartrules.MaxLimit, 0)
}

// GetPlaylistPage is GetPlaylist with an explicit page for smart playlists
// (limit is capped at smartrules.MaxLimit). Manual playlists return all items.
func (s *PlaylistService) GetPlaylistPage(userID int, playlistID int, limit, offset int) (*models.Playlist, error) {
	return s.GetPlaylistPageCtx(context.Background(), userID, playlistID, limit, offset)
}

// GetPlaylistPageCtx is GetPlaylistPage with the request context.
func (s *PlaylistService) GetPlaylistPageCtx(ctx context.Context, userID int, playlistID int, limit, offset int) (*models.Playlist, error) {
	if s.playlistRepo == nil {
		return nil, fmt.Errorf("playlist repository not configured")
	}

	playlist, err := s.playlistRepo.GetPlaylistByID(playlistID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPlaylistNotFound, err)
	}

	if playlist.UserID != userID && !playlist.IsPublic {
		return nil, ErrPlaylistNotFound
	}

	if playlist.IsSmart {
		rs, err := smartrules.ParseStored(string(playlist.SmartRules))
		if err != nil {
			return nil, fmt.Errorf("stored smart rules are invalid: %w", err)
		}
		// user-scoped fields (tags, favorite) are evaluated for the VIEWER
		items, total, err := s.playlistRepo.EvaluateSmart(ctx, playlistID, rs, int64(userID), limit, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate smart playlist: %w", err)
		}
		playlist.Items = items
		playlist.ItemsTotal = &total
		return playlist, nil
	}

	// Load items
	items, err := s.playlistRepo.GetPlaylistItems(playlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to load playlist items: %w", err)
	}
	playlist.Items = items
	n := len(items)
	playlist.ItemsTotal = &n

	return playlist, nil
}

// GetUserPlaylists returns all playlists owned by the user.
func (s *PlaylistService) GetUserPlaylists(userID int, limit, offset int) ([]models.Playlist, error) {
	if s.playlistRepo == nil {
		return nil, fmt.Errorf("playlist repository not configured")
	}

	return s.playlistRepo.GetUserPlaylists(userID, limit, offset)
}

// UpdatePlaylist updates an existing playlist.
func (s *PlaylistService) UpdatePlaylist(userID int, playlistID int, req *models.UpdatePlaylistRequest) (*models.Playlist, error) {
	return s.UpdatePlaylistCtx(context.Background(), userID, playlistID, req)
}

// UpdatePlaylistCtx is UpdatePlaylist with the request context.
func (s *PlaylistService) UpdatePlaylistCtx(ctx context.Context, userID int, playlistID int, req *models.UpdatePlaylistRequest) (*models.Playlist, error) {
	if s.playlistRepo == nil {
		return nil, fmt.Errorf("playlist repository not configured")
	}

	playlist, err := s.ownedPlaylist(userID, playlistID, "update")
	if err != nil {
		return nil, err
	}

	if playlist.IsSmart {
		if req.IsSmart != nil && !*req.IsSmart {
			return nil, &smartrules.ValidationError{Msg: "a smart playlist cannot be converted to a manual one; create a new playlist"}
		}
		if req.SmartRules != nil {
			rs, err := smartrules.Parse(req.SmartRules)
			if err != nil {
				return nil, err
			}
			if _, _, err := s.playlistRepo.EvaluateSmart(ctx, playlistID, rs, int64(userID), 1, 0); err != nil {
				return nil, fmt.Errorf("smart playlist rules could not be evaluated: %w", err)
			}
			playlist.SmartRules = json.RawMessage(rs.Raw)
		}
	} else if (req.IsSmart != nil && *req.IsSmart) || len(req.SmartRules) > 0 {
		return nil, &smartrules.ValidationError{Msg: "a manual playlist cannot become smart; create a new smart playlist"}
	}

	if req.Name != nil {
		playlist.Name = *req.Name
	}
	if req.Description != nil {
		playlist.Description = *req.Description
	}
	if req.IsPublic != nil {
		playlist.IsPublic = *req.IsPublic
	}
	playlist.UpdatedAt = time.Now()

	if err := s.playlistRepo.UpdatePlaylist(playlist); err != nil {
		return nil, fmt.Errorf("failed to update playlist: %w", err)
	}

	return playlist, nil
}

// DeletePlaylist removes a playlist owned by the user.
func (s *PlaylistService) DeletePlaylist(userID int, playlistID int) error {
	if s.playlistRepo == nil {
		return fmt.Errorf("playlist repository not configured")
	}

	if _, err := s.ownedPlaylist(userID, playlistID, "delete"); err != nil {
		return err
	}

	return s.playlistRepo.DeletePlaylist(playlistID)
}

// AddItem adds an entity to a playlist.
func (s *PlaylistService) AddItem(userID int, playlistID int, req *models.AddPlaylistItemRequest) (*models.PlaylistItem, error) {
	if s.playlistRepo == nil {
		return nil, fmt.Errorf("playlist repository not configured")
	}

	playlist, err := s.ownedPlaylist(userID, playlistID, "modify")
	if err != nil {
		return nil, err
	}
	if playlist.IsSmart {
		return nil, ErrSmartPlaylistItems
	}

	item := &models.PlaylistItem{
		PlaylistID: playlistID,
		EntityID:   req.EntityID,
		EntityType: req.EntityType,
		AddedAt:    time.Now(),
	}

	id, err := s.playlistRepo.AddPlaylistItem(item)
	if err != nil {
		return nil, fmt.Errorf("failed to add item to playlist: %w", err)
	}

	item.ID = id

	// Update playlist timestamp
	_ = s.playlistRepo.TouchPlaylistUpdatedAt(playlistID)

	return item, nil
}

// RemoveItem removes an item from a playlist.
func (s *PlaylistService) RemoveItem(userID int, playlistID int, itemID int) error {
	if s.playlistRepo == nil {
		return fmt.Errorf("playlist repository not configured")
	}

	playlist, err := s.ownedPlaylist(userID, playlistID, "modify")
	if err != nil {
		return err
	}
	if playlist.IsSmart {
		return ErrSmartPlaylistItems
	}

	item, err := s.playlistRepo.GetPlaylistItemByID(itemID)
	if err != nil {
		return fmt.Errorf("playlist item not found: %w", err)
	}

	if item.PlaylistID != playlistID {
		return fmt.Errorf("item does not belong to this playlist")
	}

	if err := s.playlistRepo.DeletePlaylistItem(itemID); err != nil {
		return fmt.Errorf("failed to remove item from playlist: %w", err)
	}

	// Update playlist timestamp
	_ = s.playlistRepo.TouchPlaylistUpdatedAt(playlistID)

	return nil
}

// ownedPlaylist loads a playlist the caller wants to CHANGE. A playlist the caller cannot read at all
// (someone else's private one) is reported as not found, exactly like GET; a readable (public) playlist
// of another user is forbidden.
func (s *PlaylistService) ownedPlaylist(userID, playlistID int, verb string) (*models.Playlist, error) {
	playlist, err := s.playlistRepo.GetPlaylistByID(playlistID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPlaylistNotFound, err)
	}
	if playlist.UserID == userID {
		return playlist, nil
	}
	if !playlist.IsPublic {
		return nil, ErrPlaylistNotFound
	}
	return nil, fmt.Errorf("%w (%s)", ErrPlaylistForbidden, verb)
}
