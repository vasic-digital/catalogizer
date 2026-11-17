//go:build !routedump

package main

import "github.com/gin-gonic/gin"

// routeDumpHook is a no-op in every normal build. The test-only build tag "routedump" (main_routedump_on.go) replaces it with a
// dump of the finished route table, which the admin-gate ground-truth test reads (ADMD1F1-S1 round 5).
func routeDumpHook(_ *gin.Engine) {}
