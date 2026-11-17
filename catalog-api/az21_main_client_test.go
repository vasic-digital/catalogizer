package main

import (
	"testing"

	"catalogizer/internal/netguard"
	"catalogizer/internal/netguard/netguardtest"
	"catalogizer/internal/services"

	root_config "catalogizer/config"

	"github.com/stretchr/testify/require"
)

// AZ2-1: the LLM provider client built in main.go refuses a redirect into a blocked class.
func TestAZ21_MainProxyHTTPClientRefusesRedirectToBlockedClass(t *testing.T) {
	services.AllowPrivateSSRFForTests(t)
	top := netguardtest.New(t)
	c := buildProxyHTTPClient(root_config.ProxyConfig{})
	r, err := c.Get(top.FrontURL + "/ok")
	require.NoError(t, err)
	r.Body.Close()
	_, err = c.Get(top.FrontURL + "/to-unspecified")
	require.ErrorIs(t, err, netguard.ErrBlocked)
	require.Equal(t, 0, top.SinkHits())
}
