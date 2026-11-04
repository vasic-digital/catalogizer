package filesystem

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

// ftpOpTimeout bounds one protocol operation (command exchange plus its data transfer) when the caller's context carries no earlier
// deadline. Before ST-D2 an operation against a server that stopped answering blocked forever in textproto.ReadResponse.
var ftpOpTimeout = 5 * time.Minute

// ftpConnGroup tracks every network connection one logical FTP session opens (the control connection and its passive data connections)
// so one deadline can be applied to all of them, and so an expired context can unblock a read that is in progress.
type ftpConnGroup struct {
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	deadline time.Time
	// opCtx is the context of the operation currently running on the session; passive data connections are dialled with it. The Connect
	// context only governs the control connection: it commonly ends right after Connect returns and must not break later transfers.
	opCtx context.Context
	first bool // the control connection (first dial) has been made
}

type groupConn struct {
	net.Conn
	g *ftpConnGroup
}

func (c *groupConn) Close() error {
	c.g.mu.Lock()
	delete(c.g.conns, c.Conn)
	c.g.mu.Unlock()
	return c.Conn.Close()
}

// dialFunc dials like the library's default (30 s connect timeout) but with ctx, and registers the connection in the group.
func (g *ftpConnGroup) dialFunc(ctx context.Context) func(network, address string) (net.Conn, error) {
	return func(network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 30 * time.Second}
		g.mu.Lock()
		dctx := ctx
		if g.first { // a data connection
			dctx = context.WithoutCancel(ctx)
			if g.opCtx != nil {
				dctx = g.opCtx
			}
		}
		g.first = true
		g.mu.Unlock()
		conn, err := d.DialContext(dctx, network, address)
		if err != nil {
			return nil, err
		}
		g.mu.Lock()
		if g.conns == nil {
			g.conns = map[net.Conn]struct{}{}
		}
		g.conns[conn] = struct{}{}
		dl := g.deadline
		g.mu.Unlock()
		if !dl.IsZero() {
			_ = conn.SetDeadline(dl)
		}
		return &groupConn{Conn: conn, g: g}, nil
	}
}

// setDeadline applies t (zero = none) to every live connection of the group and to connections opened later.
func (g *ftpConnGroup) setOpCtx(ctx context.Context) {
	g.mu.Lock()
	g.opCtx = ctx
	g.mu.Unlock()
}

func (g *ftpConnGroup) setDeadline(t time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.deadline = t
	for c := range g.conns {
		_ = c.SetDeadline(t)
	}
}

// beginOp is the entry of every serialised FTP operation. It (1) fails at once on an already-cancelled context, without touching the
// network (ST-D1), (2) waits for its turn on the control connection, giving up when ctx ends (ST-D2), (3) takes the session snapshot,
// and (4) arms a read/write deadline on the session's connections: the earlier of ctx's deadline and ftpOpTimeout, and an immediate
// one if ctx is cancelled mid-operation. The returned release must be called exactly once (defer it). When the operation was cut
// short by the context or the deadline, the control connection may hold an unread reply, so release replaces the session.
func (c *FTPClient) beginOp(ctx context.Context) (*ftp.ServerConn, string, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, "", nil, err
	}
	select {
	case c.opSem <- struct{}{}:
	case <-ctx.Done():
		return nil, "", nil, ctx.Err()
	}
	conn, base, err := c.session()
	if err != nil {
		<-c.opSem
		return nil, "", nil, err
	}
	if err := ctx.Err(); err != nil { // cancelled while waiting for the turn
		<-c.opSem
		return nil, "", nil, err
	}
	dl := time.Now().Add(ftpOpTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(dl) {
		dl = d
	}
	var grp *ftpConnGroup
	if v, ok := c.ctl.Load(conn); ok {
		grp = v.(*ftpConnGroup)
		grp.setOpCtx(ctx)
		grp.setDeadline(dl)
	}
	stop := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			if grp != nil {
				grp.setDeadline(time.Unix(1, 0)) // unblocks a read/write in progress at once
			}
		case <-stop:
		}
	}()
	release := func() {
		close(stop)
		<-watcherDone
		cut := ctx.Err() != nil || !time.Now().Before(dl)
		if grp != nil {
			grp.setOpCtx(nil)
		}
		if grp != nil && !cut {
			grp.setDeadline(time.Time{})
		}
		if cut {
			rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			_ = c.reconnectIf(rctx, conn) // best effort; on failure the client stays disconnected and the next call says so
			cancel()
		}
		<-c.opSem
	}
	return conn, base, release, nil
}

// lockOps takes the operation turn without arming deadlines. It is for operations that run their own cancellation machinery (CopyFile).
func (c *FTPClient) lockOps(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case c.opSem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return func() { <-c.opSem }, nil
}
