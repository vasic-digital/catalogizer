package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// SMBDialer opens the TCP connection to an already vetted address ("ip:port"). Tests replace it to record what is dialed; production uses
// the system dialer. It is only ever handed an IP literal, never a host name, so it cannot trigger a second DNS lookup.
var SMBDialer = func(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, "tcp", addr)
}

// maxVettedDials caps how many vetted addresses one request tries, so a name with many answers cannot multiply the connect cost.
const maxVettedDials = 4

// smbNow is the clock dialVetted measures its single connect deadline with; tests replace it.
var smbNow = time.Now

// dialVetted tries the vetted IPs in order on port and returns the first connection, trying at most maxVettedDials addresses. ONE connect
// deadline (timeout from the first attempt) is shared by all attempts, so N blackholed answers cost at most timeout, not N x timeout. The
// SMB session is then opened on that net.Conn (go-smb2 Dialer.Dial(conn) takes no host name; NTLM uses the supplied identity, not the
// target name), so nothing needs the name again.
func dialVetted(ctx context.Context, ips []net.IP, port int, timeout time.Duration) (net.Conn, error) {
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: host does not resolve", ErrInvalidSMBInput)
	}
	deadline := smbNow().Add(timeout)
	var last error
	for i, ip := range ips {
		if i >= maxVettedDials {
			break
		}
		remaining := deadline.Sub(smbNow())
		if remaining <= 0 {
			break
		}
		conn, err := SMBDialer(ctx, net.JoinHostPort(ip.String(), strconv.Itoa(port)), remaining)
		if err == nil {
			return conn, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	if last == nil {
		last = errors.New("connect deadline exceeded")
	}
	return nil, last
}
