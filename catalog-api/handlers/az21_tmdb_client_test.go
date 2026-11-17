package handlers

import (
	"testing"

	"catalogizer/internal/netguard"
	"catalogizer/internal/netguard/netguardtest"
	"catalogizer/internal/services"

	"github.com/stretchr/testify/require"
)

// AZ2-1: the TMDB client built by the real handler method refuses a redirect into a blocked class.
func TestAZ21_TMDBClientRefusesRedirectToBlockedClass(t *testing.T) {
	services.AllowPrivateSSRFForTests(t)
	top := netguardtest.New(t)
	h := &MediaEntityHandler{}
	c := h.buildProxyHTTPClient(5e9)
	r, err := c.Get(top.FrontURL + "/ok")
	require.NoError(t, err)
	r.Body.Close()
	_, err = c.Get(top.FrontURL + "/to-unspecified")
	require.ErrorIs(t, err, netguard.ErrBlocked)
	require.Equal(t, 0, top.SinkHits())
}
