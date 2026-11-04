package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"catalogizer/services"
	"catalogizer/utils"
	"digital.vasic.challenges/pkg/challenge"

	"github.com/gin-gonic/gin"
)

// challengeService defines the methods needed by ChallengeHandler.
type challengeService interface {
	ListChallenges() []services.ChallengeSummary
	RunChallenge(ctx context.Context, id string) (*challenge.Result, error)
	RunAll(ctx context.Context) ([]*challenge.Result, error)
	RunAllStreaming(ctx context.Context) <-chan *challenge.Result
	RunByCategory(ctx context.Context, category string) ([]*challenge.Result, error)
	RunByCategoryStreaming(ctx context.Context, category string) <-chan *challenge.Result
	GetResults() []*challenge.Result
}

// ChallengeHandler handles challenge API endpoints.
type ChallengeHandler struct {
	service challengeService
}

// NewChallengeHandler creates a new challenge handler.
func NewChallengeHandler(
	service challengeService,
) *ChallengeHandler {
	return &ChallengeHandler{service: service}
}

// ListChallenges returns all registered challenges.
func (h *ChallengeHandler) ListChallenges(c *gin.Context) {
	challenges := h.service.ListChallenges()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    challenges,
		"count":   len(challenges),
	})
}

// GetChallenge returns details of a specific challenge.
func (h *ChallengeHandler) GetChallenge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.SendErrorResponse(
			c, http.StatusBadRequest,
			"Challenge ID is required", nil,
		)
		return
	}

	challenges := h.service.ListChallenges()
	for _, ch := range challenges {
		if ch.ID == id {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    ch,
			})
			return
		}
	}

	utils.SendErrorResponse(
		c, http.StatusNotFound,
		"Challenge not found", nil,
	)
}

// RunChallenge executes a single challenge by ID.
//
// FIX-QA-2026-04-21-005 (partial closure of DEFER-QA-2026-04-21-001):
// single-challenge runs now propagate c.Request.Context() instead of
// context.Background() so the challenge terminates when the HTTP
// client disconnects (curl timeout, closed TCP). This keeps the
// concurrency limiter from accumulating zombie handlers — the exact
// pathology observed in the 2026-04-20 RunAll session.
//
// RunAll/RunByCategory deliberately keep context.Background() because
// they're long-running by design; their full ctx-threading still
// requires the submodule refactor documented in DEFER-001.
func (h *ChallengeHandler) RunChallenge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.SendErrorResponse(
			c, http.StatusBadRequest,
			"Challenge ID is required", nil,
		)
		return
	}

	result, err := h.service.RunChallenge(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			utils.SendErrorResponse(c, http.StatusNotFound, "Challenge not found", err)
			return
		}
		utils.SendErrorResponse(
			c, http.StatusInternalServerError,
			"Failed to run challenge", err,
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// RunAll executes all registered challenges in dependency order.
//
// FIX-QA-2026-04-21-008 (partial closure of DEFER-QA-2026-04-21-001 #3):
// uses `context.WithoutCancel(c.Request.Context())` instead of the
// previous `context.Background()`. That gives the service a ctx that:
//   - **inherits** the request's values (tracing, request_id, auth
//     subject, etc.) so downstream logging stays threaded,
//   - **does not cancel** when the outer `RequestTimeout(60*time.Second)`
//     middleware or client disconnect fires, because RunAll routinely
//     runs for 10+ minutes by design.
//
// Streaming mode (query param `stream=true`) writes each challenge
// result as a JSON line (NDJSON) as it completes, avoiding the
// tens-of-MB buffer that blocks read endpoints on 508-challenge runs.
//
// This is the "long-running but still traceable" contract documented
// in the DEFER-001 ticket. Full ctx-threading through the Challenges
// submodule runner so individual challenge steps can co-operatively
// observe progress is still pending (DEFER-001 #4).
func (h *ChallengeHandler) RunAll(c *gin.Context) {
	ctx := context.WithoutCancel(c.Request.Context())

	// Streaming mode yields results as they complete instead of
	// buffering the entire payload.
	if c.Query("stream") == "true" {
		c.Header("Content-Type", "application/x-ndjson")
		c.Header("Transfer-Encoding", "chunked")
		c.Status(http.StatusOK)

		enc := json.NewEncoder(c.Writer)
		passed, failed := 0, 0
		for result := range h.service.RunAllStreaming(ctx) {
			if err := enc.Encode(result); err != nil {
				break
			}
			c.Writer.Flush()
			if result.Status == challenge.StatusPassed {
				passed++
			} else {
				failed++
			}
		}
		return
	}

	results, err := h.service.RunAll(ctx)
	if err != nil {
		utils.SendErrorResponse(
			c, http.StatusInternalServerError,
			"Failed to run challenges", err,
		)
		return
	}

	passed := 0
	failed := 0
	for _, r := range results {
		if r.Status == "passed" {
			passed++
		} else {
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
		"summary": gin.H{
			"total":  len(results),
			"passed": passed,
			"failed": failed,
		},
	})
}

// RunByCategory executes all challenges in a category.
func (h *ChallengeHandler) RunByCategory(c *gin.Context) {
	category := c.Param("category")
	if category == "" {
		utils.SendErrorResponse(
			c, http.StatusBadRequest,
			"Category is required", nil,
		)
		return
	}

	// FIX-QA-2026-04-21-008: see RunAll comment — detach from request
	// lifetime without losing request-scoped values.
	ctx := context.WithoutCancel(c.Request.Context())

	// Streaming mode yields results as they complete.
	if c.Query("stream") == "true" {
		c.Header("Content-Type", "application/x-ndjson")
		c.Header("Transfer-Encoding", "chunked")
		c.Status(http.StatusOK)

		enc := json.NewEncoder(c.Writer)
		passed, failed := 0, 0
		for result := range h.service.RunByCategoryStreaming(ctx, category) {
			if err := enc.Encode(result); err != nil {
				break
			}
			c.Writer.Flush()
			if result.Status == challenge.StatusPassed {
				passed++
			} else {
				failed++
			}
		}
		return
	}

	results, err := h.service.RunByCategory(ctx, category)
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			utils.SendErrorResponse(c, http.StatusNotFound, "Challenge category not found", err)
			return
		}
		utils.SendErrorResponse(
			c, http.StatusInternalServerError,
			"Failed to run challenges", err,
		)
		return
	}

	passed := 0
	failed := 0
	for _, r := range results {
		if r.Status == "passed" {
			passed++
		} else {
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
		"summary": gin.H{
			"total":  len(results),
			"passed": passed,
			"failed": failed,
		},
	})
}

// GetResults returns stored challenge execution results.
//
// FIX-QA-2026-04-21-004 (partial mitigation of DEFER-QA-2026-04-21-001):
// after a 508-challenge RunAll the full result slice carries tens of MB
// of embedded assertion + output data; serialising all of it through
// gin's JSON encoder can sit well past the 60s RequestTimeout and
// present as a "hung" endpoint. Accept an optional `limit` query param
// (default: last 100 results; 0 = unlimited) so diagnostic callers get
// a fast answer without starving the server on huge post-RunAll
// payloads.
func (h *ChallengeHandler) GetResults(c *gin.Context) {
	all := h.service.GetResults()

	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			limit = n
		}
	}

	results := all
	if limit > 0 && len(all) > limit {
		results = all[len(all)-limit:]
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"data":        results,
		"count":       len(results),
		"total_count": len(all),
	})
}
