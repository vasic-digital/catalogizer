package filesystem

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

// ftpOpTimeout is the IDLE bound of a protocol operation: the longest the control channel may wait for a reply, and the longest a
// data transfer may go without moving a byte. It is pushed forward on every Read/Write of a data connection (progress), so a multi-GB
// transfer is never cut while bytes still move (review F2), but a server that stops answering is given up on after this long.
// Before ST-D2 an operation against a silent server blocked forever in textproto.ReadResponse.
var ftpOpTimeout = 5 * time.Minute

// ftpDialTimeout bounds connecting, the greeting banner, login and the initial CWD/PWD of a new session (review F1).
var ftpDialTimeout = 30 * time.Second

// ftpConnGroup tracks every network connection one logical FTP session opens (the control connection and its passive data connections)
// so one deadline policy can be applied to all of them, and so an expired context can unblock a read that is in progress.
type ftpConnGroup struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
	// armed: a deadline policy is in force (idle bound + optional hard deadline). When false no deadline is set on any connection, which
	// is the state of a transfer handed to the caller after its operation returned (reading a returned stream is not time-capped).
	armed     bool
	idle      time.Duration
	hard      time.Time // zero = none (ctx deadline)
	cancelled bool      // ctx ended: deadlines are in the past and must not be pushed forward again
	lastTouch time.Time
	curDL     time.Time
	// opCtx is the context of the operation currently running on the session; passive data connections are dialled with it. The Connect
	// context only governs the control connection: it commonly ends right after Connect returns and must not break later transfers.
	opCtx context.Context
	first bool // the control connection (first dial) has been made
}

type groupConn struct {
	net.Conn
	g    *ftpConnGroup
	data bool
}

func (c *groupConn) Close() error {
	c.g.mu.Lock()
	delete(c.g.conns, c.Conn)
	c.g.mu.Unlock()
	return c.Conn.Close()
}

// Read and Write push the idle deadline forward on data connections: progress keeps an operation alive.
func (c *groupConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 && c.data {
		c.g.touch()
	}
	return n, err
}

func (c *groupConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 && c.data {
		c.g.touch()
	}
	return n, err
}

// dialFunc dials like the library's default (30 s connect timeout) but with ctx, and registers the connection in the group.
func (g *ftpConnGroup) dialFunc(ctx context.Context) func(network, address string) (net.Conn, error) {
	return func(network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 30 * time.Second}
		g.mu.Lock()
		dctx := ctx
		data := g.first
		if data { // a data connection
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
		if dl := g.deadlineLocked(); !dl.IsZero() {
			_ = conn.SetDeadline(dl)
		}
		g.mu.Unlock()
		return &groupConn{Conn: conn, g: g, data: data}, nil
	}
}

// deadlineLocked is the deadline new activity gets: now+idle, never later than the hard (ctx) deadline; in the past once cancelled.
func (g *ftpConnGroup) deadlineLocked() time.Time {
	if g.cancelled {
		return time.Unix(1, 0)
	}
	if !g.armed {
		return time.Time{}
	}
	dl := time.Now().Add(g.idle)
	if !g.hard.IsZero() && g.hard.Before(dl) {
		dl = g.hard
	}
	g.curDL = dl
	return dl
}

func (g *ftpConnGroup) applyLocked() {
	dl := g.deadlineLocked()
	for c := range g.conns {
		_ = c.SetDeadline(dl)
	}
	g.lastTouch = time.Now()
}

// arm starts a deadline policy: idle bound plus the ctx's own deadline as hard limit, applied to every live connection.
func (g *ftpConnGroup) arm(ctx context.Context, idle time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.armed, g.idle, g.cancelled, g.opCtx = true, idle, false, ctx
	g.hard = time.Time{}
	if d, ok := ctx.Deadline(); ok {
		g.hard = d
	}
	g.applyLocked()
}

// disarm removes every deadline; the caller may keep reading a returned transfer without a cap.
func (g *ftpConnGroup) disarm() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.armed, g.opCtx = false, nil
	for c := range g.conns {
		_ = c.SetDeadline(time.Time{})
	}
}

// cancel makes every deadline immediately expired: a read or write in progress returns at once.
func (g *ftpConnGroup) cancel() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancelled = true
	for c := range g.conns {
		_ = c.SetDeadline(time.Unix(1, 0))
	}
}

// touch is called after a data connection moved bytes: the idle deadline of the whole session moves forward (the control connection
// is waiting for the transfer's final reply meanwhile). Rate-limited to a quarter of the idle bound.
func (g *ftpConnGroup) touch() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.armed || g.cancelled || time.Since(g.lastTouch) < g.idle/4 {
		return
	}
	g.applyLocked()
}

// expired reports whether the current deadline has passed, i.e. a timeout (not necessarily a ctx cancel) may have cut an exchange.
func (g *ftpConnGroup) expired() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cancelled || (g.armed && !g.curDL.IsZero() && !time.Now().Before(g.curDL))
}

// beginOp is the entry of every serialised FTP operation. It (1) fails at once on an already-cancelled context, without touching the
// network (ST-D1), (2) waits for its turn on the control connection, giving up when ctx ends (ST-D2), (3) takes the session snapshot,
// and (4) arms the idle/hard deadline policy on the session's connections, plus an immediate expiry if ctx ends mid-operation. The
// returned release(opErr) must be called exactly once (defer it) with the error the operation is returning. When the operation FAILED
// because the context or a deadline cut it short, the control connection may hold an unread reply, so release replaces the session; an
// operation that succeeded never loses its session, whatever the context does afterwards (review F4).
func (c *FTPClient) beginOp(ctx context.Context) (*ftp.ServerConn, string, func(opErr error), error) {
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
	var grp *ftpConnGroup
	if v, ok := c.ctl.Load(conn); ok {
		grp = v.(*ftpConnGroup)
		grp.arm(ctx, ftpOpTimeout)
	}
	stop := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			if grp != nil {
				grp.cancel()
			}
		case <-stop:
		}
	}()
	release := func(opErr error) {
		close(stop)
		<-watcherDone
		cut := opErr != nil && (ctx.Err() != nil || (grp != nil && grp.expired()))
		if grp != nil {
			grp.disarm()
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

// armDial bounds a new session's connect, greeting banner, login and initial CWD/PWD (review F1): idle = ftpDialTimeout, hard = ctx's
// deadline, and an expiry at once if ctx ends. The returned func removes the deadlines (call it when the dial finished either way).
func (g *ftpConnGroup) armDial(ctx context.Context) func() {
	g.arm(ctx, ftpDialTimeout)
	stop := context.AfterFunc(ctx, g.cancel)
	return func() {
		stop()
		g.disarm()
	}
}
