package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// F6: one name with many vetted answers must not multiply the connect cost: at most maxVettedDials addresses are tried.
func TestOperatorDialContextCapsVettedDials(t *testing.T) {
	oldL, oldD := lookupIPAddr, dialTCP
	defer func() { lookupIPAddr, dialTCP = oldL, oldD }()
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		var a []net.IPAddr
		for i := 1; i <= 9; i++ {
			a = append(a, net.IPAddr{IP: net.ParseIP(fmt.Sprintf("192.168.1.%d", i))})
		}
		return a, nil
	}
	var tried []string
	dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, port string) (net.Conn, error) {
		tried = append(tried, ip)
		return nil, errors.New("down")
	}
	_, err := OperatorDialContext(context.Background(), "tcp", "many.lan:445")
	require.Error(t, err)
	require.Len(t, tried, 4)
	require.Equal(t, []string{"192.168.1.1", "192.168.1.2", "192.168.1.3", "192.168.1.4"}, tried, "the first answers, in order")
}

// F6 control: a later answer inside the cap still gets its turn after earlier ones fail.
func TestOperatorDialContextStillFailsOverWithinCap(t *testing.T) {
	oldL, oldD := lookupIPAddr, dialTCP
	defer func() { lookupIPAddr, dialTCP = oldL, oldD }()
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("192.168.1.1")}, {IP: net.ParseIP("192.168.1.2")}, {IP: net.ParseIP("192.168.1.3")}}, nil
	}
	n := 0
	dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, port string) (net.Conn, error) {
		n++
		if ip == "192.168.1.3" {
			c, _ := net.Pipe()
			return c, nil
		}
		return nil, errors.New("down")
	}
	c, err := OperatorDialContext(context.Background(), "tcp", "many.lan:445")
	require.NoError(t, err)
	_ = c.Close()
	require.Equal(t, 3, n)
}
