package handlers

import (
	"context"
	"net/http"
	"time"

	"catalogizer/database"
	root_handlers "catalogizer/handlers"
	"catalogizer/internal/netguard"
	"catalogizer/internal/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const hostAllowedErr = "Host not allowed"

// ProbeAndIngestRequest represents the request to probe a host and ingest results.
// Only host is required — identities are loaded from environment variables
// (CATALOGIZER_IDENTITY_*) so no credentials ever travel over the wire (§11.4.10).
type ProbeAndIngestRequest struct {
	Host string `json:"host" binding:"required"`
	// ConfirmPrivateNetwork is the operator's confirmation that a host on a private network (or one that cannot be resolved now) is meant to be
	// reached and stored as a storage root; without it such a host is refused with 400 before anything is probed or stored (CAT-CPN-1).
	ConfirmPrivateNetwork bool `json:"confirm_private_network"`
}

// ProbeAndIngestResponse is the JSON response shape returned to the caller.
// It carries only the host, whether any identity authenticated, the label
// (username or "guest", §11.4.10 — never a password), share counts, and
// ingestion counts. The identity_label is present ONLY when Authenticated=true.
type ProbeAndIngestResponse struct {
	Host          string `json:"host"`
	Authenticated bool   `json:"authenticated"`
	IdentityLabel string `json:"identity_label,omitempty"`
	Shares        int    `json:"shares,omitempty"`
	BoundShares   int    `json:"bound_shares,omitempty"`
	NewRoots      int    `json:"new_roots,omitempty"`
	Message       string `json:"message,omitempty"`
	// Partial is set on a persistence error when some shares were already committed (CPN1R-m4); BoundShares/NewRoots then count them.
	Partial bool `json:"partial,omitempty"`
}

// ProbeAndIngestHandler handles SMB probe-and-ingest API requests.
// It composes an SMBDiscoveryService (for probing) with a BindingIngester
// (for persisting the discovered (host, identity_index, share) bindings).
type ProbeAndIngestHandler struct {
	discoveryService *services.SMBDiscoveryService
	bindingIngester  *services.BindingIngester
	logger           *zap.Logger
}

// NewProbeAndIngestHandler creates a new ProbeAndIngestHandler.
func NewProbeAndIngestHandler(
	discoveryService *services.SMBDiscoveryService,
	bindingIngester *services.BindingIngester,
	logger *zap.Logger,
) *ProbeAndIngestHandler {
	return &ProbeAndIngestHandler{
		discoveryService: discoveryService,
		bindingIngester:  bindingIngester,
		logger:           logger,
	}
}

// ProbeAndIngest probes an SMB host with every configured identity (loaded from
// environment variables via LoadSMBIdentitiesFromEnv), and if one authenticates,
// ingests the discovered shares as storage_root + share_identity_binding records.
//
// SECURITY (§11.4.10): identities are loaded from environment variables on the
// SERVER side — the HTTP request body carries ONLY the host name, NEVER
// credentials. The response includes identity_label (a username or "guest") to
// indicate which identity authenticated, but NEVER a password or other secret.
//
//	POST /api/v1/smb/probe-and-ingest
//	{"host": "nas.example.com", "confirm_private_network": true}
//
// @Summary Probe host and ingest bindings
// @Description Probes an SMB host with all configured identities and persists the first working (host, identity, shares) binding
// @Tags SMB
// @Accept json
// @Produce json
// @Param request body ProbeAndIngestRequest true "Probe request (host only — identities loaded from server env)"
// @Success 200 {object} ProbeAndIngestResponse
// @Failure 400 {object} map[string]interface{} "invalid request, disallowed host, or a private-network host without confirm_private_network"
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/smb/probe-and-ingest [post]
func (h *ProbeAndIngestHandler) ProbeAndIngest(c *gin.Context) {
	var req ProbeAndIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid probe-and-ingest request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// CPN1-M2: resolve and vet the host ONCE; classify exactly those addresses and dial exactly those addresses. A second lookup (the old
	// isHostAllowed + classifier + probe each resolved on their own) lets a name that was public when classified be private when dialled.
	ips, vetErr := services.VetSMBHostIPs(req.Host)
	if vetErr != nil {
		h.logger.Warn("Blocked probe to disallowed host", zap.String("host", req.Host))
		c.JSON(http.StatusBadRequest, gin.H{"error": hostAllowedErr})
		return
	}

	// CAT-CPN-1: this route stores storage roots, so it applies the same private-LAN rule as POST /storage/roots, before any probe.
	// The class of the host is the widest class of its vetted addresses (IP literals: the classifier does not look anything up again).
	class := netguard.ClassPublic
	cctx, ccancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	for _, ip := range ips {
		ipClass, refused := root_handlers.ClassifyStorageDestination(cctx, ip.String(), "")
		if refused {
			ccancel()
			c.JSON(http.StatusBadRequest, gin.H{"error": hostAllowedErr})
			return
		}
		if ipClass != netguard.ClassPublic {
			class = netguard.ClassPrivate
		}
	}
	ccancel()
	if class != netguard.ClassPublic && !req.ConfirmPrivateNetwork {
		root_handlers.RespondPrivateConfirmationRequired(c)
		return
	}
	// CPN1-M1: the stored approval is what the operator was told: public/auto for a public host, private/operator for a confirmed private one.
	assignment := services.ApprovedClassAssignment{Class: netguard.ClassPublic, Source: database.ApprovedSourceAuto}
	if class != netguard.ClassPublic {
		assignment = services.ApprovedClassAssignment{Class: netguard.ClassPrivate, Source: database.ApprovedSourceOperator}
	}

	h.logger.Info("Probing SMB host with configured identities",
		zap.String("host", req.Host))

	identities := services.LoadSMBIdentitiesFromEnv()
	h.logger.Debug("Loaded SMB identities from environment",
		zap.Int("count", len(identities)))

	result, err := h.discoveryService.ProbeVettedIPsWithIdentities(c.Request.Context(), req.Host, ips, identities)
	if err != nil {
		h.logger.Error("SMB probe failed",
			zap.String("host", req.Host),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "SMB probe failed",
		})
		return
	}

	resp := ProbeAndIngestResponse{
		Host:          result.Host,
		Authenticated: result.Authenticated,
	}

	if result.Authenticated {
		resp.IdentityLabel = result.IdentityLabel
		resp.Shares = len(result.Shares)

		ingestResult, ingestErr := h.bindingIngester.IngestProbeResultWithClass(c.Request.Context(), result, assignment)
		if ingestErr != nil {
			h.logger.Error("Failed to ingest probe result",
				zap.String("host", req.Host),
				zap.Error(ingestErr))
			body := gin.H{"error": "Failed to persist bindings"}
			if ingestResult != nil && (ingestResult.BoundShares > 0 || ingestResult.NewRoots > 0) {
				body["partial"] = true
				body["bound_shares"] = ingestResult.BoundShares
				body["new_roots"] = ingestResult.NewRoots
			}
			c.JSON(http.StatusInternalServerError, body)
			return
		}

		resp.BoundShares = ingestResult.BoundShares
		resp.NewRoots = ingestResult.NewRoots
		resp.Message = "Shares discovered and bindings persisted"
	} else {
		resp.Message = "No identity could authenticate against the host"
	}

	c.JSON(http.StatusOK, resp)
}
