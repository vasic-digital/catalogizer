package providers

import (
	"testing"

	"catalogizer/internal/netguard"
	"catalogizer/internal/netguard/netguardtest"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// AZ2-1: the shared provider-manager client (also handed to the LLM provider) refuses a redirect into a blocked class.
func TestAZ21_ProviderManagerClientRefusesRedirectToBlockedClass(t *testing.T) {
	top := netguardtest.New(t) // TestMain already relaxes loopback for this package
	pm := NewProviderManager(zap.NewNop())
	r, err := pm.client.Get(top.FrontURL + "/ok")
	require.NoError(t, err)
	r.Body.Close()
	_, err = pm.client.Get(top.FrontURL + "/to-unspecified")
	require.ErrorIs(t, err, netguard.ErrBlocked)
	require.Equal(t, 0, top.SinkHits())
}
