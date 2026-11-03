package netguard

import (
	"context"
	"net"
	"testing"
)

func TestStrictDialIgnoresAllowList(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	// positive control: the allow-list-aware dialer connects
	c, err := DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("control dial should succeed with the env allow-list: %v", err)
	}
	c.Close()
	if c, err := StrictDialContext(context.Background(), "tcp", ln.Addr().String()); err == nil {
		c.Close()
		t.Fatal("strict dial must refuse loopback even when the env allows it")
	}
	if !IsBlockedIPStrict(net.ParseIP("127.0.0.2")) || IsBlockedIP(net.ParseIP("127.0.0.2")) {
		t.Fatal("strict must block, allow-list-aware must allow, 127.0.0.2")
	}
	for _, ip := range []string{"169.254.169.254", "10.0.0.1", "192.168.1.1", "100.64.0.1", "::1"} {
		if !IsBlockedIPStrict(net.ParseIP(ip)) {
			t.Errorf("%s must be blocked in strict mode", ip)
		}
	}
}
