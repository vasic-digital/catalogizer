package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// applyTrustedProxies configures which TCP peers may supply X-Forwarded-For
// (RULE-GO-005 / Master Plan §4.2). Policy:
//   - tp == "":     trust no proxy header; ClientIP() is the real peer.
//   - tp == "auto": loopback + RFC-1918 + ULA (dev behind a local proxy).
//     DEPRECATED and FORGEABLE (CAT-TRUSTEDPROXIES-AUTO-1): gin skips trusted
//     entries from the right of X-Forwarded-For, so a client that is itself in
//     a trusted range can send "XFF: 9.9.9.9" and have the proxy append its
//     own address, yielding ClientIP 9.9.9.9 and bypassing per-IP limits.
//     gin offers no "trust only the direct hop" mode, so the safe policy is to
//     refuse auto in release mode unless TRUSTED_PROXIES_ALLOW_AUTO=true
//     acknowledges the risk; production must list explicit proxy CIDRs.
//   - otherwise:    explicit comma-separated CIDR/IP allow-list.
//
// Gin derives ClientIP from the RIGHTMOST X-Forwarded-For entry that is not a
// trusted proxy, and only when the TCP peer itself is trusted.
//
// Fail closed: if the policy cannot be applied (refused auto, bad CIDR) the
// engine is left trusting NO proxy header, never gin's trust-everything
// default, and the error is returned for the caller to log.
func applyTrustedProxies(r *gin.Engine, tp string) error {
	if err := setTrustedProxies(r, tp); err != nil {
		_ = r.SetTrustedProxies(nil)
		return err
	}
	return nil
}

func setTrustedProxies(r *gin.Engine, tp string) error {
	switch tp {
	case "":
		return r.SetTrustedProxies(nil)
	case "auto":
		if gin.Mode() == gin.ReleaseMode && os.Getenv("TRUSTED_PROXIES_ALLOW_AUTO") != "true" {
			return fmt.Errorf("TRUSTED_PROXIES=auto is refused in release mode (client-forgeable X-Forwarded-For): " +
				"set TRUSTED_PROXIES to explicit proxy CIDR(s), or TRUSTED_PROXIES_ALLOW_AUTO=true to accept the risk")
		}
		return r.SetTrustedProxies([]string{
			"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
			"::1/128", "fc00::/7",
		})
	}
	var list []string
	for _, s := range strings.Split(tp, ",") {
		if s = strings.TrimSpace(s); s != "" {
			list = append(list, s)
		}
	}
	return r.SetTrustedProxies(list)
}

// trustsEverything reports whether an explicit list contains a catch-all CIDR
// (startup warning: it re-creates gin's trust-all default).
func trustsEverything(tp string) bool {
	for _, s := range strings.Split(tp, ",") {
		if s = strings.TrimSpace(s); s == "0.0.0.0/0" || s == "::/0" {
			return true
		}
	}
	return false
}

// warnTrustedProxies emits the startup warnings for risky TRUSTED_PROXIES
// values (catch-all CIDR, deprecated "auto"). Extracted from main() so the
// warnings are testable with an injected logger (TP-FU-1); behaviour unchanged.
func warnTrustedProxies(logger *zap.Logger, tp string) {
	if trustsEverything(tp) {
		logger.Warn("TRUSTED_PROXIES contains a catch-all CIDR (0.0.0.0/0 or ::/0): every peer may forge its client IP via X-Forwarded-For")
	}
	if tp == "auto" {
		logger.Warn("TRUSTED_PROXIES=auto is DEPRECATED: any client inside a trusted private range can forge its client IP via X-Forwarded-For and bypass per-IP rate limits; use explicit proxy CIDR(s)")
	}
}
