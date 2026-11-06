package main

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// applyTrustedProxies configures which TCP peers may supply X-Forwarded-For
// (RULE-GO-005 / Master Plan §4.2). Policy:
//   - tp == "":     trust no proxy header; ClientIP() is the real peer.
//   - tp == "auto": loopback + RFC-1918 + ULA (dev behind a local proxy).
//   - otherwise:    explicit comma-separated CIDR/IP allow-list.
//
// Gin derives ClientIP from the RIGHTMOST X-Forwarded-For entry that is not a
// trusted proxy, and only when the TCP peer itself is trusted.
func applyTrustedProxies(r *gin.Engine, tp string) error {
	switch tp {
	case "":
		return r.SetTrustedProxies(nil)
	case "auto":
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
