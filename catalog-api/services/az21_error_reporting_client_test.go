package services

import (
	"testing"

	"catalogizer/internal/netguard"
	"catalogizer/internal/netguard/netguardtest"
	isvc "catalogizer/internal/services"

	"github.com/stretchr/testify/require"
)

// AZ2-1: the error-reporting webhook client refuses a redirect into a blocked class.
func TestAZ21_ErrorReportingClientRefusesRedirectToBlockedClass(t *testing.T) {
	isvc.AllowPrivateSSRFForTests(t)
	top := netguardtest.New(t)
	c := NewErrorReportingService(nil, nil).httpClient
	r, err := c.Get(top.FrontURL + "/ok")
	require.NoError(t, err)
	r.Body.Close()
	_, err = c.Get(top.FrontURL + "/to-unspecified")
	require.ErrorIs(t, err, netguard.ErrBlocked)
	require.Equal(t, 0, top.SinkHits())
}
