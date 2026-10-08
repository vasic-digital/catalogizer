package models

import (
	"time"
)

// Media types
const (
	MediaTypeVideo = "video"
	MediaTypeAudio = "audio"
	MediaTypeImage = "image"
	MediaTypeText  = "text"
	MediaTypeBook  = "book"
	MediaTypeGame  = "game"
	MediaTypeOther = "other"
)

// File represents a file record from the catalog database
type File struct {
	ID               int64      `json:"id" db:"id"`
	StorageRootID    int64      `json:"storage_root_id" db:"storage_root_id"`
	StorageRootName  string     `json:"storage_root_name" db:"storage_root_name"`
	Path             string     `json:"path" db:"path"`
	Name             string     `json:"name" db:"name"`
	Extension        *string    `json:"extension" db:"extension"`
	MimeType         *string    `json:"mime_type" db:"mime_type"`
	FileType         *string    `json:"file_type" db:"file_type"`
	Size             int64      `json:"size" db:"size"`
	IsDirectory      bool       `json:"is_directory" db:"is_directory"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	ModifiedAt       time.Time  `json:"modified_at" db:"modified_at"`
	AccessedAt       *time.Time `json:"accessed_at" db:"accessed_at"`
	Deleted          bool       `json:"deleted" db:"deleted"`
	DeletedAt        *time.Time `json:"deleted_at" db:"deleted_at"`
	LastScanAt       time.Time  `json:"last_scan_at" db:"last_scan_at"`
	LastVerifiedAt   *time.Time `json:"last_verified_at" db:"last_verified_at"`
	MD5              *string    `json:"md5" db:"md5"`
	SHA256           *string    `json:"sha256" db:"sha256"`
	SHA1             *string    `json:"sha1" db:"sha1"`
	BLAKE3           *string    `json:"blake3" db:"blake3"`
	QuickHash        *string    `json:"quick_hash" db:"quick_hash"`
	IsDuplicate      bool       `json:"is_duplicate" db:"is_duplicate"`
	DuplicateGroupID *int64     `json:"duplicate_group_id" db:"duplicate_group_id"`
	ParentID         *int64     `json:"parent_id" db:"parent_id"`
}

// StorageRoot represents a storage root configuration for any protocol
type StorageRoot struct {
	ID                       int64      `json:"id" db:"id"`
	Name                     string     `json:"name" db:"name"`
	Protocol                 string     `json:"protocol" db:"protocol"` // smb, ftp, nfs, webdav, local
	Host                     *string    `json:"host,omitempty" db:"host"`
	Port                     *int       `json:"port,omitempty" db:"port"`
	Path                     *string    `json:"path,omitempty" db:"path"` // share for SMB, path for FTP/NFS/WebDAV, base_path for local
	Username                 *string    `json:"username,omitempty" db:"username"`
	Password                 *string    `json:"password,omitempty" db:"password"`
	Domain                   *string    `json:"domain,omitempty" db:"domain"`           // SMB specific
	MountPoint               *string    `json:"mount_point,omitempty" db:"mount_point"` // NFS specific
	Options                  *string    `json:"options,omitempty" db:"options"`         // NFS/WebDAV specific
	URL                      *string    `json:"url,omitempty" db:"url"`                 // WebDAV specific
	Enabled                  bool       `json:"enabled" db:"enabled"`
	MaxDepth                 int        `json:"max_depth" db:"max_depth"`
	AllowEmpty               bool       `json:"allow_empty" db:"allow_empty"` // an empty share is expected: its scan may complete with 0 files (default false, WF22 R3)
	EnableDuplicateDetection bool       `json:"enable_duplicate_detection" db:"enable_duplicate_detection"`
	EnableMetadataExtraction bool       `json:"enable_metadata_extraction" db:"enable_metadata_extraction"`
	IncludePatterns          *string    `json:"include_patterns" db:"include_patterns"`
	ExcludePatterns          *string    `json:"exclude_patterns" db:"exclude_patterns"`
	CreatedAt                time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at" db:"updated_at"`
	LastScanAt               *time.Time `json:"last_scan_at" db:"last_scan_at"`
}

// StorageRootConnColumns lists, in the order of ConnScanTargets, the storage_roots columns the settings contract
// (filesystem.SettingsFromRoot) consumes. EVERY query that builds a StorageRoot to hand to a client factory selects exactly these, so no caller can
// forget the url of a WebDAV root or the mount point of an NFS one (WF22 H1/R1: two callers did).
const StorageRootConnColumns = "protocol, host, port, path, username, password, domain, url, mount_point, options"

// StorageRootConnColumnsFor is StorageRootConnColumns with every column qualified by the table alias (for joins).
func StorageRootConnColumnsFor(alias string) string {
	return alias + ".protocol, " + alias + ".host, " + alias + ".port, " + alias + ".path, " + alias + ".username, " + alias + ".password, " +
		alias + ".domain, " + alias + ".url, " + alias + ".mount_point, " + alias + ".options"
}

// ConnScanTargets returns the Scan destinations for StorageRootConnColumns.
func (r *StorageRoot) ConnScanTargets() []interface{} {
	return []interface{}{&r.Protocol, &r.Host, &r.Port, &r.Path, &r.Username, &r.Password, &r.Domain, &r.URL, &r.MountPoint, &r.Options}
}

// FileMetadata represents file metadata
type FileMetadata struct {
	ID       int64  `json:"id" db:"id"`
	FileID   int64  `json:"file_id" db:"file_id"`
	Key      string `json:"key" db:"key"`
	Value    string `json:"value" db:"value"`
	DataType string `json:"data_type" db:"data_type"`
}

// DuplicateGroup represents a group of duplicate files
type DuplicateGroup struct {
	ID        int64     `json:"id" db:"id"`
	FileCount int       `json:"file_count" db:"file_count"`
	TotalSize int64     `json:"total_size" db:"total_size"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// VirtualPath represents a virtual file system path
type VirtualPath struct {
	ID         int64     `json:"id" db:"id"`
	Path       string    `json:"path" db:"path"`
	TargetType string    `json:"target_type" db:"target_type"`
	TargetID   int64     `json:"target_id" db:"target_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// ScanHistory represents scan operation history
type ScanHistory struct {
	ID             int64      `json:"id" db:"id"`
	StorageRootID  int64      `json:"storage_root_id" db:"storage_root_id"`
	ScanType       string     `json:"scan_type" db:"scan_type"`
	Status         string     `json:"status" db:"status"`
	StartTime      time.Time  `json:"start_time" db:"start_time"`
	EndTime        *time.Time `json:"end_time" db:"end_time"`
	FilesProcessed int        `json:"files_processed" db:"files_processed"`
	FilesAdded     int        `json:"files_added" db:"files_added"`
	FilesUpdated   int        `json:"files_updated" db:"files_updated"`
	FilesDeleted   int        `json:"files_deleted" db:"files_deleted"`
	ErrorCount     int        `json:"error_count" db:"error_count"`
	ErrorMessage   *string    `json:"error_message" db:"error_message"`
}

// FileWithMetadata represents a file with its metadata
type FileWithMetadata struct {
	File
	Metadata []FileMetadata `json:"metadata,omitempty"`
}

// DirectoryInfo represents directory information with statistics
type DirectoryInfo struct {
	Path            string    `json:"path" db:"path"`
	Name            string    `json:"name" db:"name"`
	StorageRootName string    `json:"storage_root_name" db:"storage_root_name"`
	FileCount       int       `json:"file_count" db:"file_count"`
	DirectoryCount  int       `json:"directory_count" db:"directory_count"`
	TotalSize       int64     `json:"total_size" db:"total_size"`
	DuplicateCount  int       `json:"duplicate_count" db:"duplicate_count"`
	ModifiedAt      time.Time `json:"modified_at" db:"modified_at"`
}

// SearchFilter represents search filter criteria
type SearchFilter struct {
	Query              string     `json:"query,omitempty"`
	Path               string     `json:"path,omitempty"`
	Name               string     `json:"name,omitempty"`
	Extension          string     `json:"extension,omitempty"`
	FileType           string     `json:"file_type,omitempty"`
	MimeType           string     `json:"mime_type,omitempty"`
	StorageRoots       []string   `json:"storage_roots,omitempty"`
	MinSize            *int64     `json:"min_size,omitempty"`
	MaxSize            *int64     `json:"max_size,omitempty"`
	ModifiedAfter      *time.Time `json:"modified_after,omitempty"`
	ModifiedBefore     *time.Time `json:"modified_before,omitempty"`
	IncludeDeleted     bool       `json:"include_deleted"`
	OnlyDuplicates     bool       `json:"only_duplicates"`
	ExcludeDuplicates  bool       `json:"exclude_duplicates"`
	IncludeDirectories bool       `json:"include_directories"`
}

// SortOptions represents sorting options
type SortOptions struct {
	Field string `json:"field"` // name, size, modified_at, created_at, path, extension
	Order string `json:"order"` // asc, desc
}

// PaginationOptions represents pagination options
type PaginationOptions struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// SearchResult represents search results with pagination
type SearchResult struct {
	Files      []FileWithMetadata `json:"files"`
	TotalCount int64              `json:"total_count"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalPages int                `json:"total_pages"`
}

// MediaMetadata represents media metadata information
type MediaMetadata struct {
	ID          int64                  `json:"id" db:"id"`
	Title       string                 `json:"title" db:"title"`
	Description string                 `json:"description,omitempty" db:"description"`
	Genre       string                 `json:"genre,omitempty" db:"genre"`
	Year        *int                   `json:"year,omitempty" db:"year"`
	Rating      *float64               `json:"rating,omitempty" db:"rating"`
	Duration    *int                   `json:"duration,omitempty" db:"duration"`
	Language    string                 `json:"language,omitempty" db:"language"`
	Country     string                 `json:"country,omitempty" db:"country"`
	Director    string                 `json:"director,omitempty" db:"director"`
	Producer    string                 `json:"producer,omitempty" db:"producer"`
	Cast        []string               `json:"cast,omitempty" db:"cast"`
	MediaType   string                 `json:"media_type,omitempty" db:"media_type"`
	Resolution  string                 `json:"resolution,omitempty" db:"resolution"`
	FileSize    *int64                 `json:"file_size,omitempty" db:"file_size"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	ExternalIDs map[string]string      `json:"external_ids,omitempty" db:"external_ids"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// MediaItem represents a media item for testing
type MediaItem struct {
	ID     int    `json:"id"`
	UserID int    `json:"user_id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
}

// Stress test status constants
const (
	StressTestStatusPending   = "pending"
	StressTestStatusRunning   = "running"
	StressTestStatusCompleted = "completed"
	StressTestStatusFailed    = "failed"
	StressTestStatusTimeout   = "timeout"
	StressTestStatusCancelled = "cancelled"
)

// WizardSession represents a configuration wizard session
type WizardSession struct {
	SessionID     string                 `json:"session_id" db:"session_id"`
	UserID        int                    `json:"user_id" db:"user_id"`
	CurrentStep   int                    `json:"current_step" db:"current_step"`
	TotalSteps    int                    `json:"total_steps" db:"total_steps"`
	StepData      map[string]interface{} `json:"step_data" db:"step_data"`
	Configuration map[string]interface{} `json:"configuration" db:"configuration"`
	StartedAt     time.Time              `json:"started_at" db:"started_at"`
	LastActivity  time.Time              `json:"last_activity" db:"last_activity"`
	IsCompleted   bool                   `json:"is_completed" db:"is_completed"`
	ConfigType    string                 `json:"config_type" db:"config_type"`
}

// ConfigurationProfile represents a saved configuration profile
type ConfigurationProfile struct {
	ProfileID     string                 `json:"profile_id" db:"profile_id"`
	Name          string                 `json:"name" db:"name"`
	Description   string                 `json:"description" db:"description"`
	UserID        int                    `json:"user_id" db:"user_id"`
	Configuration map[string]interface{} `json:"configuration" db:"configuration"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" db:"updated_at"`
	IsActive      bool                   `json:"is_active" db:"is_active"`
	Tags          []string               `json:"tags" db:"tags"`
}
