package handlers

// CAT-SECQ1-C (W4): the address-class approval of a storage root at create and change time.
//
//   - Create: the endpoint's host (or the host of its URL) is classified with netguard.ClassifyEndpoint. A public host is approved
//     for ClassPublic automatically (database.AutoClassifyApprovedClass, source "auto"). A private host, or one that cannot be
//     resolved now, is approved for ClassPrivate only with the operator's explicit confirmation (confirm_private_network), stored
//     with source "operator". An address no endpoint may reach (loopback, link-local, metadata, own address) is refused.
//   - Change: PUT /storage/roots/:id/approved-class. Widening (public -> private) is a step-up action: the acting administrator
//     re-presents their password. Narrowing (private -> public) is not.
//
// The stored class reaches the WebDAV and S3 clients through models.StorageRoot.ApprovedClass -> filesystem.SettingsFromRoot.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/models"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
)

// Machine-readable reasons of the 409 / 403 answers of this file.
const (
	codePrivateConfirmationRequired = "private_network_confirmation_required"
	codeStepUpRequired              = "step_up_required"
	codeStepUpFailed                = "step_up_failed"
)

// rootDestination returns the host the server will connect to for root, and whether the root has a network destination at all
// (a local root has none). WebDAV and S3 roots name a URL, the other network protocols a host.
func rootDestination(root *models.StorageRoot) (host, rawURL string, networked bool) {
	if root.Protocol == "local" {
		return "", "", false
	}
	host = strings.TrimSpace(derefString(root.Host))
	rawURL = strings.TrimSpace(derefString(root.URL))
	if root.Protocol == "webdav" || root.Protocol == "s3" {
		host = "" // the URL is the destination; ClassifyEndpoint takes its host (userinfo is not part of it)
	}
	return host, rawURL, host != "" || rawURL != ""
}

// classifyDestination is the classifier of the create path; tests replace it to hold a name's answer (rebinding test).
var classifyDestination = netguard.ClassifyEndpoint

// classifyForApproval classifies a destination for the create path. refused is true for an address no endpoint may be approved for.
// An unresolvable name is not refused and not public: it needs the same confirmation as a private one (unresolved).
func classifyForApproval(ctx context.Context, host, rawURL string) (class netguard.Class, refused bool) {
	c, err := classifyDestination(ctx, host, rawURL)
	if err == nil {
		return c, false
	}
	if errors.Is(err, netguard.ErrBlocked) {
		return netguard.ClassNever, true
	}
	return netguard.ClassPrivate, false
}

// approveNewRoot stores the class of a freshly inserted root. It classifies the destination AGAIN (the early answer of CreateStorageRoot
// is preClass): the name may have re-resolved in between, and an approval is only ever stored for what the operator was told. A destination
// that was public at the early check and is private now is refused (the operator never confirmed a private destination), as is a private one
// without confirmation. It returns the stored class, or an error after removing the row (a root must never be left behind with an approval
// the operator did not make). The row exists with the transient default (private, source default) between the insert and this call; no
// client is built from it in that window, because the root is not reported to the caller until this returns.
func (h *ScanHandler) approveNewRoot(ctx context.Context, id int64, root *models.StorageRoot, preClass netguard.Class, confirmed bool) (netguard.Class, error) {
	host, rawURL, networked := rootDestination(root)
	if !networked {
		return netguard.ClassPrivate, nil
	}
	got, cerr := classifyDestination(ctx, host, rawURL)
	if cerr != nil {
		if errors.Is(cerr, netguard.ErrBlocked) {
			h.deleteRootRow(ctx, id)
			return netguard.ClassNever, fmt.Errorf("%w: destination refused at classification", netguard.ErrBlocked)
		}
		got = netguard.ClassPrivate // not classifiable now: private, and it stands only for a confirmed root
	}
	if got == netguard.ClassPublic {
		if preClass != netguard.ClassPublic && !confirmed {
			h.deleteRootRow(ctx, id) // was private or unresolved when asked, public now: the operator saw another answer
			return netguard.ClassPublic, errPrivateUnconfirmed
		}
		if err := h.db.SetApprovedClass(ctx, database.TableStorageRoots, id, netguard.ClassPublic, database.ApprovedSourceAuto); err != nil {
			h.deleteRootRow(ctx, id)
			return netguard.ClassNever, err
		}
		return netguard.ClassPublic, nil
	}
	// private (or unresolved now)
	if !confirmed || preClass == netguard.ClassPublic {
		// unconfirmed, or the destination re-resolved from public to private after the early check: refuse, whatever the confirmation flag says
		h.deleteRootRow(ctx, id)
		return netguard.ClassPrivate, errPrivateUnconfirmed
	}
	if err := h.db.SetApprovedClass(ctx, database.TableStorageRoots, id, netguard.ClassPrivate, database.ApprovedSourceOperator); err != nil {
		h.deleteRootRow(ctx, id)
		return netguard.ClassNever, err
	}
	return netguard.ClassPrivate, nil
}

var errPrivateUnconfirmed = errors.New("private network confirmation required")

func (h *ScanHandler) deleteRootRow(ctx context.Context, id int64) {
	// the context of the request may already be done; the cleanup must still run
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, _ = h.db.ExecContext(cctx, `DELETE FROM storage_roots WHERE id = ?`, id)
}

func respondPrivateConfirmationRequired(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{
		"error": "this storage destination is on a private network (or cannot be resolved now); confirm it to approve private-network access",
		"code":  codePrivateConfirmationRequired,
		"hint":  "repeat the request with confirm_private_network=true if the destination is meant to be on your LAN",
	})
}

// contextUserID reads the authenticated user id the auth middleware stored under "user_id".
func contextUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(x)
		return n, err == nil
	}
	return 0, false
}

// verifyStepUp checks the acting user's password. Every failure looks the same to the caller.
func (h *ScanHandler) verifyStepUp(ctx context.Context, userID int, secret string) bool {
	if secret == "" {
		return false
	}
	var hash, salt string
	err := h.db.QueryRowContext(ctx, `SELECT password_hash, salt FROM users WHERE id = ?`, userID).Scan(&hash, &salt)
	if err != nil {
		return false
	}
	ok, _ := services.CheckPassword(secret, salt, hash)
	return ok
}

type setApprovedClassRequest struct {
	Class          string `json:"class" binding:"required"`
	StepUpPassword string `json:"step_up_password"`
}

// SetStorageRootApprovedClass handles PUT /api/v1/storage/roots/:id/approved-class.
// The class is enforced on WebDAV and S3 connections. For SMB, FTP and NFS roots it is stored and reported but NOT enforced yet (their
// clients are not class-pinned: SECQ1-D), so changing it on such a root has no effect on the connection.
func (h *ScanHandler) SetStorageRootApprovedClass(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid storage root id"})
		return
	}
	var req setApprovedClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadBody(c, err)
		return
	}
	want, ok := netguard.ParseClass(req.Class)
	if !ok || (want != netguard.ClassPrivate && want != netguard.ClassPublic) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "class must be private or public"})
		return
	}
	ctx := c.Request.Context()
	cur, _, err := h.db.GetApprovedClass(ctx, database.TableStorageRoots, id)
	if errors.Is(err, database.ErrEndpointNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "storage root not found"})
		return
	}
	if err != nil {
		respondInternal(c, http.StatusInternalServerError, "failed to read the approved class", err)
		return
	}
	if cur == netguard.ClassPublic && want == netguard.ClassPrivate { // widening: step-up
		uid, have := contextUserID(c)
		if !have {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if req.StepUpPassword == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "widening the approved network class needs the password of the acting administrator", "code": codeStepUpRequired})
			return
		}
		if !stepUpAttempt(c, uid, false, func() bool { return h.verifyStepUp(ctx, uid, req.StepUpPassword) }) {
			return
		}
	}
	if err := h.db.SetApprovedClass(ctx, database.TableStorageRoots, id, want, database.ApprovedSourceOperator); err != nil {
		if errors.Is(err, database.ErrEndpointNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "storage root not found"})
			return
		}
		respondInternal(c, http.StatusInternalServerError, "failed to store the approved class", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "approved_class": want.String(), "approved_class_source": database.ApprovedSourceOperator})
}
