package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"catalogizer/internal/pathsafe"
	"catalogizer/smb"

	"github.com/hirochachacha/go-smb2"
	"go.uber.org/zap"
)

// SMBShareInfo represents an SMB share
type SMBShareInfo struct {
	Host        string  `json:"host"`
	ShareName   string  `json:"share_name"`
	Path        string  `json:"path"`
	Writable    bool    `json:"writable"`
	Description *string `json:"description"`
}

// SMBFileEntry represents a file or directory in an SMB share
type SMBFileEntry struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	IsDirectory bool    `json:"is_directory"`
	Size        *int64  `json:"size"`
	Modified    *string `json:"modified"`
}

// SMBConnectionConfig represents SMB connection parameters
type SMBConnectionConfig struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Share    string  `json:"share"`
	Username string  `json:"username"`
	Password string  `json:"password"`
	Domain   *string `json:"domain"`
}

// SMBDiscoveryService provides SMB share discovery and testing
type SMBDiscoveryService struct {
	logger  *zap.Logger
	timeout time.Duration
	// openShare is a unit-class seam: it replaces only dial + session + mount. Everything above it
	// (validation, SplitShare, confinement) is the real code. nil means the real SMB network path.
	openShare func(ctx context.Context, cfg SMBConnectionConfig, shareName string) (ShareAPI, func(), error)
}

// ErrInvalidSMBInput marks a request the caller got wrong (admin or malformed share, parent-directory path, port other than 445). Handlers
// map it to 400; every other error from this service is an operational failure and carries no server or network detail.
var ErrInvalidSMBInput = errors.New("invalid SMB request")

// smbPort is the only SMB-over-TCP port the discovery routes may dial (a free port choice would make them a port scanner).
const smbPort = 445

// ValidateSMBPort accepts 0 (default 445) and any port in SMBAllowedPorts; everything else is refused.
func ValidateSMBPort(port int) error {
	if port == 0 {
		return nil
	}
	for _, p := range SMBAllowedPorts() {
		if p == port {
			return nil
		}
	}
	return fmt.Errorf("%w: port not allowed", ErrInvalidSMBInput)
}

// openConfined validates the request-supplied port and share, mounts the SHARE part only (smb.SplitShare: refuses IPC$, C$, ADMIN$, device
// names, "..", backslashes) and returns the share confined to the sub-directory of a "share/sub/dir" spec, exactly like SMBService.withShare.
func (s *SMBDiscoveryService) openConfined(ctx context.Context, config SMBConnectionConfig) (ShareAPI, func(), error) {
	if err := ValidateSMBHostSyntax(config.Host); err != nil {
		return nil, nil, err
	}
	if err := ValidateSMBPort(config.Port); err != nil {
		return nil, nil, err
	}
	if config.Port == 0 {
		config.Port = smbPort
	}
	shareName, sub, err := smb.SplitShare(config.Share)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidSMBInput, err)
	}
	raw, release, err := s.mountShare(ctx, config, shareName)
	if err != nil {
		return nil, nil, err
	}
	if sub == "" {
		return raw, release, nil
	}
	return confinedShare{in: raw, sub: sub}, release, nil
}

// NewSMBDiscoveryService creates a new SMB discovery service
func NewSMBDiscoveryService(logger *zap.Logger) *SMBDiscoveryService {
	return &SMBDiscoveryService{
		logger:  logger,
		timeout: 10 * time.Second,
	}
}

// DiscoverShares discovers available SMB shares on a host
func (s *SMBDiscoveryService) DiscoverShares(ctx context.Context, host string, username, password string, domain *string) ([]SMBShareInfo, error) {
	s.logger.Info("Discovering SMB shares", zap.String("host", host), zap.String("username", username))

	ips, err := VetSMBHostIPs(host)
	if err != nil {
		return nil, err
	}
	// Establish connection to a vetted address (no second lookup of the name, SMB-FU-9)
	conn, err := dialVetted(ctx, ips, smbPort, s.timeout)
	if err != nil {
		return nil, fmt.Errorf("connect to %s:445: %w", host, err)
	}
	defer conn.Close()

	// Create SMB session
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username,
			Password: password,
			Domain:   getStringValue(domain),
		},
	}

	session, err := d.Dial(conn)
	if err != nil {
		return nil, fmt.Errorf("SMB session on %s: %w", host, err)
	}
	defer session.Logoff()

	return s.enumerateShares(session, host)
}

// enumerateShares enumerates the REAL shares exported by the host via SRVSVC
// (go-smb2 ListSharenames), not a guessed list of common names.
//
// §11.4.6 (no-guessing): the previous implementation iterated a hardcoded list
// ("shared", "public", "media", "music", "data", ...) and reported only those
// that happened to mount. That MISSED any share whose name was not in the guess
// list. Verified FACT against a real Synology NAS (2026-06-26): the actual
// exported shares are names like "Data", "DATA18", "DATA12", "DATA20", "WORK20",
// "Projects" — none of which the guess list contained, so the old code returned
// an empty/partial inventory while the shares plainly existed. ListSharenames
// returns the authoritative set the server actually advertises.
func (s *SMBDiscoveryService) enumerateShares(session *smb2.Session, host string) ([]SMBShareInfo, error) {
	names, err := session.ListSharenames()
	if err != nil {
		return nil, fmt.Errorf("enumerate shares on %s: %w", host, err)
	}

	availableShares := filterShareInfos(host, names)
	return availableShares, nil
}

// testShareAccess tests if a share can be accessed
func (s *SMBDiscoveryService) testShareAccess(session *smb2.Session, shareName string) bool {
	share, err := session.Mount(shareName)
	if err != nil {
		return false
	}
	defer share.Umount()

	// Try to list the root directory
	_, err = share.ReadDir(".")
	return err == nil
}

// TestConnection tests an SMB connection with the provided credentials
func (s *SMBDiscoveryService) TestConnection(ctx context.Context, config SMBConnectionConfig) bool {
	s.logger.Info("Testing SMB connection",
		zap.String("host", config.Host),
		zap.String("share", config.Share),
		zap.String("username", config.Username))

	share, release, err := s.openConfined(ctx, config)
	if err != nil {
		s.logger.Debug("Failed to open SMB share", zap.String("host", config.Host), zap.String("share", config.Share), zap.Error(err))
		return false
	}
	defer release()

	// Try to list the root directory
	if _, err = share.ReadDir("."); err != nil {
		s.logger.Debug("Failed to read SMB share directory", zap.String("share", config.Share), zap.Error(err))
		return false
	}

	s.logger.Info("SMB connection test successful", zap.String("host", config.Host), zap.String("share", config.Share))
	return true
}

// BrowseShare browses files and directories in an SMB share
func (s *SMBDiscoveryService) BrowseShare(ctx context.Context, config SMBConnectionConfig, path string) ([]SMBFileEntry, error) {
	s.logger.Info("Browsing SMB share",
		zap.String("host", config.Host),
		zap.String("share", config.Share),
		zap.String("path", path))

	if err := pathsafe.SafeRemoteSharePath(path); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSMBInput, err)
	}
	share, release, err := s.openConfined(ctx, config)
	if err != nil {
		if errors.Is(err, ErrInvalidSMBInput) {
			return nil, err
		}
		s.logger.Debug("Failed to open SMB share", zap.String("host", config.Host), zap.Error(err))
		return nil, errors.New("failed to mount SMB share")
	}
	defer release()

	// List directory contents (confined below the sub-directory of a share/sub root)
	entries, err := share.ReadDir(path)
	if err != nil {
		s.logger.Debug("Failed to read SMB directory", zap.String("host", config.Host), zap.Error(err))
		return nil, errors.New("failed to read directory")
	}

	// Convert to our format
	fileEntries := []SMBFileEntry{}
	for _, entry := range entries {
		var size *int64
		if !entry.IsDir() {
			entrySize := entry.Size()
			size = &entrySize
		}

		modTime := entry.ModTime().Format("2006-01-02 15:04:05")

		fileEntries = append(fileEntries, SMBFileEntry{
			Name:        entry.Name(),
			Path:        path + "/" + entry.Name(),
			IsDirectory: entry.IsDir(),
			Size:        size,
			Modified:    &modTime,
		})
	}

	return fileEntries, nil
}

// mountShare dials the host, opens a session and mounts shareName, returning the share and a release func.
func (s *SMBDiscoveryService) mountShare(ctx context.Context, config SMBConnectionConfig, shareName string) (ShareAPI, func(), error) {
	if s.openShare != nil {
		return s.openShare(ctx, config, shareName)
	}
	ips, err := VetSMBHostIPs(config.Host)
	if err != nil {
		return nil, nil, err
	}
	conn, err := dialVetted(ctx, ips, config.Port, s.timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SMB host %s: %w", config.Host, err)
	}
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     config.Username,
			Password: config.Password,
			Domain:   getStringValue(config.Domain),
		},
	}
	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create SMB session: %w", err)
	}
	sh, err := session.Mount(shareName)
	if err != nil {
		session.Logoff()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to mount SMB share: %w", err)
	}
	return smb2ShareAPI{sh}, func() { sh.Umount(); session.Logoff(); conn.Close() }, nil
}

// Helper functions
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getShareDescription(shareName string) *string {
	descriptions := map[string]string{
		"C$":        "System drive (administrative)",
		"D$":        "Data drive (administrative)",
		"E$":        "Additional drive (administrative)",
		"F$":        "Additional drive (administrative)",
		"admin$":    "Administrative share",
		"print$":    "Printer drivers",
		"ipc$":      "Inter-process communication",
		"shared":    "Shared folder",
		"public":    "Public folder",
		"media":     "Media files",
		"downloads": "Downloads",
		"documents": "Documents",
		"music":     "Music files",
		"videos":    "Video files",
		"pictures":  "Pictures",
		"backup":    "Backup files",
		"data":      "Data files",
	}

	if desc, exists := descriptions[strings.ToLower(shareName)]; exists {
		return &desc
	}
	return nil
}
