package filesystem

import (
	"fmt"
)

// DefaultClientFactory implements ClientFactory for all supported protocols
type DefaultClientFactory struct{}

// NewDefaultClientFactory creates a new default client factory
func NewDefaultClientFactory() *DefaultClientFactory {
	return &DefaultClientFactory{}
}

// CreateClient creates a filesystem client based on the storage configuration
func (f *DefaultClientFactory) CreateClient(config *StorageConfig) (FileSystemClient, error) {
	// PA-01: a key the protocol does not consume is an error, never a silently ignored value.
	if err := ValidateSettings(config.Protocol, config.Settings); err != nil {
		return nil, err
	}
	switch config.Protocol {
	case "smb":
		smbConfig := &SmbConfig{
			Host:     getStringSetting(config.Settings, "host", ""),
			Port:     getIntSetting(config.Settings, "port", 445),
			Share:    getStringSetting(config.Settings, "share", ""),
			Username: getStringSetting(config.Settings, "username", ""),
			Password: getStringSetting(config.Settings, "password", ""),
			Domain:   getStringSetting(config.Settings, "domain", "WORKGROUP"),
		}
		return NewSmbClient(smbConfig), nil

	case "ftp":
		ftpConfig := &FTPConfig{
			Host:     getStringSetting(config.Settings, "host", ""),
			Port:     getIntSetting(config.Settings, "port", 21),
			Username: getStringSetting(config.Settings, "username", ""),
			Password: getStringSetting(config.Settings, "password", ""),
			Path:     getStringSetting(config.Settings, "path", ""),
		}
		return NewFTPClient(ftpConfig), nil

	case "nfs":
		nfsConfig := &NFSConfig{
			Host:       getStringSetting(config.Settings, "host", ""),
			Path:       getStringSetting(config.Settings, "path", ""),
			MountPoint: getStringSetting(config.Settings, "mount_point", ""),
			Options:    getStringSetting(config.Settings, "options", "vers=3"),
		}
		client, err := NewNFSClient(*nfsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create NFS client: %w", err)
		}
		return client, nil

	case "webdav":
		webdavConfig := &WebDAVConfig{
			URL:      getStringSetting(config.Settings, "url", ""),
			Username: getStringSetting(config.Settings, "username", ""),
			Password: getStringSetting(config.Settings, "password", ""),
			Path:     getStringSetting(config.Settings, "path", ""),
		}
		return NewWebDAVClient(webdavConfig)

	case "local":
		localConfig := &LocalConfig{
			BasePath: getStringSetting(config.Settings, "base_path", ""),
		}
		return NewLocalClient(localConfig), nil

	default:
		// Pluggable protocols (sftp, ftps, nfs3, ...) registered through RegisterProtocol.
		registryMu.RLock()
		spec, ok := registered[config.Protocol]
		registryMu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
		}
		return spec.New(config.Settings)
	}
}

// SupportedProtocols returns the list of supported protocols
func (f *DefaultClientFactory) SupportedProtocols() []string {
	return append(append([]string{}, builtinOrder...), RegisteredProtocols()...)
}

// Helper functions to extract settings
func getStringSetting(settings map[string]interface{}, key, defaultValue string) string {
	if val, ok := settings[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getIntSetting(settings map[string]interface{}, key string, defaultValue int) int {
	if val, ok := settings[key]; ok {
		// ValidateSettings already refused a non-integer or out-of-range value; a caller outside the factory gets the default instead of garbage.
		if n, ok := wholeNumber(val); ok {
			return int(n)
		}
	}
	return defaultValue
}
