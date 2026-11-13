package netguard

import (
	"context"
	"net"
)

// OperatorResolve resolves host (an IP literal or a DNS name) under the operator-class policy and returns the addresses a connection may
// reach: the refused ones (loopback, link-local, metadata, reserved, own addresses) are dropped, and the name is refused when none remain.
// It is the resolve half of OperatorDialContext for callers that hand an ADDRESS to another party (the kernel NFS client): they must use the
// returned addresses and never the name (SEC-DIAL-1, anti DNS rebinding).
func OperatorResolve(ctx context.Context, host string) ([]net.IP, error) {
	return resolveOperator(ctx, host)
}

// CheckOperatorHost reports whether host passes the operator-class policy (see OperatorResolve). It is the create-time early refusal for
// storage roots that name a host instead of a URL (smb, ftp, nfs); the dial re-checks on every connection.
func CheckOperatorHost(ctx context.Context, host string) error {
	_, err := resolveOperator(ctx, host)
	return err
}
