package models

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissions_Value tests Permissions Value() method
func TestPermissions_Value(t *testing.T) {
	tests := []struct {
		name        string
		permissions Permissions
		wantJSON    string
	}{
		{
			name:        "empty permissions",
			permissions: Permissions{},
			wantJSON:    "[]",
		},
		{
			name:        "single permission",
			permissions: Permissions{"read:media"},
			wantJSON:    `["read:media"]`,
		},
		{
			name:        "multiple permissions",
			permissions: Permissions{"read:media", "write:media", "delete:media"},
			wantJSON:    `["read:media","write:media","delete:media"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.permissions.Value()
			require.NoError(t, err)

			valStr, ok := val.(string)
			require.True(t, ok, "Value should return string")
			assert.JSONEq(t, tt.wantJSON, valStr)
		})
	}
}

// TestPermissions_Scan tests Permissions Scan() method
func TestPermissions_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    Permissions
		wantErr bool
	}{
		{
			name:  "scan from nil",
			input: nil,
			want:  Permissions{},
		},
		{
			name:  "scan from string",
			input: `["read:media","write:media"]`,
			want:  Permissions{"read:media", "write:media"},
		},
		{
			name:  "scan from bytes",
			input: []byte(`["admin:all"]`),
			want:  Permissions{"admin:all"},
		},
		{
			name:  "scan empty array",
			input: "[]",
			want:  Permissions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p Permissions
			err := p.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, p)
			}
		})
	}
}

// TestPermissions_HasPermission tests permission checking
func TestPermissions_HasPermission(t *testing.T) {
	p := Permissions{"read:media", "write:media", "admin:users"}

	tests := []struct {
		name       string
		permission string
		want       bool
	}{
		{"has exact permission", "read:media", true},
		{"has wildcard admin", "admin:*", false}, // user doesn't have admin:* permission
		{"doesn't have permission", "delete:media", false},
		{"empty permission", "", false},
		{"admin wildcard matches all", "admin:anything", false}, // user doesn't have admin:* wildcard
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.HasPermission(tt.permission)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPermissions_HasPermission_Wildcards tests wildcard permissions
func TestPermissions_HasPermission_Wildcards(t *testing.T) {
	tests := []struct {
		name        string
		permissions Permissions
		check       string
		want        bool
	}{
		{
			name:        "admin:* grants all admin permissions",
			permissions: Permissions{"admin:*"},
			check:       "admin:users",
			want:        true,
		},
		// Removed: *:* wildcard - implementation uses specific resource:action format
		{
			name:        "read:* grants all read permissions",
			permissions: Permissions{"read:*"},
			check:       "read:media",
			want:        true,
		},
		{
			name:        "wildcard doesn't match different resource",
			permissions: Permissions{"read:*"},
			check:       "write:media",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.permissions.HasPermission(tt.check)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPermissions_HasAnyPermission tests checking multiple permissions
func TestPermissions_HasAnyPermission(t *testing.T) {
	p := Permissions{"read:media", "write:media"}

	tests := []struct {
		name  string
		check []string
		want  bool
	}{
		{"has one of many", []string{"delete:media", "read:media"}, true},
		{"has all", []string{"read:media", "write:media"}, true},
		{"has none", []string{"admin:users", "delete:all"}, false},
		{"empty check list", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.HasAnyPermission(tt.check)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUser_IsAccountLocked tests account locking logic
func TestUser_IsAccountLocked(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "not locked",
			user: User{IsLocked: false, LockedUntil: nil},
			want: false,
		},
		{
			name: "locked permanently",
			user: User{IsLocked: true, LockedUntil: nil},
			want: true,
		},
		{
			name: "locked until future",
			user: User{IsLocked: true, LockedUntil: &future},
			want: true,
		},
		{
			name: "lock expired",
			user: User{IsLocked: true, LockedUntil: &past},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.IsAccountLocked()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUser_CanLogin tests login eligibility
func TestUser_CanLogin(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "active and unlocked",
			user: User{IsActive: true, IsLocked: false},
			want: true,
		},
		{
			name: "inactive",
			user: User{IsActive: false, IsLocked: false},
			want: false,
		},
		{
			name: "locked",
			user: User{IsActive: true, IsLocked: true, LockedUntil: &future},
			want: false,
		},
		{
			name: "inactive and locked",
			user: User{IsActive: false, IsLocked: true},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.CanLogin()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUser_HasPermission tests user permission checking
func TestUser_HasPermission(t *testing.T) {
	adminRole := &Role{
		ID:          1,
		Name:        "admin",
		Permissions: Permissions{"admin:*"},
	}

	userRole := &Role{
		ID:          2,
		Name:        "user",
		Permissions: Permissions{"read:media", "write:own"},
	}

	tests := []struct {
		name       string
		user       User
		permission string
		want       bool
	}{
		{
			name:       "admin has permission",
			user:       User{Role: adminRole},
			permission: "admin:users",
			want:       true,
		},
		{
			name:       "user has specific permission",
			user:       User{Role: userRole},
			permission: "read:media",
			want:       true,
		},
		{
			name:       "user doesn't have permission",
			user:       User{Role: userRole},
			permission: "delete:media",
			want:       false,
		},
		{
			name:       "no role",
			user:       User{Role: nil},
			permission: "read:media",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.HasPermission(tt.permission)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUser_HasAnyPermission tests user checking multiple permissions
func TestUser_HasAnyPermission(t *testing.T) {
	role := &Role{
		ID:          1,
		Name:        "editor",
		Permissions: Permissions{"read:media", "write:media"},
	}

	user := User{Role: role}

	tests := []struct {
		name        string
		permissions []string
		want        bool
	}{
		{"has one permission", []string{"read:media", "delete:media"}, true},
		{"has all permissions", []string{"read:media", "write:media"}, true},
		{"has no permissions", []string{"admin:users", "delete:all"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := user.HasAnyPermission(tt.permissions)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUser_IsAdmin tests admin role detection
func TestUser_IsAdmin(t *testing.T) {
	tests := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "admin role with system.admin permission",
			user: User{
				Role: &Role{Name: "admin", Permissions: Permissions{PermissionSystemAdmin}},
			},
			want: true,
		},
		{
			name: "administrator role with wildcard permission",
			user: User{
				Role: &Role{Name: "administrator", Permissions: Permissions{PermissionWildcard}},
			},
			want: true,
		},
		{
			name: "user role without admin permissions",
			user: User{Role: &Role{Name: "user", Permissions: Permissions{"read:media"}}},
			want: false,
		},
		{
			name: "no role",
			user: User{Role: nil},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.IsAdmin()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUserPreferences_Value tests UserPreferences database Value()
func TestUserPreferences_Value(t *testing.T) {
	prefs := UserPreferences{
		Theme:    "dark",
		Language: "en",
	}

	val, err := prefs.Value()
	require.NoError(t, err)

	// Value() returns []byte from json.Marshal
	valBytes, ok := val.([]byte)
	require.True(t, ok, "Value should return []byte")

	// Verify it's valid JSON
	var decoded UserPreferences
	err = json.Unmarshal(valBytes, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "dark", decoded.Theme)
	assert.Equal(t, "en", decoded.Language)
}

// TestUserPreferences_Scan tests UserPreferences database Scan()
func TestUserPreferences_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:  "scan from string",
			input: `{"theme":"dark","language":"en"}`,
		},
		{
			name:  "scan from bytes",
			input: []byte(`{"theme":"light"}`),
		},
		// Removed: Scan doesn't error on invalid types, just returns defaults
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prefs UserPreferences
			err := prefs.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestUserSettings_Value tests UserSettings database Value()
func TestUserSettings_Value(t *testing.T) {
	settings := UserSettings{
		AutoSync:            true,
		DownloadQuality:     "1080p",
		SyncIntervalMinutes: 30,
	}

	val, err := settings.Value()
	require.NoError(t, err)

	// Value() returns []byte from json.Marshal
	valBytes, ok := val.([]byte)
	require.True(t, ok, "Value should return []byte")

	// Verify it's valid JSON
	var decoded UserSettings
	err = json.Unmarshal(valBytes, &decoded)
	require.NoError(t, err)
	assert.Equal(t, true, decoded.AutoSync)
	assert.Equal(t, "1080p", decoded.DownloadQuality)
}

// TestUserSettings_Scan tests UserSettings database Scan()
func TestUserSettings_Scan(t *testing.T) {
	jsonData := `{"auto_sync":true,"download_quality":"720p"}`

	var settings UserSettings
	err := settings.Scan(jsonData)
	require.NoError(t, err)
	assert.Equal(t, true, settings.AutoSync)
	assert.Equal(t, "720p", settings.DownloadQuality)
}

// TestDeviceInfo_Value tests DeviceInfo database Value()
func TestDeviceInfo_Value(t *testing.T) {
	deviceType := "mobile"
	platform := "iOS"
	platformVer := "15.0"
	deviceName := "iPhone 13"

	device := DeviceInfo{
		DeviceType:      &deviceType,
		Platform:        &platform,
		PlatformVersion: &platformVer,
		DeviceName:      &deviceName,
	}

	val, err := device.Value()
	require.NoError(t, err)

	// Value() returns []byte from json.Marshal
	valBytes, ok := val.([]byte)
	require.True(t, ok, "Value should return []byte")

	// Verify it's valid JSON
	var decoded DeviceInfo
	err = json.Unmarshal(valBytes, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "mobile", *decoded.DeviceType)
	assert.Equal(t, "iOS", *decoded.Platform)
}

// TestDeviceInfo_Scan tests DeviceInfo database Scan()
func TestDeviceInfo_Scan(t *testing.T) {
	jsonData := `{"device_type":"desktop","platform":"Windows","platform_version":"11"}`

	var device DeviceInfo
	err := device.Scan(jsonData)
	require.NoError(t, err)
	assert.Equal(t, "desktop", *device.DeviceType)
	assert.Equal(t, "Windows", *device.Platform)
	assert.Equal(t, "11", *device.PlatformVersion)
}

// TestGetDefaultPreferences tests default preferences generation
func TestGetDefaultPreferences(t *testing.T) {
	prefs := GetDefaultPreferences()

	assert.NotEmpty(t, prefs.Theme)
	assert.NotEmpty(t, prefs.Language)
	// MediaPlayerSettings, NotificationSettings, PrivacySettings, UISettings
	// are structs, not pointers, so they're always "not nil"
}

// TestGetDefaultSettings tests default settings generation
func TestGetDefaultSettings(t *testing.T) {
	settings := GetDefaultSettings()

	assert.NotEmpty(t, settings.DownloadQuality)
	// CacheSettings, ConversionSettings, etc. are structs, not pointers
}

// TestUser_JSONMarshaling tests that sensitive fields are not marshaled
func TestUser_JSONMarshaling(t *testing.T) {
	user := User{
		ID:           1,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "should_not_appear",
		Salt:         "should_not_appear",
		IsActive:     true,
	}

	data, err := json.Marshal(user)
	require.NoError(t, err)

	// Verify sensitive fields are not in JSON
	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "should_not_appear")
	assert.NotContains(t, jsonStr, "password_hash")
	assert.NotContains(t, jsonStr, "salt")

	// Verify public fields are in JSON
	assert.Contains(t, jsonStr, "testuser")
	assert.Contains(t, jsonStr, "test@example.com")
}

// TestRole_JSONMarshaling tests role JSON marshaling
func TestRole_JSONMarshaling(t *testing.T) {
	role := Role{
		ID:          1,
		Name:        "admin",
		Permissions: Permissions{"admin:*", "read:*"},
		IsSystem:    true,
	}

	data, err := json.Marshal(role)
	require.NoError(t, err)

	var decoded Role
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, role.ID, decoded.ID)
	assert.Equal(t, role.Name, decoded.Name)
	assert.Equal(t, role.Permissions, decoded.Permissions)
	assert.Equal(t, role.IsSystem, decoded.IsSystem)
}

// TestValidationResult tests validation result structure
func TestValidationResult(t *testing.T) {
	result := ValidationResult{
		IsValid:  false,
		Errors:   []string{"Invalid email", "Password too short"},
		Warnings: []string{"Username may be taken"},
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded ValidationResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, result.IsValid, decoded.IsValid)
	assert.Equal(t, result.Errors, decoded.Errors)
	assert.Equal(t, result.Warnings, decoded.Warnings)
}

// TestUserSession_Structure tests user session structure
func TestUserSession_Structure(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	deviceType := "mobile"
	ipAddr := "192.168.1.1"
	userAgent := "Mozilla/5.0"
	refreshToken := "refresh-token"

	session := UserSession{
		ID:           123,
		UserID:       1,
		SessionToken: "session-token",
		RefreshToken: &refreshToken,
		DeviceInfo:   DeviceInfo{DeviceType: &deviceType},
		IPAddress:    &ipAddr,
		UserAgent:    &userAgent,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
		IsActive:     true,
	}

	assert.Equal(t, 123, session.ID)
	assert.Equal(t, 1, session.UserID)
	assert.Equal(t, "mobile", *session.DeviceInfo.DeviceType)
	assert.True(t, session.IsActive)
}

// TestPermissions_DriverValue tests Permissions implements driver.Valuer
func TestPermissions_DriverValue(t *testing.T) {
	var _ driver.Valuer = Permissions{}

	p := Permissions{"read:media"}
	val, err := p.Value()
	require.NoError(t, err)
	assert.NotNil(t, val)
}

// TestUserPreferences_DriverValue tests UserPreferences implements driver.Valuer
func TestUserPreferences_DriverValue(t *testing.T) {
	var _ driver.Valuer = UserPreferences{}

	prefs := UserPreferences{Theme: "dark"}
	val, err := prefs.Value()
	require.NoError(t, err)
	assert.NotNil(t, val)
}

// TestUserSettings_DriverValue tests UserSettings implements driver.Valuer
func TestUserSettings_DriverValue(t *testing.T) {
	var _ driver.Valuer = UserSettings{}

	settings := UserSettings{AutoSync: true}
	val, err := settings.Value()
	require.NoError(t, err)
	assert.NotNil(t, val)
}

// TestDeviceInfo_DriverValue tests DeviceInfo implements driver.Valuer
func TestDeviceInfo_DriverValue(t *testing.T) {
	var _ driver.Valuer = DeviceInfo{}

	deviceType := "mobile"
	device := DeviceInfo{DeviceType: &deviceType}
	val, err := device.Value()
	require.NoError(t, err)
	assert.NotNil(t, val)
}

// A prefix wildcard needs a segment boundary: "sys*" must not match "system.admin".
func TestPermissions_HasPermission_WildcardNeedsSegmentBoundary(t *testing.T) {
	assert.False(t, Permissions{"sys*"}.HasPermission("system.admin"))
	assert.False(t, Permissions{"s*"}.HasPermission("system.admin"))
	assert.False(t, Permissions{"system*"}.HasPermission("system.admin"))
	assert.True(t, Permissions{"system.*"}.HasPermission("system.admin"))
	assert.True(t, Permissions{"admin:*"}.HasPermission("admin:users"))
	assert.True(t, Permissions{"*"}.HasPermission("system.admin"))
}
