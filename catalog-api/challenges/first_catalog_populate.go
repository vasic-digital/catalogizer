package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/httpclient"
)

// FirstCatalogPopulateChallenge triggers the full scan pipeline
// via the REST API (just like an end user would): creates a storage
// root, queues scans for each configured content directory, polls
// until completion, and verifies that the catalog database is populated.
type FirstCatalogPopulateChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewFirstCatalogPopulateChallenge creates CH-008.
func NewFirstCatalogPopulateChallenge() *FirstCatalogPopulateChallenge {
	return &FirstCatalogPopulateChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"first-catalog-populate",
			"Populate Catalog Database",
			"Creates a storage root via the API, triggers scans for each "+
				"configured content directory, polls until completion, and "+
				"verifies the catalog database contains files with non-zero "+
				"total_files and total_size",
			"e2e",
			allFirstCatalogDeps,
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the populate challenge.
func (c *FirstCatalogPopulateChallenge) Execute(ctx context.Context) (*challenge.Result, error) {
	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{
		"api_url": c.config.BaseURL,
	}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	// Step 1: Login
	_, loginErr := client.Login(ctx, c.config.Username, c.config.Password)
	loginOK := loginErr == nil
	assertions = append(assertions, challenge.AssertionResult{
		Type:    "not_empty",
		Target:  "login",
		Passed:  loginOK,
		Message: challenge.Ternary(loginOK, "Admin login succeeded", fmt.Sprintf("Login failed: %v", loginErr)),
	})
	if !loginOK {
		challengeResult := c.CreateResult(challenge.StatusFailed, start, assertions, nil, outputs, fmt.Sprintf("login failed: %v", loginErr))
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	// Step 2: Load endpoint config to get NAS details and content directories
	epCfg, epErr := LoadEndpointConfig(DefaultConfigPath())
	if epErr != nil {
		assertions = append(assertions, challenge.AssertionResult{
			Type:    "not_empty",
			Target:  "endpoint_config",
			Passed:  false,
			Message: fmt.Sprintf("Failed to load endpoint config: %v", epErr),
		})
		challengeResult := c.CreateResult(challenge.StatusFailed, start, assertions, nil, outputs, fmt.Sprintf("endpoint config: %v", epErr))
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("endpoint config: %v", epErr)))
		return challengeResult, nil
	}
	if len(epCfg.Endpoints) == 0 {
		assertions = append(assertions, challenge.AssertionResult{
			Type:    "not_empty",
			Target:  "endpoint_config",
			Passed:  false,
			Message: "No endpoints configured",
		})
		challengeResult := c.CreateResult(challenge.StatusFailed, start, assertions, nil, outputs, "no endpoints configured")
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), "no endpoints configured"))
		return challengeResult, nil
	}

	ep := epCfg.Endpoints[0]
	rootName := fmt.Sprintf("%s-%s", ep.Name, ep.Share)
	outputs["storage_root_name"] = rootName
	outputs["smb_host"] = ep.Host
	outputs["smb_share"] = ep.Share
	outputs["content_directories"] = fmt.Sprintf("%d", len(ep.Directories))

	if len(ep.Directories) == 0 {
		assertions = append(assertions, challenge.AssertionResult{
			Type:    "not_empty",
			Target:  "content_directories",
			Passed:  false,
			Message: "No content directories configured in endpoint",
		})
		challengeResult := c.CreateResult(challenge.StatusFailed, start, assertions, nil, outputs, "no content directories configured")
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), "no content directories configured"))
		return challengeResult, nil
	}

	// Pre-check: verify NAS endpoint is reachable.
	if !isEndpointReachable(ep.Host, ep.Port) {
		assertions = append(assertions, challenge.AssertionResult{
			Type:    "infrastructure",
			Target:  "nas_reachable",
			Passed:  false,
			Message: fmt.Sprintf("NAS at %s:%d not reachable - skipped (requires NAS infrastructure)", ep.Host, ep.Port),
		})
		challengeResult := c.CreateResult(challenge.StatusSkipped, start, assertions, nil, outputs, "")
		challengeResult.RecordAction(fmt.Sprintf("%s: failed", c.Name()))
		return challengeResult, nil
	}

	// Step 3: POST /api/v1/storage/roots — create SMB storage root. The NAS is on the operator's LAN, so the request carries the operator's
	// confirmation (CAT-CPN-1); it is harmless for a public destination.
	createBody := createStorageRootBody(rootName, ep)
	createCode, createRaw, createErr := client.PostJSON(ctx, "/api/v1/storage/roots", createBody)
	createOK := createErr == nil && (createCode == 200 || createCode == 201)
	var storageRootID float64
	if createOK && createRaw != nil {
		var resp map[string]interface{}
		if json.Unmarshal(createRaw, &resp) == nil {
			if id, ok := resp["id"].(float64); ok {
				storageRootID = id
			}
		}
	}
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "create_storage_root",
		Expected: "HTTP 201 with storage root ID",
		Actual:   fmt.Sprintf("HTTP %d, id=%.0f", createCode, storageRootID),
		Passed:   createOK && storageRootID > 0,
		Message:  challenge.Ternary(createOK, fmt.Sprintf("Storage root created: id=%.0f name=%s", storageRootID, rootName), fmt.Sprintf("Create storage root failed: code=%d err=%v", createCode, createErr)),
	})
	if !createOK || storageRootID == 0 {
		challengeResult := c.CreateResult(challenge.StatusFailed, start, assertions, nil, outputs, "failed to create storage root")
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), "failed to create storage root"))
		return challengeResult, nil
	}
	outputs["storage_root_id"] = fmt.Sprintf("%.0f", storageRootID)

	// Step 4: Queue a scan for each configured content directory
	type dirScan struct {
		dir   Directory
		jobID string
	}
	var scans []dirScan

	for _, dir := range ep.Directories {
		scanBody := fmt.Sprintf(
			`{"storage_root_id":%.0f,"path":%q,"scan_type":"full","max_depth":10}`,
			storageRootID, dir.Path,
		)
		scanCode, scanRaw, scanErr := client.PostJSON(ctx, "/api/v1/scans", scanBody)
		scanOK := scanErr == nil && (scanCode == 200 || scanCode == 202)
		var jobID string
		if scanOK && scanRaw != nil {
			var resp map[string]interface{}
			if json.Unmarshal(scanRaw, &resp) == nil {
				if id, ok := resp["job_id"].(string); ok {
					jobID = id
				}
			}
		}
		target := fmt.Sprintf("queue_scan_%s", dir.ContentType)
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   target,
			Expected: "HTTP 202 with job_id",
			Actual:   fmt.Sprintf("HTTP %d, job_id=%s", scanCode, jobID),
			Passed:   scanOK && jobID != "",
			Message:  challenge.Ternary(scanOK && jobID != "", fmt.Sprintf("Scan queued for %s: job_id=%s", dir.Path, jobID), fmt.Sprintf("Queue scan for %s failed: code=%d err=%v", dir.Path, scanCode, scanErr)),
		})
		if !scanOK || jobID == "" {
			challengeResult := c.CreateResult(challenge.StatusFailed, start, assertions, nil, outputs, fmt.Sprintf("failed to queue scan for %s", dir.Path))
			challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("failed to queue scan for %s", dir.Path)))
			return challengeResult, nil
		}
		scans = append(scans, dirScan{dir: dir, jobID: jobID})
		outputs[fmt.Sprintf("job_id_%s", dir.ContentType)] = jobID
	}

	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "scans_queued",
		Expected: fmt.Sprintf("%d scans queued", len(ep.Directories)),
		Actual:   fmt.Sprintf("%d scans queued", len(scans)),
		Passed:   len(scans) == len(ep.Directories),
		Message:  fmt.Sprintf("All %d content directory scans queued", len(scans)),
	})

	// Step 5: Poll all scans until completed/failed (timeout 60 min)
	pollTimeout := 60 * time.Minute
	pollInterval := 5 * time.Second
	deadline := time.Now().Add(pollTimeout)

	completed := make(map[string]bool)
	failed := make(map[string]string)
	var totalFilesFound float64

	c.ReportProgress("polling scan status", map[string]any{
		"total_scans": len(scans),
	})

	pollRound := 0
	for time.Now().Before(deadline) {
		allDone := true
		for _, ds := range scans {
			if completed[ds.jobID] || failed[ds.jobID] != "" {
				continue
			}
			statusCode, statusBody, statusErr := client.Get(ctx, "/api/v1/scans/"+ds.jobID)
			if statusErr != nil || statusCode != 200 {
				if statusCode == 404 {
					completed[ds.jobID] = true
					continue
				}
				allDone = false
				continue
			}
			if statusBody != nil {
				if s, ok := statusBody["status"].(string); ok {
					if s == "completed" {
						completed[ds.jobID] = true
						if f, ok := statusBody["files_found"].(float64); ok {
							totalFilesFound += f
						}
						continue
					} else if s == "failed" {
						errMsg := "unknown error"
						if e, ok := statusBody["error"].(string); ok {
							errMsg = e
						}
						failed[ds.jobID] = errMsg
						continue
					}
				}
			}
			allDone = false
		}

		// Report progress on every poll round so the
		// liveness monitor knows we're alive.
		pollRound++
		c.ReportProgress("scan poll", map[string]any{
			"poll_round":  pollRound,
			"completed":   len(completed),
			"failed":      len(failed),
			"pending":     len(scans) - len(completed) - len(failed),
			"total_scans": len(scans),
		})

		if allDone {
			break
		}
		time.Sleep(pollInterval)
	}

	// Assess scan results
	allCompleted := len(completed) == len(scans) && len(failed) == 0
	for _, ds := range scans {
		scanOK := completed[ds.jobID]
		msg := fmt.Sprintf("Scan for %s (%s): ", ds.dir.Path, ds.dir.ContentType)
		if scanOK {
			msg += "completed"
		} else if errMsg, isFailed := failed[ds.jobID]; isFailed {
			msg += fmt.Sprintf("failed: %s", errMsg)
		} else {
			msg += "timed out"
		}
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   fmt.Sprintf("scan_completed_%s", ds.dir.ContentType),
			Expected: "completed",
			Actual:   challenge.Ternary(scanOK, "completed", challenge.Ternary(failed[ds.jobID] != "", "failed", "timed_out")),
			Passed:   scanOK,
			Message:  msg,
		})
	}

	if !allCompleted {
		challengeResult := c.CreateResult(challenge.StatusFailed, start, assertions, nil, outputs, fmt.Sprintf("scans incomplete: %d completed, %d failed, %d pending", len(completed), len(failed), len(scans)-len(completed)-len(failed)))
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("scans incomplete: %d completed, %d failed, %d pending", len(completed), len(failed), len(scans)-len(completed)-len(failed))))
		return challengeResult, nil
	}
	outputs["scan_files_found"] = fmt.Sprintf("%.0f", totalFilesFound)

	// Step 6: GET /api/v1/stats/overall — verify total_files > 0 and total_size > 0
	// Retry up to 12 times with 10-second intervals (2 min total) to handle
	// database lock errors from concurrent post-scan aggregation.
	var overallCode int
	var overallBody map[string]interface{}
	var overallErr error
	for retry := 0; retry < 12; retry++ {
		overallCode, overallBody, overallErr = client.Get(ctx, "/api/v1/stats/overall")
		if overallErr == nil && overallCode == 200 {
			break
		}
		c.ReportProgress("stats retry", map[string]any{
			"retry": retry + 1,
			"code":  overallCode,
		})
		time.Sleep(10 * time.Second)
	}
	overallOK := overallErr == nil && overallCode == 200
	var totalFiles, totalSize float64
	if overallBody != nil {
		if data, ok := overallBody["data"].(map[string]interface{}); ok {
			if f, ok := data["total_files"].(float64); ok {
				totalFiles = f
			}
			if s, ok := data["total_size"].(float64); ok {
				totalSize = s
			}
		} else {
			if f, ok := overallBody["total_files"].(float64); ok {
				totalFiles = f
			}
			if s, ok := overallBody["total_size"].(float64); ok {
				totalSize = s
			}
		}
	}

	filesOK := overallOK && totalFiles > 0
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "min_count",
		Target:   "total_files",
		Expected: "> 0 files in catalog",
		Actual:   fmt.Sprintf("%.0f files", totalFiles),
		Passed:   filesOK,
		Message:  challenge.Ternary(filesOK, fmt.Sprintf("Catalog contains %.0f files", totalFiles), fmt.Sprintf("Catalog has 0 files: code=%d err=%v", overallCode, overallErr)),
	})

	sizeOK := overallOK && totalSize > 0
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "min_count",
		Target:   "total_size",
		Expected: "> 0 bytes in catalog",
		Actual:   fmt.Sprintf("%.0f bytes", totalSize),
		Passed:   sizeOK,
		Message:  challenge.Ternary(sizeOK, fmt.Sprintf("Catalog total size: %.0f bytes", totalSize), fmt.Sprintf("Catalog has 0 bytes: code=%d", overallCode)),
	})

	outputs["total_files"] = fmt.Sprintf("%.0f", totalFiles)
	outputs["total_size"] = fmt.Sprintf("%.0f", totalSize)

	scanDuration := time.Since(start)
	metrics := map[string]challenge.MetricValue{
		"scan_time_ms":        {Name: "scan_time_ms", Value: float64(scanDuration.Milliseconds()), Unit: "ms"},
		"total_files_found":   {Name: "total_files_found", Value: totalFiles, Unit: "count"},
		"total_size_bytes":    {Name: "total_size_bytes", Value: totalSize, Unit: "bytes"},
		"directories_scanned": {Name: "directories_scanned", Value: float64(len(scans)), Unit: "count"},
	}

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(status, start, assertions, metrics, outputs, "")
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// createStorageRootBody is the POST /api/v1/storage/roots body for the NAS endpoint. The NAS is on the operator's LAN, so it carries the operator's
// confirmation (CAT-CPN-1); the flag is harmless for a public destination.
func createStorageRootBody(rootName string, ep Endpoint) string {
	return fmt.Sprintf(
		`{"name":%q,"protocol":"smb","host":%q,"port":%d,"path":%q,"username":%q,"password":%q,"domain":%q,"max_depth":10,"confirm_private_network":true}`,
		rootName, ep.Host, ep.Port, ep.Share, ep.Username, ep.Password, ep.Domain,
	)
}
