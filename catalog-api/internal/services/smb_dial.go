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

// dialVetted tries each vetted IP in order on port and returns the first connection. The SMB session is then opened on that net.Conn
// (go-smb2 Dialer.Dial(conn) takes no host name; NTLM uses the supplied identity, not the target name), so nothing needs the name again.
func dialVetted(ctx context.Context, ips []net.IP, port int, timeout time.Duration) (net.Conn, error) {
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: host does not resolve", ErrInvalidSMBInput)
	}
	var last error
	for _, ip := range ips {
		conn, err := SMBDialer(ctx, net.JoinHostPort(ip.String(), strconv.Itoa(port)), timeout)
		if err == nil {
			return conn, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, errors.Join(last)
}
