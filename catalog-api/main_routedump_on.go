//go:build routedump

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

// routeDumpHook (build tag routedump only, never shipped): when ROUTE_DUMP_FILE is set, write every route of the finished engine
// with the runtime function name of each handler of its chain to that file and exit before the server listens. Used by the
// ground-truth test (api_contract_gate_groundtruth_test.go), which builds the real main() with this tag.
func routeDumpHook(e *gin.Engine) {
	p := os.Getenv("ROUTE_DUMP_FILE")
	if p == "" {
		return
	}
	rc, err := routeChainFuncNames(e)
	if err != nil {
		fmt.Fprintln(os.Stderr, "route dump failed:", err)
		os.Exit(3)
	}
	raw, err := json.Marshal(rc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "route dump failed:", err)
		os.Exit(3)
	}
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "route dump failed:", err)
		os.Exit(3)
	}
	os.Exit(0)
}
