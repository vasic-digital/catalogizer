package services

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"catalogizer/internal/config"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// SEC-DIAL-1 (unit-class evidence, §11.4.289): the config-file SMB host of SMBService.getConnection is vetted and the vetted IP is dialled.
func secDialSMBService(host string) *SMBService {
	cfg := &config.Config{SMB: config.SMBConfig{Hosts: []config.SMBHost{{Name: "h", Host: host, Port: 445, Username: "u", Password: "p"}}}}
	return NewSMBService(cfg, zap.NewNop())
}

func TestSECDIAL1_SMBServiceConfigHostVetted(t *testing.T) {
	var dialed []string
	withSMBDialer(t, func(ctx context.Context, addr string, d time.Duration) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, errors.New("recorded")
	})
	for _, host := range []string{"127.0.0.1", "100.100.100.200", "168.63.129.16", "198.18.0.1", "169.254.169.254", "0.0.0.0"} {
		_, err := secDialSMBService(host).getConnection("h")
		require.ErrorIs(t, err, ErrInvalidSMBInput, host)
	}
	require.Empty(t, dialed, "nothing may be dialled for a refused config host")
	_, err := secDialSMBService("192.168.1.30").getConnection("h")
	require.Error(t, err)
	require.Equal(t, []string{"192.168.1.30:445"}, dialed, "a LAN NAS is dialled")
}

// DNS rebinding: the name is resolved ONCE, vetted, and the dial gets that IP literal, never the name.
func TestSECDIAL1_SMBServiceDialsPinnedIPNotName(t *testing.T) {
	var dialed []string
	resolves := 0
	withSMBHostResolver(t, func(h string) ([]net.IP, error) {
		resolves++
		if resolves == 1 {
			return []net.IP{net.ParseIP("192.168.9.9")}, nil
		}
		return []net.IP{net.ParseIP("169.254.169.254")}, nil // a second lookup would be the rebound answer
	})
	withSMBDialer(t, func(ctx context.Context, addr string, d time.Duration) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, errors.New("recorded")
	})
	_, _ = secDialSMBService("nas.lan").getConnection("h")
	require.Equal(t, []string{"192.168.9.9:445"}, dialed)
	require.Equal(t, 1, resolves)
}
