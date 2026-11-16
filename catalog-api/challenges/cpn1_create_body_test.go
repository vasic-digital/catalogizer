package challenges

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// CAT-CPN-1 (review F3): the populate challenge creates an SMB root on a LAN NAS, so its request must carry the operator's confirmation.
func TestCPN1_CreateStorageRootBodyConfirmsPrivateNetwork(t *testing.T) {
	raw := createStorageRootBody("nas-root", Endpoint{Host: "192.168.77.9", Port: 445, Share: "media", Username: "u", Password: "p", Domain: "WG"})
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &m), raw)
	require.Equal(t, true, m["confirm_private_network"])
	require.Equal(t, "smb", m["protocol"])
	require.Equal(t, "192.168.77.9", m["host"])
}
