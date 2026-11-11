package handlers

import (
	"catalogizer/internal/pathsafe"
	"catalogizer/internal/services"
	"catalogizer/smb"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SMBDiscoveryHandler handles SMB discovery API requests
type SMBDiscoveryHandler struct {
	service *services.SMBDiscoveryService
	logger  *zap.Logger
}

// ListIdentityInfo is a safe (no-secrets) representation of an SMB identity
// returned by ListIdentities. Never exposes passwords or any secret material (§11.4.10).
type ListIdentityInfo struct {
	Index    int    `json:"index"`
	Kind     string `json:"kind"`
	Username string `json:"username"`
	Label    string `json:"label"`
}

// ProbeHostRequest represents the request to probe a host with identity-based auth
type ProbeHostRequest struct {
	Host string `json:"host" binding:"required"`
}

// NewSMBDiscoveryHandler creates a new SMB discovery handler
func NewSMBDiscoveryHandler(service *services.SMBDiscoveryService, logger *zap.Logger) *SMBDiscoveryHandler {
	return &SMBDiscoveryHandler{
		service: service,
		logger:  logger,
	}
}

// DiscoverSharesRequest represents the request to discover SMB shares
type DiscoverSharesRequest struct {
	Host     string  `json:"host" binding:"required"`
	Username string  `json:"username" binding:"required"`
	Password string  `json:"password" binding:"required"`
	Domain   *string `json:"domain"`
}

// TestConnectionRequest represents the request to test SMB connection
type TestConnectionRequest struct {
	Host     string  `json:"host" binding:"required"`
	Port     int     `json:"port"`
	Share    string  `json:"share" binding:"required"`
	Username string  `json:"username" binding:"required"`
	Password string  `json:"password" binding:"required"`
	Domain   *string `json:"domain"`
}

// BrowseShareRequest represents the request to browse SMB share
type BrowseShareRequest struct {
	Host     string  `json:"host" binding:"required"`
	Port     int     `json:"port"`
	Share    string  `json:"share" binding:"required"`
	Username string  `json:"username" binding:"required"`
	Password string  `json:"password" binding:"required"`
	Domain   *string `json:"domain"`
	Path     string  `json:"path"`
}

// DiscoverShares discovers available SMB shares on a host
// @Summary Discover SMB shares
// @Description Discovers available SMB shares on the specified host
// @Tags SMB
// @Accept json
// @Produce json
// @Param request body DiscoverSharesRequest true "Discovery request"
// @Success 200 {array} services.SMBShareInfo
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/smb/discover [post]
func (h *SMBDiscoveryHandler) DiscoverShares(c *gin.Context) {
	var req DiscoverSharesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if !isHostAllowed(req.Host) {
		h.logger.Warn("Blocked discovery of disallowed host", zap.String("host", req.Host))
		c.JSON(http.StatusBadRequest, gin.H{"error": hostAllowedErr})
		return
	}

	h.logger.Info("Discovering SMB shares", zap.String("host", req.Host), zap.String("username", req.Username))

	shares, err := h.service.DiscoverShares(c.Request.Context(), req.Host, req.Username, req.Password, req.Domain)
	if err != nil {
		h.logger.Error("Failed to discover SMB shares", zap.Error(err))
		st, msg := smbErrorStatus(err, "Failed to discover shares")
		c.JSON(st, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, shares)
}

// TestConnection tests an SMB connection
// @Summary Test SMB connection
// @Description Tests connectivity to an SMB share with the provided credentials
// @Tags SMB
// @Accept json
// @Produce json
// @Param request body TestConnectionRequest true "Connection test request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/smb/test [post]
func (h *SMBDiscoveryHandler) TestConnection(c *gin.Context) {
	var req TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Set default port if not provided
	if req.Port == 0 {
		req.Port = 445
	}

	if msg := validateSMBTarget(req.Host, req.Port, req.Share, "."); msg != "" {
		h.logger.Warn("Rejected SMB connection test", zap.String("host", req.Host), zap.String("reason", msg))
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	h.logger.Info("Testing SMB connection", zap.String("host", req.Host), zap.String("share", req.Share))

	config := services.SMBConnectionConfig{
		Host:     req.Host,
		Port:     req.Port,
		Share:    req.Share,
		Username: req.Username,
		Password: req.Password,
		Domain:   req.Domain,
	}

	success := h.service.TestConnection(c.Request.Context(), config)

	c.JSON(http.StatusOK, gin.H{
		"success":    success,
		"host":       req.Host,
		"share":      req.Share,
		"username":   req.Username,
		"connection": success,
	})
}

// BrowseShare browses files and directories in an SMB share
// @Summary Browse SMB share
// @Description Lists files and directories in the specified SMB share path
// @Tags SMB
// @Accept json
// @Produce json
// @Param request body BrowseShareRequest true "Browse request"
// @Success 200 {array} services.SMBFileEntry
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/smb/browse [post]
func (h *SMBDiscoveryHandler) BrowseShare(c *gin.Context) {
	var req BrowseShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Set default port if not provided
	if req.Port == 0 {
		req.Port = 445
	}

	// Set default path if not provided
	if req.Path == "" {
		req.Path = "."
	}

	if msg := validateSMBTarget(req.Host, req.Port, req.Share, req.Path); msg != "" {
		h.logger.Warn("Rejected SMB browse", zap.String("host", req.Host), zap.String("reason", msg))
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	h.logger.Info("Browsing SMB share", zap.String("host", req.Host), zap.String("share", req.Share), zap.String("path", req.Path))

	config := services.SMBConnectionConfig{
		Host:     req.Host,
		Port:     req.Port,
		Share:    req.Share,
		Username: req.Username,
		Password: req.Password,
		Domain:   req.Domain,
	}

	entries, err := h.service.BrowseShare(c.Request.Context(), config, req.Path)
	if err != nil {
		h.logger.Error("Failed to browse SMB share", zap.Error(err))
		st, msg := smbErrorStatus(err, "Failed to browse share")
		c.JSON(st, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, entries)
}

// DiscoverSharesGET is the retired credential-in-URL form of share discovery (SECQ-5). Credentials in a query string
// leak into access logs, proxies and browser history, so the handler answers 410 Gone WITHOUT reading any parameter.
// @Summary Discover SMB shares (GET, retired)
// @Description Retired: credentials in a URL are refused. Use POST /api/v1/smb/discover with a JSON body.
// @Tags SMB
// @Produce json
// @Failure 410 {object} map[string]interface{}
// @Router /api/v1/smb/discover [get]
// ListIdentities returns the safe (no-secrets) identities loaded from the environment.
// @Summary List SMB identities
// @Description Returns the configured SMB identities (labels only, NEVER secrets)
// @Tags SMB
// @Produce json
// @Success 200 {array} ListIdentityInfo
// @Router /api/v1/smb/identities [get]
func (h *SMBDiscoveryHandler) ListIdentities(c *gin.Context) {
	idents := services.LoadSMBIdentitiesFromEnv()
	out := make([]ListIdentityInfo, 0, len(idents))
	for _, id := range idents {
		out = append(out, ListIdentityInfo{
			Index:    id.Index,
			Kind:     id.Kind,
			Username: id.Username,
			Label:    id.Label(),
		})
	}
	h.logger.Debug("Listed SMB identities", zap.Int("count", len(out)))
	c.JSON(http.StatusOK, out)
}

// ProbeHost probes a host using the identity-based probe (guest first, then each
// configured identity), returning the binding identity and real shares.
// @Summary Probe SMB host with identity-based auth
// @Description Probes a host (guest first, then each identity) and returns the binding identity + real shares
// @Tags SMB
// @Accept json
// @Produce json
// @Param request body ProbeHostRequest true "Probe request"
// @Success 200 {object} services.SMBProbeResult
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/smb/probe [post]
func (h *SMBDiscoveryHandler) ProbeHost(c *gin.Context) {
	var req ProbeHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid probe request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if !isHostAllowed(req.Host) {
		h.logger.Warn("Blocked probe to disallowed host", zap.String("host", req.Host))
		c.JSON(http.StatusBadRequest, gin.H{"error": hostAllowedErr})
		return
	}

	idents := services.LoadSMBIdentitiesFromEnv()
	h.logger.Info("Probing SMB host with identities",
		zap.String("host", req.Host),
		zap.Int("identity_count", len(idents)))

	result, err := h.service.ProbeHostWithIdentities(c.Request.Context(), req.Host, idents)
	if err != nil {
		h.logger.Error("Failed to probe SMB host", zap.Error(err))
		st, msg := smbErrorStatus(err, "Failed to probe host")
		c.JSON(st, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *SMBDiscoveryHandler) DiscoverSharesGET(c *gin.Context) {
	smbCredentialsInURLGone(c)
}

// TestConnectionGET is the retired credential-in-URL form of the connection test (SECQ-5); see DiscoverSharesGET.
// @Summary Test SMB connection (GET, retired)
// @Description Retired: credentials in a URL are refused. Use POST /api/v1/smb/test with a JSON body.
// @Tags SMB
// @Produce json
// @Failure 410 {object} map[string]interface{}
// @Router /api/v1/smb/test [get]
func (h *SMBDiscoveryHandler) TestConnectionGET(c *gin.Context) {
	smbCredentialsInURLGone(c)
}

// smbCredentialsInURLGone answers the retired GET forms. It deliberately never reads c.Query: the secret is not parsed,
// logged or echoed.
func smbCredentialsInURLGone(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error": "credentials in the URL are no longer accepted; use POST " + c.Request.URL.Path + " with a JSON body",
	})
}

// validateSMBTarget applies to the discovery routes the same SSRF host check ProbeHost uses, plus the port and share/path rules of the
// service (SMB-FU-2). It returns "" when the request may proceed, else a fixed client-safe message.
func validateSMBTarget(host string, port int, share, path string) string {
	if !isHostAllowed(host) {
		return hostAllowedErr
	}
	if err := services.ValidateSMBPort(port); err != nil {
		return "Unsupported port"
	}
	if _, _, err := smb.SplitShare(share); err != nil {
		return "Invalid share"
	}
	if err := pathsafe.SafeRemotePath(path); err != nil {
		return "Invalid path"
	}
	return ""
}

// smbErrorStatus maps a service error to an HTTP status and a fixed client-safe message: client input errors are 400, everything else
// is 500 with the generic text (no server, network or path detail).
func smbErrorStatus(err error, generic string) (int, string) {
	if errors.Is(err, services.ErrInvalidSMBInput) {
		return http.StatusBadRequest, "Invalid share, path, host or port"
	}
	return http.StatusInternalServerError, generic
}
