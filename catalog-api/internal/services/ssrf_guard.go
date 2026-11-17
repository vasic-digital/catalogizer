// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package services SSRF guard — thin adapter over the canonical
// digital.vasic.security/pkg/ssrf. All callers keep their existing
// public API (SSRFGuardConfig, SSRFResolver, GuardProviderURL,
// ErrSSRFBlocked, AllowPrivateSSRFForTests, SetTestAllowPrivateNetworks).
// The duplicated algorithm (~150 LOC) is removed; the guard logic lives
// exclusively in digital.vasic.security/pkg/ssrf.
package services

import (
	"fmt"

	"catalogizer/internal/netguard"

	"digital.vasic.security/pkg/ssrf"
)

// SSRFGuardConfig tunes provider SSRF defence. Zero value = safe
// defaults: public hosts only, http + https only.
// Type alias so callers can pass ssrf.Config literals directly.
type SSRFGuardConfig = ssrf.Config

// SSRFResolver is the narrow DNS contract the guard needs.
// Type alias preserving the interface for callers that inject stubs.
type SSRFResolver = ssrf.Resolver

// ErrSSRFBlocked is returned when the guard rejects an outbound URL.
// Assigned from the canonical ssrf.ErrBlocked so errors.Is() chains
// work across the package boundary.
var ErrSSRFBlocked = ssrf.ErrBlocked

// testAllowPrivateNetworks, when true, globally relaxes the guard so
// in-process httptest servers (always 127.0.0.1) can be reached.
// Tests flip this with AllowPrivateSSRFForTests(t). The production
// binary never touches the var — it defaults to false.
var testAllowPrivateNetworks bool

// AllowPrivateSSRFForTests flips testAllowPrivateNetworks for the
// duration of t. Only for use inside *_test.go files that stand up
// httptest servers. Registers a t.Cleanup hook to always restore.
func AllowPrivateSSRFForTests(t interface{ Cleanup(func()) }) {
	prev := testAllowPrivateNetworks
	testAllowPrivateNetworks = true
	restoreLoop := netguard.SetTestAllowLoopback(true) // AZ2-1: provider clients reach httptest loopback servers too
	t.Cleanup(func() { testAllowPrivateNetworks = prev; restoreLoop() })
}

// SetTestAllowPrivateNetworks flips the package-wide test-only SSRF
// relax and returns a restore func. Only for other packages' TestMain
// helpers that need httptest loopback reachability. Production code
// must never call this.
func SetTestAllowPrivateNetworks(allow bool) func() {
	prev := testAllowPrivateNetworks
	testAllowPrivateNetworks = allow
	restoreLoop := netguard.SetTestAllowLoopback(allow)
	return func() { testAllowPrivateNetworks = prev; restoreLoop() }
}

// GuardProviderURL parses target and runs every SSRF check, delegating
// to the canonical digital.vasic.security/pkg/ssrf.Validate. Returns
// ErrSSRFBlocked (wrapped with "services: " prefix) on rejection, nil
// on pass.
func GuardProviderURL(target string, cfg SSRFGuardConfig) error {
	if testAllowPrivateNetworks {
		cfg.AllowPrivateNetworks = true
	}
	if err := ssrf.Validate(target, cfg); err != nil {
		return fmt.Errorf("services: %w", err)
	}
	return nil
}
