package models

import (
	"encoding/json"
	"time"
)

// MediaType represents different types of media content
type MediaType struct {
	ID                int64     `json:"id" db:"id"`
	Name              string    `json:"name" db:"name"`
	Description       string    `json:"description" db:"description"`
	DetectionPatterns []string  `json:"detection_patterns" db:"detection_patterns"`
	MetadataProviders []string  `json:"metadata_providers" db:"metadata_providers"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

// MediaItem represents a detected media item with aggregated metadata
type MediaItem struct {
	ID            int64      `json:"id" db:"id"`
	MediaTypeID   int64      `json:"media_type_id" db:"media_type_id"`
	MediaType     *MediaType `json:"media_type,omitempty"`
	Title         string     `json:"title" db:"title"`
	OriginalTitle *string    `json:"original_title,omitempty" db:"original_title"`
	Year          *int       `json:"year,omitempty" db:"year"`
	Description   *string    `json:"description,omitempty" db:"description"`
	Genre         []string   `json:"genre,omitempty" db:"genre"`
	Director      *string    `json:"director,omitempty" db:"director"`
	CastCrew      *CastCrew  `json:"cast_crew,omitempty" db:"cast_crew"`
	Rating        *float64   `json:"rating,omitempty" db:"rating"`
	Runtime       *int       `json:"runtime,omitempty" db:"runtime"`
	Language      *string    `json:"language,omitempty" db:"language"`
	Country       *string    `json:"country,omitempty" db:"country"`
	Status        string     `json:"status" db:"status"`
	ParentID      *int64     `json:"parent_id,omitempty" db:"parent_id"`
	SeasonNumber  *int       `json:"season_number,omitempty" db:"season_number"`
	EpisodeNumber *int       `json:"episode_number,omitempty" db:"episode_number"`
	TrackNumber   *int       `json:"track_number,omitempty" db:"track_number"`
	FirstDetected time.Time  `json:"first_detected" db:"first_detected"`
	LastUpdated   time.Time  `json:"last_updated" db:"last_updated"`

	// Aggregated data (not stored in media_items table, populated by joins)
	ExternalMetadata []ExternalMetadata `json:"external_metadata,omitempty"`
	Files            []MediaFile        `json:"files,omitempty"`
	Collections      []MediaCollection  `json:"collections,omitempty"`
	UserMetadata     *UserMetadata      `json:"user_metadata,omitempty"`
}

// CastCrew represents cast and crew information
type CastCrew struct {
	Director   *string  `json:"director,omitempty"`
	Writers    []string `json:"writers,omitempty"`
	Actors     []Actor  `json:"actors,omitempty"`
	Producers  []string `json:"producers,omitempty"`
	Musicians  []string `json:"musicians,omitempty"`
	Developers []string `json:"developers,omitempty"`
}

// Actor represents an actor with their character
type Actor struct {
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
	Order     int    `json:"order,omitempty"`
}

// ExternalMetadata represents metadata from external sources
type ExternalMetadata struct {
	ID          int64     `json:"id" db:"id"`
	MediaItemID int64     `json:"media_item_id" db:"media_item_id"`
	Provider    string    `json:"provider" db:"provider"`
	ExternalID  string    `json:"external_id" db:"external_id"`
	Data        string    `json:"data" db:"data"`
	Rating      *float64  `json:"rating,omitempty" db:"rating"`
	ReviewURL   *string   `json:"review_url,omitempty" db:"review_url"`
	CoverURL    *string   `json:"cover_url,omitempty" db:"cover_url"`
	TrailerURL  *string   `json:"trailer_url,omitempty" db:"trailer_url"`
	LastFetched time.Time `json:"last_fetched" db:"last_fetched"`
}

// DirectoryAnalysis represents analysis of a directory's content
type DirectoryAnalysis struct {
	ID              int64         `json:"id" db:"id"`
	DirectoryPath   string        `json:"directory_path" db:"directory_path"`
	SmbRoot         string        `json:"smb_root" db:"smb_root"`
	MediaItemID     *int64        `json:"media_item_id,omitempty" db:"media_item_id"`
	MediaItem       *MediaItem    `json:"media_item,omitempty"`
	ConfidenceScore float64       `json:"confidence_score" db:"confidence_score"`
	DetectionMethod string        `json:"detection_method" db:"detection_method"`
	AnalysisData    *AnalysisData `json:"analysis_data,omitempty" db:"analysis_data"`
	LastAnalyzed    time.Time     `json:"last_analyzed" db:"last_analyzed"`
	FilesCount      int           `json:"files_count" db:"files_count"`
	TotalSize       int64         `json:"total_size" db:"total_size"`
}

// AnalysisData contains detailed analysis information
type AnalysisData struct {
	MatchedPatterns   []string         `json:"matched_patterns"`
	FileTypes         map[string]int   `json:"file_types"`
	SizeDistribution  map[string]int64 `json:"size_distribution"`
	DetectedLanguages []string         `json:"detected_languages"`
	QualityIndicators []string         `json:"quality_indicators"`
	StructureScore    float64          `json:"structure_score"`
	FilenameScore     float64          `json:"filename_score"`
	MetadataScore     float64          `json:"metadata_score"`
	AlternativeTitles []string         `json:"alternative_titles"`
}

// MediaFile represents individual file versions linked to a media entity.
// FileID links to the existing files table; the other fields are denormalized for display.
type MediaFile struct {
	ID             int64           `json:"id" db:"id"`
	MediaItemID    int64           `json:"media_item_id" db:"media_item_id"`
	FileID         *int64          `json:"file_id,omitempty" db:"file_id"`
	IsPrimary      bool            `json:"is_primary" db:"is_primary"`
	FilePath       string          `json:"file_path" db:"file_path"`
	SmbRoot        string          `json:"smb_root" db:"smb_root"`
	Filename       string          `json:"filename" db:"filename"`
	FileSize       int64           `json:"file_size" db:"file_size"`
	FileExtension  *string         `json:"file_extension,omitempty" db:"file_extension"`
	QualityInfo    *QualityInfo    `json:"quality_info,omitempty" db:"quality_info"`
	Language       *string         `json:"language,omitempty" db:"language"`
	SubtitleTracks []SubtitleTrack `json:"subtitle_tracks,omitempty" db:"subtitle_tracks"`
	AudioTracks    []AudioTrack    `json:"audio_tracks,omitempty" db:"audio_tracks"`
	Duration       *int            `json:"duration,omitempty" db:"duration"`
	Checksum       *string         `json:"checksum,omitempty" db:"checksum"`
	VirtualSmbLink *string         `json:"virtual_smb_link,omitempty" db:"virtual_smb_link"`
	DirectSmbLink  string          `json:"direct_smb_link" db:"direct_smb_link"`
	LastVerified   time.Time       `json:"last_verified" db:"last_verified"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}

// QualityInfo represents file quality information
type QualityInfo struct {
	Resolution     *Resolution `json:"resolution,omitempty"`
	Bitrate        *int        `json:"bitrate,omitempty"`
	VideoCodec     *string     `json:"video_codec,omitempty"`
	AudioCodec     *string     `json:"audio_codec,omitempty"`
	FrameRate      *float64    `json:"frame_rate,omitempty"`
	AspectRatio    *string     `json:"aspect_ratio,omitempty"`
	ColorDepth     *int        `json:"color_depth,omitempty"`
	HDR            bool        `json:"hdr,omitempty"`
	QualityProfile *string     `json:"quality_profile,omitempty"`
	Source         *string     `json:"source,omitempty"` // BluRay, DVD, WEB-DL, etc.
	QualityScore   int         `json:"quality_score"`
}

// Resolution represents video resolution
type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// SubtitleTrack represents subtitle information
type SubtitleTrack struct {
	Language string `json:"language"`
	Format   string `json:"format"`
	Forced   bool   `json:"forced,omitempty"`
	Default  bool   `json:"default,omitempty"`
}

// AudioTrack represents audio track information
type AudioTrack struct {
	Language   string `json:"language"`
	Codec      string `json:"codec"`
	Channels   string `json:"channels"`
	Bitrate    *int   `json:"bitrate,omitempty"`
	SampleRate *int   `json:"sample_rate,omitempty"`
	Default    bool   `json:"default,omitempty"`
	Commentary bool   `json:"commentary,omitempty"`
}

// QualityProfile represents quality comparison profiles
type QualityProfile struct {
	ID               int64     `json:"id" db:"id"`
	Name             string    `json:"name" db:"name"`
	ResolutionWidth  *int      `json:"resolution_width,omitempty" db:"resolution_width"`
	ResolutionHeight *int      `json:"resolution_height,omitempty" db:"resolution_height"`
	MinBitrate       *int      `json:"min_bitrate,omitempty" db:"min_bitrate"`
	MaxBitrate       *int      `json:"max_bitrate,omitempty" db:"max_bitrate"`
	PreferredCodecs  []string  `json:"preferred_codecs,omitempty" db:"preferred_codecs"`
	QualityScore     int       `json:"quality_score" db:"quality_score"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// MediaCollection represents collections of related media
type MediaCollection struct {
	ID             int64                 `json:"id" db:"id"`
	Name           string                `json:"name" db:"name"`
	CollectionType string                `json:"collection_type" db:"collection_type"`
	Description    *string               `json:"description,omitempty" db:"description"`
	TotalItems     int                   `json:"total_items" db:"total_items"`
	ExternalIDs    map[string]string     `json:"external_ids,omitempty" db:"external_ids"`
	CoverURL       *string               `json:"cover_url,omitempty" db:"cover_url"`
	CreatedAt      time.Time             `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at" db:"updated_at"`
	Items          []MediaCollectionItem `json:"items,omitempty"`
	// IsSmart collections have no stored members: SmartRules are evaluated at read time.
	IsSmart bool `json:"is_smart" db:"is_smart"`
	// SmartRules is the validated rule document (JSON array) of a smart collection.
	SmartRules json.RawMessage `json:"smart_rules,omitempty" db:"smart_rules"`
	// OwnerUserID is the creating user; nil for legacy shared collections.
	OwnerUserID *int64 `json:"owner_id,omitempty" db:"owner_user_id"`
	// IsPublic makes the collection readable by every authenticated user (never editable).
	IsPublic bool `json:"is_public" db:"is_public"`
}

// MediaCollectionItem represents an item within a collection
type MediaCollectionItem struct {
	ID             int64      `json:"id" db:"id"`
	CollectionID   int64      `json:"collection_id" db:"collection_id"`
	MediaItemID    int64      `json:"media_item_id" db:"media_item_id"`
	MediaItem      *MediaItem `json:"media_item,omitempty"`
	SequenceNumber *int       `json:"sequence_number,omitempty" db:"sequence_number"`
	SeasonNumber   *int       `json:"season_number,omitempty" db:"season_number"`
	ReleaseOrder   *int       `json:"release_order,omitempty" db:"release_order"`
}

// UserMetadata represents user-specific metadata
type UserMetadata struct {
	ID            int64      `json:"id" db:"id"`
	MediaItemID   int64      `json:"media_item_id" db:"media_item_id"`
	UserID        int64      `json:"user_id" db:"user_id"`
	UserRating    *float64   `json:"user_rating,omitempty" db:"user_rating"`
	WatchedStatus *string    `json:"watched_status,omitempty" db:"watched_status"`
	WatchedDate   *time.Time `json:"watched_date,omitempty" db:"watched_date"`
	PersonalNotes *string    `json:"personal_notes,omitempty" db:"personal_notes"`
	Tags          []string   `json:"tags,omitempty" db:"tags"`
	Favorite      bool       `json:"favorite" db:"favorite"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// DetectionRule represents rules for media detection
type DetectionRule struct {
	ID               int64     `json:"id" db:"id"`
	MediaTypeID      int64     `json:"media_type_id" db:"media_type_id"`
	RuleName         string    `json:"rule_name" db:"rule_name"`
	RuleType         string    `json:"rule_type" db:"rule_type"`
	Pattern          string    `json:"pattern" db:"pattern"`
	ConfidenceWeight float64   `json:"confidence_weight" db:"confidence_weight"`
	Enabled          bool      `json:"enabled" db:"enabled"`
	Priority         int       `json:"priority" db:"priority"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// ChangeLog represents tracked changes for real-time updates
type ChangeLog struct {
	ID         int64     `json:"id" db:"id"`
	EntityType string    `json:"entity_type" db:"entity_type"`
	EntityID   string    `json:"entity_id" db:"entity_id"`
	ChangeType string    `json:"change_type" db:"change_type"`
	OldData    *string   `json:"old_data,omitempty" db:"old_data"`
	NewData    *string   `json:"new_data,omitempty" db:"new_data"`
	DetectedAt time.Time `json:"detected_at" db:"detected_at"`
	Processed  bool      `json:"processed" db:"processed"`
}

// MediaSearchRequest represents search parameters for media
type MediaSearchRequest struct {
	Query         string     `json:"query" form:"query"`
	MediaTypes    []string   `json:"media_types" form:"media_types"`
	Year          *int       `json:"year" form:"year"`
	YearRange     *YearRange `json:"year_range"`
	Genre         []string   `json:"genre" form:"genre"`
	Quality       []string   `json:"quality" form:"quality"`
	Language      []string   `json:"language" form:"language"`
	MinRating     *float64   `json:"min_rating" form:"min_rating"`
	HasExternals  *bool      `json:"has_externals" form:"has_externals"`
	SmbRoots      []string   `json:"smb_roots" form:"smb_roots"`
	WatchedStatus *string    `json:"watched_status" form:"watched_status"`
	SortBy        string     `json:"sort_by" form:"sort_by"`
	SortOrder     string     `json:"sort_order" form:"sort_order"`
	Limit         int        `json:"limit" form:"limit"`
	Offset        int        `json:"offset" form:"offset"`
}

// YearRange represents a range of years
type YearRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// MediaOverview represents aggregated media statistics
type MediaOverview struct {
	ID                 int64      `json:"id" db:"id"`
	Title              string     `json:"title" db:"title"`
	Year               *int       `json:"year" db:"year"`
	MediaType          string     `json:"media_type" db:"media_type"`
	FileCount          int        `json:"file_count" db:"file_count"`
	TotalSize          int64      `json:"total_size" db:"total_size"`
	LastVerified       *time.Time `json:"last_verified" db:"last_verified"`
	AvailableQualities []string   `json:"available_qualities" db:"available_qualities"`
}

// Custom JSON marshaling for database storage
func (mt *MediaType) MarshalDetectionPatterns() ([]byte, error) {
	return json.Marshal(mt.DetectionPatterns)
}

func (mt *MediaType) UnmarshalDetectionPatterns(data []byte) error {
	return json.Unmarshal(data, &mt.DetectionPatterns)
}

func (mi *MediaItem) MarshalGenre() ([]byte, error) {
	return json.Marshal(mi.Genre)
}

func (mi *MediaItem) UnmarshalGenre(data []byte) error {
	return json.Unmarshal(data, &mi.Genre)
}

func (mi *MediaItem) MarshalCastCrew() ([]byte, error) {
	return json.Marshal(mi.CastCrew)
}

func (mi *MediaItem) UnmarshalCastCrew(data []byte) error {
	return json.Unmarshal(data, &mi.CastCrew)
}

// Helper methods for quality comparison
func (qi *QualityInfo) IsBetterThan(other *QualityInfo) bool {
	if qi == nil || other == nil {
		return qi != nil
	}
	return qi.QualityScore > other.QualityScore
}

func (qi *QualityInfo) GetDisplayName() string {
	if qi.QualityProfile != nil {
		return *qi.QualityProfile
	}
	if qi.Resolution != nil {
		return qi.Resolution.GetDisplayName()
	}
	return "Unknown"
}

func (r *Resolution) GetDisplayName() string {
	switch {
	case r.Width >= 3840:
		return "4K/UHD"
	case r.Width >= 1920:
		return "1080p"
	case r.Width >= 1280:
		return "720p"
	case r.Width >= 720:
		return "480p/DVD"
	default:
		return "Low Quality"
	}
}
