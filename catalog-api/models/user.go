package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	IsValid  bool     `json:"is_valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Configuration represents a configuration object
type Configuration struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Settings    map[string]interface{} `json:"settings"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Permission represents a permission object
type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
}

// User represents a system user with role-based permissions
type User struct {
	ID                  int        `json:"id" db:"id"`
	Username            string     `json:"username" db:"username"`
	Email               string     `json:"email" db:"email"`
	PasswordHash        string     `json:"-" db:"password_hash"` // Never include in JSON
	Salt                string     `json:"-" db:"salt"`          // Never include in JSON
	RoleID              int        `json:"role_id" db:"role_id"`
	Role                *Role      `json:"role,omitempty"`
	FirstName           *string    `json:"first_name" db:"first_name"`
	LastName            *string    `json:"last_name" db:"last_name"`
	DisplayName         *string    `json:"display_name" db:"display_name"`
	AvatarURL           *string    `json:"avatar_url" db:"avatar_url"`
	TimeZone            *string    `json:"time_zone" db:"time_zone"`
	Language            *string    `json:"language" db:"language"`
	Settings            string     `json:"settings" db:"settings"`
	IsActive            bool       `json:"is_active" db:"is_active"`
	IsLocked            bool       `json:"is_locked" db:"is_locked"`
	LockedUntil         *time.Time `json:"locked_until,omitempty" db:"locked_until"`
	FailedLoginAttempts int        `json:"failed_login_attempts" db:"failed_login_attempts"`
	LastLoginAt         *time.Time `json:"last_login_at" db:"last_login_at"`
	LastLoginIP         *string    `json:"last_login_ip,omitempty" db:"last_login_ip"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
}

// Role represents a user role with specific permissions
type Role struct {
	ID          int         `json:"id" db:"id"`
	Name        string      `json:"name" db:"name"`
	Description *string     `json:"description" db:"description"`
	Permissions Permissions `json:"permissions" db:"permissions"`
	IsSystem    bool        `json:"is_system" db:"is_system"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

// Permissions represents a list of permission strings
type Permissions []string

// Value implements the driver.Valuer interface for database storage
func (p Permissions) Value() (driver.Value, error) {
	if len(p) == 0 {
		return "[]", nil
	}
	bytes, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

// Scan implements the sql.Scanner interface for database retrieval
func (p *Permissions) Scan(value interface{}) error {
	if value == nil {
		*p = Permissions{}
		return nil
	}

	switch v := value.(type) {
	case string:
		return json.Unmarshal([]byte(v), p)
	case []byte:
		return json.Unmarshal(v, p)
	}

	return nil
}

// HasPermission checks if the permissions include a specific permission
func (p Permissions) HasPermission(permission string) bool {
	// Check for wildcard permission
	for _, perm := range p {
		if perm == "*" {
			return true
		}
		if perm == permission {
			return true
		}
		// Check for wildcard patterns (e.g., "media.*" matches "media.view")
		if len(perm) > 0 && perm[len(perm)-1] == '*' {
			prefix := perm[:len(perm)-1]
			// A prefix wildcard needs a segment boundary ("system.*", "admin:*"):
			// "sys*" must not span "system.admin".
			if prefix != "" && prefix[len(prefix)-1] != '.' && prefix[len(prefix)-1] != ':' {
				continue
			}
			if len(permission) >= len(prefix) && permission[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

// HasAnyPermission checks if the permissions include any of the specified permissions
func (p Permissions) HasAnyPermission(permissions []string) bool {
	for _, permission := range permissions {
		if p.HasPermission(permission) {
			return true
		}
	}
	return false
}

// UserPreferences represents user-specific preferences
type UserPreferences struct {
	Theme                string            `json:"theme,omitempty"`
	Language             string            `json:"language,omitempty"`
	Timezone             string            `json:"timezone,omitempty"`
	MediaPlayerSettings  MediaPlayerPrefs  `json:"media_player,omitempty"`
	NotificationSettings NotificationPrefs `json:"notifications,omitempty"`
	PrivacySettings      PrivacyPrefs      `json:"privacy,omitempty"`
	UISettings           UIPrefs           `json:"ui,omitempty"`
}

// MediaPlayerPrefs represents media player preferences
type MediaPlayerPrefs struct {
	AutoPlay       bool    `json:"auto_play"`
	DefaultQuality string  `json:"default_quality"`
	Volume         float64 `json:"volume"`
	Subtitles      bool    `json:"subtitles"`
	SkipIntro      bool    `json:"skip_intro"`
}

// NotificationPrefs represents notification preferences
type NotificationPrefs struct {
	EmailNotifications bool `json:"email_notifications"`
	PushNotifications  bool `json:"push_notifications"`
	SyncNotifications  bool `json:"sync_notifications"`
	ErrorNotifications bool `json:"error_notifications"`
}

// PrivacyPrefs represents privacy preferences
type PrivacyPrefs struct {
	ShareUsageData      bool `json:"share_usage_data"`
	LocationTracking    bool `json:"location_tracking"`
	AnalyticsTracking   bool `json:"analytics_tracking"`
	PersonalizedContent bool `json:"personalized_content"`
}

// UIPrefs represents UI preferences
type UIPrefs struct {
	DensityMode     string `json:"density_mode"` // compact, comfortable, spacious
	GridSize        int    `json:"grid_size"`    // items per row
	ShowThumbnails  bool   `json:"show_thumbnails"`
	ShowMetadata    bool   `json:"show_metadata"`
	DefaultSortBy   string `json:"default_sort_by"`
	DefaultSortDesc bool   `json:"default_sort_desc"`
}

// UserSettings represents user-specific settings
type UserSettings struct {
	DefaultShare        string             `json:"default_share,omitempty"`
	AutoSync            bool               `json:"auto_sync"`
	SyncIntervalMinutes int                `json:"sync_interval_minutes"`
	DownloadQuality     string             `json:"download_quality"`
	CacheSettings       CacheSettings      `json:"cache,omitempty"`
	ConversionSettings  ConversionSettings `json:"conversion,omitempty"`
	BackupSettings      BackupSettings     `json:"backup,omitempty"`
	SecuritySettings    SecuritySettings   `json:"security,omitempty"`
}

// CacheSettings represents caching preferences
type CacheSettings struct {
	MaxCacheSize    int64 `json:"max_cache_size"` // bytes
	CacheThumbnails bool  `json:"cache_thumbnails"`
	CacheMetadata   bool  `json:"cache_metadata"`
	CacheTimeout    int   `json:"cache_timeout"` // minutes
}

// ConversionSettings represents format conversion preferences
type ConversionSettings struct {
	DefaultVideoFormat string `json:"default_video_format"`
	DefaultAudioFormat string `json:"default_audio_format"`
	DefaultQuality     string `json:"default_quality"`
	MaxConcurrentJobs  int    `json:"max_concurrent_jobs"`
}

// BackupSettings represents backup preferences
type BackupSettings struct {
	AutoBackup       bool   `json:"auto_backup"`
	BackupInterval   string `json:"backup_interval"`  // daily, weekly, monthly
	BackupRetention  int    `json:"backup_retention"` // days
	BackupLocation   string `json:"backup_location"`
	BackupEncryption bool   `json:"backup_encryption"`
}

// SecuritySettings represents security preferences
type SecuritySettings struct {
	TwoFactorEnabled   bool     `json:"two_factor_enabled"`
	SessionTimeout     int      `json:"session_timeout"`      // minutes
	RequirePasswordFor []string `json:"require_password_for"` // actions requiring password
	LoginNotifications bool     `json:"login_notifications"`
}

// Value implements the driver.Valuer interface for UserPreferences
func (up UserPreferences) Value() (driver.Value, error) {
	return json.Marshal(up)
}

// Scan implements the sql.Scanner interface for UserPreferences
func (up *UserPreferences) Scan(value interface{}) error {
	if value == nil {
		*up = UserPreferences{}
		return nil
	}

	switch v := value.(type) {
	case string:
		return json.Unmarshal([]byte(v), up)
	case []byte:
		return json.Unmarshal(v, up)
	}

	return nil
}

// Value implements the driver.Valuer interface for UserSettings
func (us UserSettings) Value() (driver.Value, error) {
	return json.Marshal(us)
}

// Scan implements the sql.Scanner interface for UserSettings
func (us *UserSettings) Scan(value interface{}) error {
	if value == nil {
		*us = UserSettings{}
		return nil
	}

	switch v := value.(type) {
	case string:
		return json.Unmarshal([]byte(v), us)
	case []byte:
		return json.Unmarshal(v, us)
	}

	return nil
}

// UserSession represents an active user session
type UserSession struct {
	ID             int        `json:"id" db:"id"`
	UserID         int        `json:"user_id" db:"user_id"`
	SessionToken   string     `json:"session_token" db:"session_token"`
	RefreshToken   *string    `json:"refresh_token,omitempty" db:"refresh_token"`
	DeviceInfo     DeviceInfo `json:"device_info" db:"device_info"`
	IPAddress      *string    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent      *string    `json:"user_agent,omitempty" db:"user_agent"`
	IsActive       bool       `json:"is_active" db:"is_active"`
	ExpiresAt      time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	LastActivityAt time.Time  `json:"last_activity_at" db:"last_activity_at"`
}

// DeviceInfo represents information about the user's device
type DeviceInfo struct {
	DeviceType      *string `json:"device_type,omitempty"` // mobile, tablet, desktop, tv
	Platform        *string `json:"platform,omitempty"`    // android, ios, windows, macos, linux
	PlatformVersion *string `json:"platform_version,omitempty"`
	AppVersion      *string `json:"app_version,omitempty"`
	DeviceModel     *string `json:"device_model,omitempty"`
	DeviceName      *string `json:"device_name,omitempty"`
	ScreenSize      *string `json:"screen_size,omitempty"`
	IsEmulator      *bool   `json:"is_emulator,omitempty"`
}

// Value implements the driver.Valuer interface for DeviceInfo
func (di DeviceInfo) Value() (driver.Value, error) {
	return json.Marshal(di)
}

// Scan implements the sql.Scanner interface for DeviceInfo
func (di *DeviceInfo) Scan(value interface{}) error {
	if value == nil {
		*di = DeviceInfo{}
		return nil
	}

	switch v := value.(type) {
	case string:
		return json.Unmarshal([]byte(v), di)
	case []byte:
		return json.Unmarshal(v, di)
	}

	return nil
}

// UserSummary represents a summary view of user information
type UserSummary struct {
	ID                 int        `json:"id" db:"id"`
	Username           string     `json:"username" db:"username"`
	Email              string     `json:"email" db:"email"`
	DisplayName        *string    `json:"display_name" db:"display_name"`
	RoleName           string     `json:"role_name" db:"role_name"`
	RoleDisplayName    string     `json:"role_display_name" db:"role_display_name"`
	IsActive           bool       `json:"is_active" db:"is_active"`
	LastLoginAt        *time.Time `json:"last_login_at" db:"last_login_at"`
	TotalMediaAccesses int        `json:"total_media_accesses" db:"total_media_accesses"`
	TotalFavorites     int        `json:"total_favorites" db:"total_favorites"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Username                string           `json:"username" validate:"required,min=3,max=50"`
	Email                   string           `json:"email" validate:"required,email"`
	Password                string           `json:"password" validate:"required,min=8"`
	RoleID                  int              `json:"role_id" validate:"required"`
	FirstName               *string          `json:"first_name"`
	LastName                *string          `json:"last_name"`
	TimeZone                *string          `json:"time_zone"`
	Language                *string          `json:"language"`
	IsActive                *bool            `json:"is_active"`
	DisplayName             *string          `json:"display_name"`
	LocationTrackingEnabled *bool            `json:"location_tracking_enabled"`
	AnalyticsEnabled        *bool            `json:"analytics_enabled"`
	Preferences             *UserPreferences `json:"preferences"`
	Settings                *UserSettings    `json:"settings"`
}

// UpdateUserRequest represents a request to update user information
type UpdateUserRequest struct {
	Username                *string          `json:"username" validate:"omitempty,min=3,max=50"`
	Email                   *string          `json:"email" validate:"omitempty,email"`
	RoleID                  *int             `json:"role_id"`
	FirstName               *string          `json:"first_name"`
	LastName                *string          `json:"last_name"`
	TimeZone                *string          `json:"time_zone"`
	Language                *string          `json:"language"`
	DisplayName             *string          `json:"display_name"`
	AvatarURL               *string          `json:"avatar_url"`
	LocationTrackingEnabled *bool            `json:"location_tracking_enabled"`
	AnalyticsEnabled        *bool            `json:"analytics_enabled"`
	IsActive                *bool            `json:"is_active"`
	Preferences             *UserPreferences `json:"preferences"`
	Settings                *UserSettings    `json:"settings"`
}

// CreateRoleRequest represents a request to create a new role
type CreateRoleRequest struct {
	Name        string   `json:"name" validate:"required,min=2,max=50"`
	Description *string  `json:"description"`
	Permissions []string `json:"permissions" validate:"required"`
}

// UpdateRoleRequest represents a request to update role information
type UpdateRoleRequest struct {
	Name        *string  `json:"name" validate:"omitempty,min=2,max=50"`
	Description *string  `json:"description"`
	Permissions []string `json:"permissions"`
}

// ChangePasswordRequest represents a request to change user password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username   string     `json:"username" validate:"required"`
	Password   string     `json:"password" validate:"required"`
	DeviceInfo DeviceInfo `json:"device_info,omitempty"`
	RememberMe bool       `json:"remember_me"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	User         User      `json:"user"`
	SessionToken string    `json:"session_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// UserListResponse represents a paginated list of users
type UserListResponse struct {
	Users      []UserSummary `json:"users"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}

// Common permission constants
const (
	// System permissions
	PermissionSystemAdmin  = "system.admin"
	PermissionSystemConfig = "system.configure"

	// User management permissions
	PermissionUserView   = "user.view"
	PermissionUserCreate = "user.create"
	PermissionUserUpdate = "user.update"
	PermissionUserDelete = "user.delete"
	PermissionUserManage = "user.manage" // includes all user operations

	// Media permissions
	PermissionMediaView     = "media.view"
	PermissionMediaUpload   = "media.upload"
	PermissionMediaEdit     = "media.edit"
	PermissionMediaDelete   = "media.delete"
	PermissionMediaShare    = "media.share"
	PermissionMediaDownload = "media.download"
	PermissionMediaConvert  = "media.convert"
	PermissionMediaManage   = "media.manage" // includes all media operations

	// Share permissions
	PermissionShareView    = "share.view"
	PermissionShareCreate  = "share.create"
	PermissionShareEdit    = "share.edit"
	PermissionShareDelete  = "share.delete"
	PermissionShareManage  = "share.manage" // includes all share operations
	PermissionViewShares   = "share.view"
	PermissionEditShares   = "share.edit"
	PermissionDeleteShares = "share.delete"

	// Analytics permissions
	PermissionAnalyticsView   = "analytics.view"
	PermissionAnalyticsExport = "analytics.export"
	PermissionAnalyticsManage = "analytics.manage"
	PermissionViewAnalytics   = "analytics.view"

	// Favorites permissions
	PermissionFavoriteView   = "favorite.view"
	PermissionFavoriteCreate = "favorite.create"
	PermissionFavoriteDelete = "favorite.delete"

	// Profile permissions
	PermissionProfileView   = "profile.view"
	PermissionProfileUpdate = "profile.update"

	// Sync permissions
	PermissionSyncView    = "sync.view"
	PermissionSyncCreate  = "sync.create"
	PermissionSyncExecute = "sync.execute"
	PermissionSyncManage  = "sync.manage"

	// Conversion permissions
	PermissionConversionView   = "conversion.view"
	PermissionConversionCreate = "conversion.create"
	PermissionConversionManage = "conversion.manage"

	// Report permissions
	PermissionReportView   = "report.view"
	PermissionReportCreate = "report.create"
	PermissionReportExport = "report.export"

	// Wildcard permission
	PermissionWildcard = "*"
)

// Utility functions

// IsAccountLocked checks if the user account is currently locked
func (u *User) IsAccountLocked() bool {
	if !u.IsLocked {
		return false
	}
	// If locked but LockedUntil is nil, it's a permanent lock
	if u.LockedUntil == nil {
		return true
	}
	// If locked with expiry, check if lock has expired
	return u.LockedUntil.After(time.Now())
}

// CanLogin checks if the user can login (active and not locked)
func (u *User) CanLogin() bool {
	return u.IsActive && !u.IsAccountLocked()
}

// HasPermission checks if the user has a specific permission
func (u *User) HasPermission(permission string) bool {
	if u.Role == nil {
		return false
	}
	return u.Role.Permissions.HasPermission(permission)
}

// HasAnyPermission checks if the user has any of the specified permissions
func (u *User) HasAnyPermission(permissions []string) bool {
	if u.Role == nil {
		return false
	}
	return u.Role.Permissions.HasAnyPermission(permissions)
}

// IsAdmin checks if the user is an administrator
func (u *User) IsAdmin() bool {
	return u.HasPermission(PermissionSystemAdmin) || u.HasPermission(PermissionWildcard)
}

// GetDefaultPreferences returns default user preferences
func GetDefaultPreferences() UserPreferences {
	return UserPreferences{
		Theme:    "auto",
		Language: "en",
		Timezone: "UTC",
		MediaPlayerSettings: MediaPlayerPrefs{
			AutoPlay:       false,
			DefaultQuality: "auto",
			Volume:         0.8,
			Subtitles:      false,
			SkipIntro:      false,
		},
		NotificationSettings: NotificationPrefs{
			EmailNotifications: true,
			PushNotifications:  true,
			SyncNotifications:  true,
			ErrorNotifications: true,
		},
		PrivacySettings: PrivacyPrefs{
			ShareUsageData:      true,
			LocationTracking:    true,
			AnalyticsTracking:   true,
			PersonalizedContent: true,
		},
		UISettings: UIPrefs{
			DensityMode:     "comfortable",
			GridSize:        4,
			ShowThumbnails:  true,
			ShowMetadata:    true,
			DefaultSortBy:   "name",
			DefaultSortDesc: false,
		},
	}
}

// GetDefaultSettings returns default user settings
func GetDefaultSettings() UserSettings {
	return UserSettings{
		AutoSync:            true,
		SyncIntervalMinutes: 60,
		DownloadQuality:     "original",
		CacheSettings: CacheSettings{
			MaxCacheSize:    1024 * 1024 * 1024, // 1GB
			CacheThumbnails: true,
			CacheMetadata:   true,
			CacheTimeout:    60, // 1 hour
		},
		ConversionSettings: ConversionSettings{
			DefaultVideoFormat: "mp4",
			DefaultAudioFormat: "mp3",
			DefaultQuality:     "high",
			MaxConcurrentJobs:  3,
		},
		BackupSettings: BackupSettings{
			AutoBackup:       false,
			BackupInterval:   "weekly",
			BackupRetention:  30,
			BackupEncryption: true,
		},
		SecuritySettings: SecuritySettings{
			TwoFactorEnabled:   false,
			SessionTimeout:     1440, // 24 hours in minutes
			RequirePasswordFor: []string{"delete", "share"},
			LoginNotifications: true,
		},
	}
}

// Analytics and Reporting Models

// MediaAccessLog represents a log entry for media access
type MediaAccessLog struct {
	ID               int            `json:"id" db:"id"`
	UserID           int            `json:"user_id" db:"user_id"`
	MediaID          int            `json:"media_id" db:"media_id"`
	Action           string         `json:"action" db:"action"`
	DeviceInfo       *DeviceInfo    `json:"device_info,omitempty" db:"device_info"`
	Location         *Location      `json:"location,omitempty" db:"location"`
	IPAddress        *string        `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent        *string        `json:"user_agent,omitempty" db:"user_agent"`
	PlaybackDuration *time.Duration `json:"playback_duration,omitempty" db:"playback_duration"`
	AccessTime       time.Time      `json:"access_time" db:"access_time"`
}

// AnalyticsEvent represents a general analytics event
type AnalyticsEvent struct {
	ID            int         `json:"id" db:"id"`
	UserID        int         `json:"user_id" db:"user_id"`
	EventType     string      `json:"event_type" db:"event_type"`
	EventCategory string      `json:"event_category" db:"event_category"`
	Data          string      `json:"data" db:"data"`
	DeviceInfo    *DeviceInfo `json:"device_info,omitempty" db:"device_info"`
	Location      *Location   `json:"location,omitempty" db:"location"`
	IPAddress     *string     `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent     *string     `json:"user_agent,omitempty" db:"user_agent"`
	Timestamp     time.Time   `json:"timestamp" db:"timestamp"`
}

// Location represents geographic coordinates
type Location struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Country   *string  `json:"country,omitempty"`
	City      *string  `json:"city,omitempty"`
	Accuracy  *float64 `json:"accuracy,omitempty"`
}

// AnalyticsEventRequest represents a request to track an analytics event
type AnalyticsEventRequest struct {
	EventType  string                 `json:"event_type"`
	EntityType string                 `json:"entity_type,omitempty"`
	EntityID   int                    `json:"entity_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
}

// AnalyticsFilters represents filters for analytics queries
type AnalyticsFilters struct {
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	EventTypes  []string   `json:"event_types,omitempty"`
	EntityTypes []string   `json:"entity_types,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// AnalyticsData represents aggregated analytics data
type AnalyticsData struct {
	TotalEvents    int64                    `json:"total_events"`
	EventBreakdown map[string]int           `json:"event_breakdown"`
	Trends         map[string][]interface{} `json:"trends"`
	TopEntities    []map[string]interface{} `json:"top_entities"`
}

// DashboardMetrics represents dashboard metrics
type DashboardMetrics struct {
	TotalUsers       int64 `json:"total_users"`
	ActiveUsers      int64 `json:"active_users"`
	TotalMediaItems  int64 `json:"total_media_items"`
	TotalStorageUsed int64 `json:"total_storage_used"`
	RecentActivity   int64 `json:"recent_activity"`
}

// RealtimeMetrics represents realtime metrics
type RealtimeMetrics struct {
	ActiveUsers    int     `json:"active_users"`
	CurrentStreams int     `json:"current_streams"`
	RecentEvents   int     `json:"recent_events"`
	SystemLoad     float64 `json:"system_load"`
}

// ReportRequest represents a request to generate a report
type ReportRequest struct {
	ReportType string                 `json:"report_type"`
	Params     map[string]interface{} `json:"params,omitempty"`
}

// UserAnalytics represents analytics data for a user
type UserAnalytics struct {
	UserID               int                `json:"user_id"`
	StartDate            time.Time          `json:"start_date"`
	EndDate              time.Time          `json:"end_date"`
	TotalMediaAccesses   int                `json:"total_media_accesses"`
	TotalEvents          int                `json:"total_events"`
	UniqueMediaAccessed  int                `json:"unique_media_accessed"`
	TotalPlaybackTime    time.Duration      `json:"total_playback_time"`
	MostAccessedMedia    []MediaAccessCount `json:"most_accessed_media"`
	PreferredAccessTimes map[string]int     `json:"preferred_access_times"`
	DeviceUsage          map[string]int     `json:"device_usage"`
	LocationAnalysis     map[string]int     `json:"location_analysis"`
}

// SystemAnalytics represents system-wide analytics
type SystemAnalytics struct {
	StartDate              time.Time              `json:"start_date"`
	EndDate                time.Time              `json:"end_date"`
	TotalUsers             int                    `json:"total_users"`
	ActiveUsers            int                    `json:"active_users"`
	TotalMediaAccesses     int                    `json:"total_media_accesses"`
	TotalEvents            int                    `json:"total_events"`
	TopAccessedMedia       []MediaAccessCount     `json:"top_accessed_media"`
	UserGrowthData         []UserGrowthPoint      `json:"user_growth_data"`
	AverageSessionDuration time.Duration          `json:"average_session_duration"`
	PeakUsageHours         map[string]int         `json:"peak_usage_hours"`
	PopularFileTypes       map[string]int         `json:"popular_file_types"`
	GeographicDistribution map[string]interface{} `json:"geographic_distribution"`
}

// MediaAnalytics represents analytics for specific media
type MediaAnalytics struct {
	MediaID             int                    `json:"media_id"`
	StartDate           time.Time              `json:"start_date"`
	EndDate             time.Time              `json:"end_date"`
	TotalAccesses       int                    `json:"total_accesses"`
	UniqueUsers         int                    `json:"unique_users"`
	TotalPlaybackTime   time.Duration          `json:"total_playback_time"`
	AveragePlaybackTime time.Duration          `json:"average_playback_time"`
	AccessPatterns      map[string]interface{} `json:"access_patterns"`
	UserRetention       float64                `json:"user_retention"`
	PopularTimeRanges   map[string]int         `json:"popular_time_ranges"`
	DevicePreferences   map[string]int         `json:"device_preferences"`
}

// MediaAccessCount represents media access statistics
type MediaAccessCount struct {
	MediaID     int `json:"media_id"`
	AccessCount int `json:"access_count"`
}

// UserGrowthPoint represents a point in user growth data
type UserGrowthPoint struct {
	Date      time.Time `json:"date"`
	UserCount int       `json:"user_count"`
}

// SessionData represents session information for analytics
type SessionData struct {
	UserID    int           `json:"user_id"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
}

// AnalyticsReport represents a generated analytics report
type AnalyticsReport struct {
	Type      string    `json:"type"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
}

// Favorites Models

// Favorite represents a user's favorite item
type Favorite struct {
	ID         int        `json:"id" db:"id"`
	UserID     int        `json:"user_id" db:"user_id"`
	EntityType string     `json:"entity_type" db:"entity_type"`
	EntityID   int        `json:"entity_id" db:"entity_id"`
	Category   *string    `json:"category,omitempty" db:"category"`
	Notes      *string    `json:"notes,omitempty" db:"notes"`
	Tags       *[]string  `json:"tags,omitempty" db:"tags"`
	IsPublic   bool       `json:"is_public" db:"is_public"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

// FavoriteCategory represents a category for organizing favorites
type FavoriteCategory struct {
	ID          int        `json:"id" db:"id"`
	UserID      int        `json:"user_id" db:"user_id"`
	Name        string     `json:"name" db:"name"`
	Description *string    `json:"description,omitempty" db:"description"`
	Color       *string    `json:"color,omitempty" db:"color"`
	Icon        *string    `json:"icon,omitempty" db:"icon"`
	IsPublic    bool       `json:"is_public" db:"is_public"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

// FavoriteShare represents sharing of favorites between users
type FavoriteShare struct {
	ID           int              `json:"id" db:"id"`
	FavoriteID   int              `json:"favorite_id" db:"favorite_id"`
	SharedByUser int              `json:"shared_by_user" db:"shared_by_user"`
	SharedWith   []int            `json:"shared_with" db:"shared_with"`
	Permissions  SharePermissions `json:"permissions" db:"permissions"`
	CreatedAt    time.Time        `json:"created_at" db:"created_at"`
	IsActive     bool             `json:"is_active" db:"is_active"`
}

// SharePermissions represents permissions for shared favorites
type SharePermissions struct {
	CanView   bool `json:"can_view"`
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
	CanShare  bool `json:"can_share"`
}

// FavoriteStatistics represents statistics about user's favorites
type FavoriteStatistics struct {
	UserID                int            `json:"user_id"`
	TotalFavorites        int            `json:"total_favorites"`
	FavoritesByEntityType map[string]int `json:"favorites_by_entity_type"`
	FavoritesByCategory   map[string]int `json:"favorites_by_category"`
	RecentFavorites       []Favorite     `json:"recent_favorites"`
}

// RecommendedFavorite represents a recommended favorite item
type RecommendedFavorite struct {
	Favorite        Favorite  `json:"favorite"`
	RecommendReason string    `json:"recommend_reason"`
	RecommendScore  float64   `json:"recommend_score"`
	RecommendedAt   time.Time `json:"recommended_at"`
}

// Request/Response Models

// UpdateFavoriteRequest represents a request to update a favorite
type UpdateFavoriteRequest struct {
	Category *string   `json:"category,omitempty"`
	Notes    *string   `json:"notes,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
	IsPublic *bool     `json:"is_public,omitempty"`
}

// UpdateFavoriteCategoryRequest represents a request to update a favorite category
type UpdateFavoriteCategoryRequest struct {
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

// BulkFavoriteRequest represents a request to add multiple favorites
type BulkFavoriteRequest struct {
	EntityType string    `json:"entity_type"`
	EntityID   int       `json:"entity_id"`
	Category   *string   `json:"category,omitempty"`
	Notes      *string   `json:"notes,omitempty"`
	Tags       *[]string `json:"tags,omitempty"`
	IsPublic   bool      `json:"is_public"`
}

// BulkFavoriteRemoveRequest represents a request to remove multiple favorites
type BulkFavoriteRemoveRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   int    `json:"entity_id"`
}

// Reporting Models

// GeneratedReport represents a generated report
type GeneratedReport struct {
	Type        string                 `json:"type"`
	Format      string                 `json:"format"`
	Content     []byte                 `json:"content"`
	GeneratedAt time.Time              `json:"generated_at"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// UserAnalyticsReport represents a user analytics report
type UserAnalyticsReport struct {
	User               *User                  `json:"user"`
	StartDate          time.Time              `json:"start_date"`
	EndDate            time.Time              `json:"end_date"`
	TotalMediaAccesses int                    `json:"total_media_accesses"`
	TotalEvents        int                    `json:"total_events"`
	MediaAccessLogs    []MediaAccessLog       `json:"media_access_logs"`
	Events             []AnalyticsEvent       `json:"events"`
	AccessPatterns     map[string]interface{} `json:"access_patterns"`
	DeviceUsage        map[string]int         `json:"device_usage"`
	LocationAnalysis   map[string]int         `json:"location_analysis"`
	TimePatterns       map[string]interface{} `json:"time_patterns"`
	PopularContent     []MediaAccessCount     `json:"popular_content"`
}

// SystemOverviewReport represents a system overview report
type SystemOverviewReport struct {
	StartDate          time.Time          `json:"start_date"`
	EndDate            time.Time          `json:"end_date"`
	TotalUsers         int                `json:"total_users"`
	ActiveUsers        int                `json:"active_users"`
	TotalMediaAccesses int                `json:"total_media_accesses"`
	TotalEvents        int                `json:"total_events"`
	TopAccessedMedia   []MediaAccessCount `json:"top_accessed_media"`
	UserGrowthData     []UserGrowthPoint  `json:"user_growth_data"`
	SystemHealth       SystemHealth       `json:"system_health"`
	UsageStatistics    UsageStatistics    `json:"usage_statistics"`
	PerformanceMetrics PerformanceMetrics `json:"performance_metrics"`
}

// MediaAnalyticsReport represents a media analytics report
type MediaAnalyticsReport struct {
	MediaID        int                    `json:"media_id"`
	StartDate      time.Time              `json:"start_date"`
	EndDate        time.Time              `json:"end_date"`
	TotalAccesses  int                    `json:"total_accesses"`
	UniqueUsers    int                    `json:"unique_users"`
	AccessLogs     []MediaAccessLog       `json:"access_logs"`
	AccessPatterns map[string]interface{} `json:"access_patterns"`
	UserEngagement UserEngagement         `json:"user_engagement"`
	GeographicData map[string]int         `json:"geographic_data"`
	DeviceAnalysis map[string]int         `json:"device_analysis"`
	TimeAnalysis   map[string]int         `json:"time_analysis"`
}

// UserActivityReport represents a user activity report
type UserActivityReport struct {
	StartDate      time.Time             `json:"start_date"`
	EndDate        time.Time             `json:"end_date"`
	UserActivities []UserActivitySummary `json:"user_activities"`
	TotalUsers     int                   `json:"total_users"`
	TotalAccesses  int                   `json:"total_accesses"`
	Summary        ActivitySummary       `json:"summary"`
}

// SecurityAuditReport represents a security audit report
type SecurityAuditReport struct {
	StartDate           time.Time          `json:"start_date"`
	EndDate             time.Time          `json:"end_date"`
	FailedLoginAttempts int                `json:"failed_login_attempts"`
	SuccessfulLogins    int                `json:"successful_logins"`
	SuspiciousActivity  []SecurityIncident `json:"suspicious_activity"`
	SecurityMetrics     SecurityMetrics    `json:"security_metrics"`
}

// PerformanceMetricsReport represents a performance metrics report
type PerformanceMetricsReport struct {
	StartDate              time.Time     `json:"start_date"`
	EndDate                time.Time     `json:"end_date"`
	AverageSessionDuration time.Duration `json:"average_session_duration"`
	TotalSessions          int           `json:"total_sessions"`
	ResponseTimes          ResponseTimes `json:"response_times"`
	SystemLoad             SystemLoad    `json:"system_load"`
	ErrorRates             ErrorRates    `json:"error_rates"`
}

// Supporting Types for Reports

type SystemHealthScore struct {
	Score  float64 `json:"score"`
	Status string  `json:"status"`
}

type UsageStatistics struct {
	PeakHours    []int   `json:"peak_hours"`
	AverageDaily int     `json:"average_daily"`
	GrowthRate   float64 `json:"growth_rate"`
}

type PerformanceMetrics struct {
	ResponseTime float64 `json:"response_time"`
	Throughput   int     `json:"throughput"`
	ErrorRate    float64 `json:"error_rate"`
}

type UserEngagement struct {
	AverageSessionTime float64 `json:"average_session_time"`
	ReturnRate         float64 `json:"return_rate"`
	InteractionDepth   float64 `json:"interaction_depth"`
}

type UserActivitySummary struct {
	User              *User     `json:"user"`
	TotalAccesses     int       `json:"total_accesses"`
	LastActivity      time.Time `json:"last_activity"`
	MostActiveHour    int       `json:"most_active_hour"`
	PreferredDevices  []string  `json:"preferred_devices"`
	AccessedLocations []string  `json:"accessed_locations"`
}

type ActivitySummary struct {
	TotalUsers       int     `json:"total_users"`
	TotalAccesses    int     `json:"total_accesses"`
	AverageAccesses  float64 `json:"average_accesses"`
	MostActiveUsers  int     `json:"most_active_users"`
	LeastActiveUsers int     `json:"least_active_users"`
}

type SecurityIncident struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	UserID      *int      `json:"user_id,omitempty"`
	IPAddress   *string   `json:"ip_address,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

type SecurityMetrics struct {
	ThreatLevel        string  `json:"threat_level"`
	VulnerabilityCount int     `json:"vulnerability_count"`
	SecurityScore      float64 `json:"security_score"`
}

type ResponseTimes struct {
	Average float64 `json:"average"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	P95     float64 `json:"p95"`
	P99     float64 `json:"p99"`
}

type SystemLoad struct {
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	Disk    float64 `json:"disk"`
	Network float64 `json:"network"`
}

type ErrorRates struct {
	HTTP4xx  float64 `json:"http_4xx"`
	HTTP5xx  float64 `json:"http_5xx"`
	Timeouts float64 `json:"timeouts"`
	Total    float64 `json:"total"`
}

// Conversion Models

// ConversionJob represents a media conversion job
type ConversionJob struct {
	ID             int            `json:"id" db:"id"`
	UserID         int            `json:"user_id" db:"user_id"`
	SourcePath     string         `json:"source_path" db:"source_path"`
	TargetPath     string         `json:"target_path" db:"target_path"`
	SourceFormat   string         `json:"source_format" db:"source_format"`
	TargetFormat   string         `json:"target_format" db:"target_format"`
	ConversionType string         `json:"conversion_type" db:"conversion_type"`
	Quality        string         `json:"quality" db:"quality"`
	Settings       *string        `json:"settings,omitempty" db:"settings"`
	Priority       int            `json:"priority" db:"priority"`
	Status         string         `json:"status" db:"status"`
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	StartedAt      *time.Time     `json:"started_at,omitempty" db:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty" db:"completed_at"`
	ScheduledFor   *time.Time     `json:"scheduled_for,omitempty" db:"scheduled_for"`
	Duration       *time.Duration `json:"duration,omitempty" db:"duration"`
	ErrorMessage   *string        `json:"error_message,omitempty" db:"error_message"`
}

// ConversionRequest represents a request to create a conversion job
type ConversionRequest struct {
	SourcePath     string     `json:"source_path"`
	TargetPath     string     `json:"target_path"`
	SourceFormat   string     `json:"source_format"`
	TargetFormat   string     `json:"target_format"`
	ConversionType string     `json:"conversion_type"`
	Quality        string     `json:"quality"`
	Settings       *string    `json:"settings,omitempty"`
	Priority       int        `json:"priority"`
	ScheduledFor   *time.Time `json:"scheduled_for,omitempty"`
}

// ConversionStatistics represents conversion statistics
type ConversionStatistics struct {
	StartDate       time.Time      `json:"start_date"`
	EndDate         time.Time      `json:"end_date"`
	TotalJobs       int            `json:"total_jobs"`
	ByStatus        map[string]int `json:"by_status"`
	ByType          map[string]int `json:"by_type"`
	ByFormat        map[string]int `json:"by_format"`
	AverageDuration *time.Duration `json:"average_duration,omitempty"`
	SuccessRate     float64        `json:"success_rate"`
}

// SupportedFormats represents supported conversion formats
type SupportedFormats struct {
	Video    VideoFormats    `json:"video"`
	Audio    AudioFormats    `json:"audio"`
	Document DocumentFormats `json:"document"`
	Image    ImageFormats    `json:"image"`
}

// VideoFormats represents supported video formats
type VideoFormats struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// AudioFormats represents supported audio formats
type AudioFormats struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// DocumentFormats represents supported document formats
type DocumentFormats struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// ImageFormats represents supported image formats
type ImageFormats struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// FormatPopularity represents format usage statistics
type FormatPopularity struct {
	Format string `json:"format"`
	Count  int    `json:"count"`
}

// Conversion Status Constants
const (
	ConversionStatusPending   = "pending"
	ConversionStatusRunning   = "running"
	ConversionStatusCompleted = "completed"
	ConversionStatusFailed    = "failed"
	ConversionStatusCancelled = "cancelled"
)

// Conversion Type Constants
const (
	ConversionTypeVideo    = "video"
	ConversionTypeAudio    = "audio"
	ConversionTypeDocument = "document"
	ConversionTypeImage    = "image"
)

// Sync and Backup Models

// SyncEndpoint represents a sync endpoint configuration
type SyncEndpoint struct {
	ID            int        `json:"id" db:"id"`
	UserID        int        `json:"user_id" db:"user_id"`
	Name          string     `json:"name" db:"name"`
	Type          string     `json:"type" db:"type"`
	URL           string     `json:"url" db:"url"`
	Username      string     `json:"username" db:"username"`
	Password      string     `json:"-" db:"password"` // Never include in JSON
	SyncDirection string     `json:"sync_direction" db:"sync_direction"`
	LocalPath     string     `json:"local_path" db:"local_path"`
	RemotePath    string     `json:"remote_path" db:"remote_path"`
	SyncSettings  *string    `json:"sync_settings,omitempty" db:"sync_settings"`
	Status        string     `json:"status" db:"status"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty" db:"last_sync_at"`
}

// SyncSession represents a sync session
type SyncSession struct {
	ID           int            `json:"id" db:"id"`
	EndpointID   int            `json:"endpoint_id" db:"endpoint_id"`
	UserID       int            `json:"user_id" db:"user_id"`
	Status       string         `json:"status" db:"status"`
	SyncType     string         `json:"sync_type" db:"sync_type"`
	StartedAt    time.Time      `json:"started_at" db:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty" db:"completed_at"`
	Duration     *time.Duration `json:"duration,omitempty" db:"duration"`
	TotalFiles   int            `json:"total_files" db:"total_files"`
	SyncedFiles  int            `json:"synced_files" db:"synced_files"`
	FailedFiles  int            `json:"failed_files" db:"failed_files"`
	SkippedFiles int            `json:"skipped_files" db:"skipped_files"`
	ErrorMessage *string        `json:"error_message,omitempty" db:"error_message"`
}

// SyncSchedule represents a scheduled sync
type SyncSchedule struct {
	ID         int        `json:"id" db:"id"`
	EndpointID int        `json:"endpoint_id" db:"endpoint_id"`
	UserID     int        `json:"user_id" db:"user_id"`
	Frequency  string     `json:"frequency" db:"frequency"`
	LastRun    *time.Time `json:"last_run,omitempty" db:"last_run"`
	NextRun    *time.Time `json:"next_run,omitempty" db:"next_run"`
	IsActive   bool       `json:"is_active" db:"is_active"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// SyncStatistics represents sync statistics
type SyncStatistics struct {
	StartDate        time.Time      `json:"start_date"`
	EndDate          time.Time      `json:"end_date"`
	TotalSessions    int            `json:"total_sessions"`
	TotalFilesSynced int            `json:"total_files_synced"`
	TotalFilesFailed int            `json:"total_files_failed"`
	ByStatus         map[string]int `json:"by_status"`
	ByType           map[string]int `json:"by_type"`
	AverageDuration  *time.Duration `json:"average_duration,omitempty"`
	SuccessRate      float64        `json:"success_rate"`
}

// UpdateSyncEndpointRequest represents a request to update a sync endpoint
type UpdateSyncEndpointRequest struct {
	Name          string  `json:"name,omitempty"`
	URL           string  `json:"url,omitempty"`
	Username      string  `json:"username,omitempty"`
	Password      string  `json:"password,omitempty"`
	SyncDirection string  `json:"sync_direction,omitempty"`
	LocalPath     string  `json:"local_path,omitempty"`
	RemotePath    string  `json:"remote_path,omitempty"`
	SyncSettings  *string `json:"sync_settings,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
}

// Sync Status Constants
const (
	SyncStatusActive   = "active"
	SyncStatusInactive = "inactive"
	SyncStatusError    = "error"
)

// Sync Session Status Constants
const (
	SyncSessionStatusRunning   = "running"
	SyncSessionStatusCompleted = "completed"
	SyncSessionStatusFailed    = "failed"
	SyncSessionStatusCancelled = "cancelled"
)

// Sync Type Constants
const (
	SyncTypeWebDAV       = "webdav"
	SyncTypeCloudStorage = "cloud_storage"
	SyncTypeLocal        = "local"
	SyncTypeManual       = "manual"
	SyncTypeScheduled    = "scheduled"
)

// Sync Direction Constants
const (
	SyncDirectionUpload        = "upload"
	SyncDirectionDownload      = "download"
	SyncDirectionBidirectional = "bidirectional"
)

// Sync Frequency Constants
const (
	SyncFrequencyHourly  = "hourly"
	SyncFrequencyDaily   = "daily"
	SyncFrequencyWeekly  = "weekly"
	SyncFrequencyMonthly = "monthly"
)

// Error and Crash Reporting Models

// ErrorReport represents an error report
type ErrorReport struct {
	ID          int                    `json:"id" db:"id"`
	UserID      int                    `json:"user_id" db:"user_id"`
	Level       string                 `json:"level" db:"level"`
	Message     string                 `json:"message" db:"message"`
	ErrorCode   string                 `json:"error_code" db:"error_code"`
	Component   string                 `json:"component" db:"component"`
	StackTrace  string                 `json:"stack_trace" db:"stack_trace"`
	Context     map[string]interface{} `json:"context" db:"context"`
	SystemInfo  map[string]interface{} `json:"system_info" db:"system_info"`
	UserAgent   string                 `json:"user_agent" db:"user_agent"`
	URL         string                 `json:"url" db:"url"`
	Fingerprint string                 `json:"fingerprint" db:"fingerprint"`
	Status      string                 `json:"status" db:"status"`
	ReportedAt  time.Time              `json:"reported_at" db:"reported_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty" db:"resolved_at"`
}

// CrashReport represents a crash report
type CrashReport struct {
	ID          int                    `json:"id" db:"id"`
	UserID      int                    `json:"user_id" db:"user_id"`
	Signal      string                 `json:"signal" db:"signal"`
	Message     string                 `json:"message" db:"message"`
	StackTrace  string                 `json:"stack_trace" db:"stack_trace"`
	Context     map[string]interface{} `json:"context" db:"context"`
	SystemInfo  map[string]interface{} `json:"system_info" db:"system_info"`
	Fingerprint string                 `json:"fingerprint" db:"fingerprint"`
	Status      string                 `json:"status" db:"status"`
	ReportedAt  time.Time              `json:"reported_at" db:"reported_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty" db:"resolved_at"`
}

// ErrorReportRequest represents a request to create an error report
type ErrorReportRequest struct {
	Level      string                 `json:"level" validate:"required"`
	Message    string                 `json:"message" validate:"required"`
	ErrorCode  string                 `json:"error_code,omitempty"`
	Component  string                 `json:"component,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	URL        string                 `json:"url,omitempty"`
}

// CrashReportRequest represents a request to create a crash report
type CrashReportRequest struct {
	Signal     string                 `json:"signal" validate:"required"`
	Message    string                 `json:"message" validate:"required"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// ErrorReportFilters represents filters for error report queries
type ErrorReportFilters struct {
	Level     string     `json:"level,omitempty"`
	Component string     `json:"component,omitempty"`
	Status    string     `json:"status,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
}

// CrashReportFilters represents filters for crash report queries
type CrashReportFilters struct {
	Signal    string     `json:"signal,omitempty"`
	Status    string     `json:"status,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
}

// ExportFilters represents filters for report export
type ExportFilters struct {
	Format         string     `json:"format"`
	Level          string     `json:"level,omitempty"`
	Component      string     `json:"component,omitempty"`
	Signal         string     `json:"signal,omitempty"`
	StartDate      *time.Time `json:"start_date,omitempty"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	Limit          int        `json:"limit,omitempty"`
	IncludeErrors  bool       `json:"include_errors"`
	IncludeCrashes bool       `json:"include_crashes"`
}

// ErrorStatistics represents error reporting statistics
type ErrorStatistics struct {
	TotalErrors       int            `json:"total_errors"`
	ErrorsByLevel     map[string]int `json:"errors_by_level"`
	ErrorsByComponent map[string]int `json:"errors_by_component"`
	RecentErrors      int            `json:"recent_errors"`
	ResolvedErrors    int            `json:"resolved_errors"`
	AvgResolutionTime float64        `json:"avg_resolution_time"`
}

// CrashStatistics represents crash reporting statistics
type CrashStatistics struct {
	TotalCrashes      int            `json:"total_crashes"`
	CrashesBySignal   map[string]int `json:"crashes_by_signal"`
	RecentCrashes     int            `json:"recent_crashes"`
	ResolvedCrashes   int            `json:"resolved_crashes"`
	AvgResolutionTime float64        `json:"avg_resolution_time"`
	CrashRate         float64        `json:"crash_rate"`
}

// SystemHealth represents system health status
type SystemHealth struct {
	Score     float64                `json:"score"`
	Status    string                 `json:"status"`
	CheckedAt time.Time              `json:"checked_at"`
	Metrics   map[string]interface{} `json:"metrics"`
}

// TopError represents frequently occurring errors
type TopError struct {
	Fingerprint string    `json:"fingerprint"`
	Count       int       `json:"count"`
	LastSeen    time.Time `json:"last_seen"`
	FirstSeen   time.Time `json:"first_seen"`
	Message     string    `json:"message"`
	Component   string    `json:"component"`
	Level       string    `json:"level"`
}

// TopCrash represents frequently occurring crashes
type TopCrash struct {
	Fingerprint string    `json:"fingerprint"`
	Count       int       `json:"count"`
	LastSeen    time.Time `json:"last_seen"`
	FirstSeen   time.Time `json:"first_seen"`
	Message     string    `json:"message"`
	Signal      string    `json:"signal"`
}

// CrashTrend represents crash trends over time
type CrashTrend struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}

// Error Status Constants
const (
	ErrorStatusNew        = "new"
	ErrorStatusInProgress = "in_progress"
	ErrorStatusResolved   = "resolved"
	ErrorStatusIgnored    = "ignored"
)

// Crash Status Constants
const (
	CrashStatusNew        = "new"
	CrashStatusInProgress = "in_progress"
	CrashStatusResolved   = "resolved"
	CrashStatusIgnored    = "ignored"
)

// Error Level Constants
const (
	ErrorLevelDebug   = "debug"
	ErrorLevelInfo    = "info"
	ErrorLevelWarning = "warning"
	ErrorLevelError   = "error"
	ErrorLevelFatal   = "fatal"
)

// Signal Constants
const (
	SignalSIGABRT = "SIGABRT"
	SignalSIGFPE  = "SIGFPE"
	SignalSIGILL  = "SIGILL"
	SignalSIGINT  = "SIGINT"
	SignalSIGSEGV = "SIGSEGV"
	SignalSIGTERM = "SIGTERM"
)

// Log Management Models

// LogCollection represents a collection of logs
type LogCollection struct {
	ID          int                    `json:"id" db:"id"`
	UserID      int                    `json:"user_id" db:"user_id"`
	Name        string                 `json:"name" db:"name"`
	Description string                 `json:"description" db:"description"`
	Components  []string               `json:"components" db:"components"`
	LogLevel    string                 `json:"log_level" db:"log_level"`
	StartTime   *time.Time             `json:"start_time,omitempty" db:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty" db:"end_time"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	Status      string                 `json:"status" db:"status"`
	EntryCount  int                    `json:"entry_count" db:"entry_count"`
	Filters     map[string]interface{} `json:"filters,omitempty" db:"filters"`
}

// LogEntry represents a single log entry
type LogEntry struct {
	ID           int                    `json:"id" db:"id"`
	CollectionID int                    `json:"collection_id" db:"collection_id"`
	Timestamp    time.Time              `json:"timestamp" db:"timestamp"`
	Level        string                 `json:"level" db:"level"`
	Component    string                 `json:"component" db:"component"`
	Message      string                 `json:"message" db:"message"`
	Context      map[string]interface{} `json:"context,omitempty" db:"context"`
}

// LogShare represents a shared log collection
type LogShare struct {
	ID           int        `json:"id" db:"id"`
	CollectionID int        `json:"collection_id" db:"collection_id"`
	UserID       int        `json:"user_id" db:"user_id"`
	ShareToken   string     `json:"share_token" db:"share_token"`
	ShareType    string     `json:"share_type" db:"share_type"`
	ExpiresAt    time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	AccessedAt   *time.Time `json:"accessed_at,omitempty" db:"accessed_at"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	Permissions  []string   `json:"permissions" db:"permissions"`
	Recipients   []string   `json:"recipients,omitempty" db:"recipients"`
}

// LogCollectionRequest represents a request to create a log collection
type LogCollectionRequest struct {
	Name        string                 `json:"name" validate:"required"`
	Description string                 `json:"description,omitempty"`
	Components  []string               `json:"components" validate:"required"`
	LogLevel    string                 `json:"log_level,omitempty"`
	StartTime   *time.Time             `json:"start_time,omitempty"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
}

// LogShareRequest represents a request to create a log share
type LogShareRequest struct {
	CollectionID int        `json:"collection_id" validate:"required"`
	ShareType    string     `json:"share_type" validate:"required"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Permissions  []string   `json:"permissions" validate:"required"`
	Recipients   []string   `json:"recipients,omitempty"`
}

// LogEntryFilters represents filters for log entry queries
type LogEntryFilters struct {
	Level     string     `json:"level,omitempty"`
	Component string     `json:"component,omitempty"`
	Search    string     `json:"search,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
}

// LogStreamFilters represents filters for log streaming
type LogStreamFilters struct {
	Level     string `json:"level,omitempty"`
	Component string `json:"component,omitempty"`
	Search    string `json:"search,omitempty"`
}

// LogAnalysis represents log analysis results
type LogAnalysis struct {
	CollectionID       int            `json:"collection_id"`
	TotalEntries       int            `json:"total_entries"`
	EntriesByLevel     map[string]int `json:"entries_by_level"`
	EntriesByComponent map[string]int `json:"entries_by_component"`
	ErrorPatterns      map[string]int `json:"error_patterns"`
	TimeRange          *TimeRange     `json:"time_range"`
	Insights           []string       `json:"insights"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// LogStatistics represents log management statistics
type LogStatistics struct {
	TotalCollections    int            `json:"total_collections"`
	TotalEntries        int            `json:"total_entries"`
	ActiveShares        int            `json:"active_shares"`
	CollectionsByStatus map[string]int `json:"collections_by_status"`
	RecentCollections   int            `json:"recent_collections"`
}

// Log Collection Status Constants
const (
	LogCollectionStatusPending    = "pending"
	LogCollectionStatusInProgress = "in_progress"
	LogCollectionStatusCompleted  = "completed"
	LogCollectionStatusFailed     = "failed"
)

// Log Share Type Constants
const (
	LogShareTypePublic   = "public"
	LogShareTypePrivate  = "private"
	LogShareTypeInternal = "internal"
)

// Log Level Constants
const (
	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
	LogLevelFatal   = "fatal"
)

// Configuration and Installer Models

// SystemConfiguration represents the complete system configuration
type SystemConfiguration struct {
	Version          string                  `json:"version" db:"version"`
	CreatedAt        time.Time               `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at" db:"updated_at"`
	Database         *DatabaseConfig         `json:"database,omitempty"`
	Storage          *StorageConfig          `json:"storage,omitempty"`
	Network          *NetworkConfig          `json:"network,omitempty"`
	Authentication   *AuthenticationConfig   `json:"authentication,omitempty"`
	ExternalServices *ExternalServicesConfig `json:"external_services,omitempty"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	MediaDirectory     string `json:"media_directory"`
	ThumbnailDirectory string `json:"thumbnail_directory"`
	TempDirectory      string `json:"temp_directory"`
	MaxFileSize        int64  `json:"max_file_size"`
	StorageQuota       int64  `json:"storage_quota,omitempty"`
}

// NetworkConfig represents network configuration
type NetworkConfig struct {
	Host  string       `json:"host"`
	Port  int          `json:"port"`
	HTTPS *HTTPSConfig `json:"https,omitempty"`
	CORS  *CORSConfig  `json:"cors,omitempty"`
}

// HTTPSConfig represents HTTPS configuration
type HTTPSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertPath string `json:"cert_path,omitempty"`
	KeyPath  string `json:"key_path,omitempty"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods,omitempty"`
	AllowedHeaders []string `json:"allowed_headers,omitempty"`
}

// AuthenticationConfig represents authentication configuration
type AuthenticationConfig struct {
	JWTSecret                string        `json:"jwt_secret"`
	SessionTimeout           time.Duration `json:"session_timeout"`
	EnableRegistration       bool          `json:"enable_registration"`
	RequireEmailVerification bool          `json:"require_email_verification"`
	AdminEmail               string        `json:"admin_email,omitempty"`
}

// ExternalServicesConfig represents external service configurations
type ExternalServicesConfig struct {
	SMTP      *SMTPConfig  `json:"smtp,omitempty"`
	Slack     *SlackConfig `json:"slack,omitempty"`
	Analytics bool         `json:"analytics"`
}

// SMTPConfig represents SMTP configuration
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// SlackConfig represents Slack integration configuration
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// WizardStep represents a setup wizard step
type WizardStep struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Type         string                 `json:"type"`
	Required     bool                   `json:"required"`
	Order        int                    `json:"order"`
	Fields       []*WizardField         `json:"fields,omitempty"`
	Content      map[string]interface{} `json:"content,omitempty"`
	Validation   map[string]interface{} `json:"validation,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
}

// WizardField represents a field in a wizard step
type WizardField struct {
	Name         string                 `json:"name"`
	Label        string                 `json:"label"`
	Type         string                 `json:"type"`
	Required     bool                   `json:"required"`
	DefaultValue interface{}            `json:"default_value,omitempty"`
	Options      []string               `json:"options,omitempty"`
	Validation   map[string]interface{} `json:"validation,omitempty"`
	ShowWhen     map[string]interface{} `json:"show_when,omitempty"`
	Generate     bool                   `json:"generate,omitempty"`
}

// WizardProgress represents wizard completion progress
type WizardProgress struct {
	UserID      int                    `json:"user_id" db:"user_id"`
	CurrentStep string                 `json:"current_step" db:"current_step"`
	StepData    map[string]interface{} `json:"step_data" db:"step_data"`
	AllData     map[string]interface{} `json:"all_data" db:"all_data"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// WizardStepValidation represents validation results for a wizard step
type WizardStepValidation struct {
	StepID   string            `json:"step_id"`
	Valid    bool              `json:"valid"`
	Errors   map[string]string `json:"errors"`
	Warnings map[string]string `json:"warnings"`
}

// ConfigurationSchema represents the configuration schema
type ConfigurationSchema struct {
	Version  string           `json:"version"`
	Sections []*ConfigSection `json:"sections"`
}

// ConfigSection represents a configuration section
type ConfigSection struct {
	Name        string         `json:"name"`
	Key         string         `json:"key"`
	Description string         `json:"description"`
	Fields      []*ConfigField `json:"fields"`
}

// ConfigField represents a configuration field
type ConfigField struct {
	Name         string      `json:"name"`
	Label        string      `json:"label"`
	Type         string      `json:"type"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Description  string      `json:"description,omitempty"`
}

// ConfigurationTest represents configuration test results
type ConfigurationTest struct {
	TestedAt      time.Time              `json:"tested_at"`
	OverallStatus string                 `json:"overall_status"`
	Results       map[string]*TestResult `json:"results"`
}

// TestResult represents a single test result
type TestResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ConfigurationHistory represents configuration history
type ConfigurationHistory struct {
	ID        int       `json:"id" db:"id"`
	Version   string    `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ConfigurationBackup represents a configuration backup
type ConfigurationBackup struct {
	ID            int                  `json:"id" db:"id"`
	Name          string               `json:"name" db:"name"`
	Version       string               `json:"version" db:"version"`
	Configuration *SystemConfiguration `json:"configuration,omitempty" db:"configuration"`
	CreatedAt     time.Time            `json:"created_at" db:"created_at"`
}

// ConfigurationTemplate represents a configuration template
type ConfigurationTemplate struct {
	ID            int                  `json:"id" db:"id"`
	Name          string               `json:"name" db:"name"`
	Description   string               `json:"description" db:"description"`
	Category      string               `json:"category" db:"category"`
	Configuration *SystemConfiguration `json:"configuration" db:"configuration"`
	CreatedAt     time.Time            `json:"created_at" db:"created_at"`
}

// ConfigurationStatistics represents configuration statistics
type ConfigurationStatistics struct {
	TotalConfigurations int       `json:"total_configurations"`
	TotalBackups        int       `json:"total_backups"`
	TotalTemplates      int       `json:"total_templates"`
	WizardCompletions   int       `json:"wizard_completions"`
	LastUpdate          time.Time `json:"last_update"`
}

// Wizard Step Type Constants
const (
	WizardStepTypeInfo     = "info"
	WizardStepTypeForm     = "form"
	WizardStepTypeSummary  = "summary"
	WizardStepTypeComplete = "complete"
)

// Field Type Constants
const (
	FieldTypeText      = "text"
	FieldTypePassword  = "password"
	FieldTypeEmail     = "email"
	FieldTypeNumber    = "number"
	FieldTypeCheckbox  = "checkbox"
	FieldTypeSelect    = "select"
	FieldTypeTextarea  = "textarea"
	FieldTypeFile      = "file"
	FieldTypeDirectory = "directory"
)

// Test Status Constants
const (
	TestStatusPassed  = "passed"
	TestStatusFailed  = "failed"
	TestStatusWarning = "warning"
)

// Configuration Category Constants
const (
	ConfigCategoryEnvironment = "Environment"
	ConfigCategoryPerformance = "Performance"
	ConfigCategorySecurity    = "Security"
	ConfigCategoryDevelopment = "Development"
)

// Playlist represents a user-created playlist of media entities.
type Playlist struct {
	ID          int            `json:"id" db:"id"`
	UserID      int            `json:"user_id" db:"user_id"`
	Name        string         `json:"name" db:"name"`
	Description string         `json:"description" db:"description"`
	IsPublic    bool           `json:"is_public" db:"is_public"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
	Items       []PlaylistItem `json:"items,omitempty"`
}

// PlaylistItem represents a single item within a playlist.
type PlaylistItem struct {
	ID         int       `json:"id" db:"id"`
	PlaylistID int       `json:"playlist_id" db:"playlist_id"`
	EntityID   int       `json:"entity_id" db:"entity_id"`
	EntityType string    `json:"entity_type" db:"entity_type"`
	Position   int       `json:"position" db:"position"`
	AddedAt    time.Time `json:"added_at" db:"added_at"`
}

// CreatePlaylistRequest represents a request to create a new playlist.
type CreatePlaylistRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdatePlaylistRequest represents a request to update an existing playlist.
type UpdatePlaylistRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

// AddPlaylistItemRequest represents a request to add an item to a playlist.
type AddPlaylistItemRequest struct {
	EntityID   int    `json:"entity_id" binding:"required"`
	EntityType string `json:"entity_type" binding:"required"`
}

// Common Error Variables
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)
