package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ALTSVC3 review follow-ups (m1..m5, n1, n2). Ephemeral ports only; a port or UDP environment
// problem is SKIPPED as INCONCLUSIVE, never a PASS and never a FAIL.

// startStackOrSkip starts the stack on a port found free on TCP and UDP. If another process takes
// the port between the probe and the bind (n2), it retries on a new port and finally SKIPs as
// INCONCLUSIVE instead of failing.
func startStackOrSkip(t *testing.T, handler http.Handler) (int, *http.Server, *http3.Server) {
	t.Helper()
	for i := 0; i < 5; i++ {
		p := freeTCPUDPPort(t)
		https, h3 := startH3Stack(zap.NewNop(), handler, "127.0.0.1", p, testCert(t))
		if https != nil && h3 != nil {
			t.Cleanup(func() { stopStack(t, https, h3) })
			return p, https, h3
		}
		stopStack(t, https, h3)
		h3Advertised.Store(false)
	}
	t.Skip("INCONCLUSIVE: the freshly probed port was taken before the bind (5 attempts)")
	return 0, nil, nil
}

// m1: Serve fails after the bind: the advertisement is cleared (mutant M1: Store(false) removed).
func TestAltSvcStartH3Stack_ServeFailureClearsFlag(t *testing.T) {
	setH3Advertised(t, false)
	prev := h3Serve
	h3Serve = func(*http3.Server, net.PacketConn) error { return errors.New("injected serve failure") }
	t.Cleanup(func() { h3Serve = prev })
	p := freeTCPUDPPort(t)
	https, h3 := startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	require.NotNil(t, h3, "the bind succeeded, so the server object exists")
	require.Eventually(t, func() bool { return !h3Advertised.Load() }, 5*time.Second, 10*time.Millisecond,
		"Serve failed after the bind: HTTP/3 must no longer be advertised")
}

// m1 control: a normal ErrServerClosed after Close is not a failure and keeps the flag as it was set by
// the bind (the instrument can see the flag; only a real failure clears it).
func TestAltSvcStartH3Stack_NormalCloseDoesNotClearByFailurePath(t *testing.T) {
	setH3Advertised(t, false)
	_, _, h3 := startStackOrSkip(t, http.NotFoundHandler())
	require.True(t, h3Advertised.Load())
	require.NoError(t, h3.Close())
	time.Sleep(200 * time.Millisecond)
	require.True(t, h3Advertised.Load(), "a normal close (ErrServerClosed) is not a failure")
}

// m2: TCP bind failure: HTTPS is absent, HTTP/3 (UDP) is independent and still starts and advertises.
func TestAltSvcStartH3Stack_TCPBindFailureHTTP3StillStarts(t *testing.T) {
	setH3Advertised(t, false)
	p := freeTCPUDPPort(t)
	tcp, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
	if err != nil {
		t.Skip("INCONCLUSIVE: the probed TCP port was taken before the test could occupy it")
	}
	t.Cleanup(func() { tcp.Close() })
	https, h3 := startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	if h3 == nil {
		// Only INCONCLUSIVE when the UDP port really was taken meanwhile; a free UDP port with no
		// HTTP/3 server is the defect (mutant M5: the TCP failure aborts the whole stack).
		pc, uerr := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(p))
		if uerr != nil {
			t.Skip("INCONCLUSIVE: the UDP port was taken as well")
		}
		pc.Close()
	}
	require.Nil(t, https, "TCP bind failed: no HTTPS server")
	require.NotNil(t, h3, "the TCP failure must not stop the independent HTTP/3 server (mutant M5)")
	require.True(t, h3Advertised.Load(), "UDP is bound, so HTTP/3 is advertised")
	_, uerr := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(p))
	require.Error(t, uerr, "the UDP port is bound by the server")
}

// m2: both ports occupied: nothing starts, nothing is advertised.
func TestAltSvcStartH3Stack_BothBindsFail(t *testing.T) {
	setH3Advertised(t, false)
	p := freeTCPUDPPort(t)
	tcp, terr := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
	udp, uerr := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(p))
	if terr != nil || uerr != nil {
		if tcp != nil {
			tcp.Close()
		}
		if udp != nil {
			udp.Close()
		}
		t.Skip("INCONCLUSIVE: could not occupy both ports")
	}
	t.Cleanup(func() { tcp.Close(); udp.Close() })
	https, h3 := startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	require.Nil(t, https)
	require.Nil(t, h3)
	require.False(t, h3Advertised.Load())
}

// m3: a REAL HTTP/3 (QUIC) request to the served port: the HTTP/3 listener answers with the real router
// (mutant M6b: the HTTP/3 server serves a different handler) and the response advertises Alt-Svc.
func TestAltSvcRealHTTP3Request(t *testing.T) {
	setH3Advertised(t, false)
	p, _, _ := startStackOrSkip(t, handlerForStack(t))
	rt := &http3.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // test-only self-signed certificate
	t.Cleanup(func() { _ = rt.Close() })
	c := &http.Client{Transport: rt, Timeout: 8 * time.Second}
	get := func(path string) *http.Response {
		resp, err := c.Get(fmt.Sprintf("https://127.0.0.1:%d%s", p, path))
		if err != nil {
			var idle *quic.IdleTimeoutError
			var ne net.Error
			if errors.As(err, &idle) || (errors.As(err, &ne) && ne.Timeout()) || os.IsTimeout(err) {
				t.Skipf("INCONCLUSIVE: no QUIC answer over loopback UDP (%v)", err)
			}
			require.NoError(t, err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}
	ok := get("/health")
	require.Equal(t, 3, ok.ProtoMajor, "control: the connection is HTTP/3")
	require.Equal(t, http.StatusOK, ok.StatusCode)
	require.Equal(t, altSvcWant, ok.Header.Get("Alt-Svc"))
	nf := get("/no-such-route-altsvc4")
	require.Equal(t, 3, nf.ProtoMajor)
	require.Equal(t, http.StatusNotFound, nf.StatusCode)
}

// n1: after the HTTP/3 server is closed, the UDP socket is released (mutant: no close of the socket).
func TestAltSvcStartH3Stack_UDPSocketReleasedAfterClose(t *testing.T) {
	setH3Advertised(t, false)
	p, https, h3 := startStackOrSkip(t, http.NotFoundHandler())
	stopStack(t, https, h3)
	require.Eventually(t, func() bool {
		pc, err := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(p))
		if err != nil {
			return false
		}
		pc.Close()
		return true
	}, 5*time.Second, 20*time.Millisecond, "the UDP port must be free again after the server is closed")
}

// ---- m4 + m5: static pins of the advertisement and of the main() wiring ----

// checkAltSvcSources returns the findings for the named sources: the single non-test
// h3Advertised.Store(true) is a top-level statement of startH3Stack after the net.ListenPacket
// assignment, h3Advertised is otherwise only Load/Store(false) (Store(false) inside a function literal of
// startH3Stack), and main() keeps both results of startH3Stack and shuts both down under a nil guard.
func checkAltSvcSources(srcs map[string]string) []string {
	var out []string
	add := func(f string, a ...any) { out = append(out, fmt.Sprintf(f, a...)) }
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for name, src := range srcs {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			add("%s: does not parse: %v", name, err)
			return out
		}
		files[name] = f
	}
	// m4: every use of h3Advertised in non-test sources
	storeTrue := 0
	for name, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			se, ok := ce.Fun.(*ast.SelectorExpr)
			if !ok || exprText(fset, se.X) != "h3Advertised" {
				return true
			}
			switch {
			case se.Sel.Name == "Load":
			case se.Sel.Name == "Store" && len(ce.Args) == 1 && exprText(fset, ce.Args[0]) == "false":
				if name != "main_h3.go" {
					add("%s: h3Advertised.Store(false) outside main_h3.go", name)
				}
			case se.Sel.Name == "Store" && len(ce.Args) == 1 && exprText(fset, ce.Args[0]) == "true":
				storeTrue++
				if name != "main_h3.go" {
					add("%s: h3Advertised.Store(true) outside main_h3.go (the advertisement belongs to startH3Stack after the UDP bind)", name)
				}
			default:
				add("%s: h3Advertised.%s(%s) is not allowed (only Load, Store(false), and the one Store(true) in startH3Stack)", name, se.Sel.Name, exprText(fset, ce))
			}
			return true
		})
	}
	if storeTrue != 1 {
		add("exactly one non-test h3Advertised.Store(true) is allowed (found %d)", storeTrue)
	}
	if h3f := files["main_h3.go"]; h3f != nil {
		var fn *ast.FuncDecl
		for _, d := range h3f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "startH3Stack" {
				fn = fd
			}
		}
		if fn == nil {
			add("main_h3.go: startH3Stack not found")
		} else {
			listenIdx, storeIdx, goStoreFalse := -1, -1, 0
			for i, st := range fn.Body.List {
				switch s := st.(type) {
				case *ast.AssignStmt:
					if len(s.Rhs) == 1 && strings.HasPrefix(exprText(fset, s.Rhs[0]), "net.ListenPacket(") {
						listenIdx = i
					}
				case *ast.ExprStmt:
					if exprText(fset, s.X) == "h3Advertised.Store(true)" {
						storeIdx = i
					}
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if fl, ok := n.(*ast.FuncLit); ok {
					ast.Inspect(fl.Body, func(m ast.Node) bool {
						if ce, ok := m.(*ast.CallExpr); ok && exprText(fset, ce) == "h3Advertised.Store(false)" {
							goStoreFalse++
						}
						return true
					})
				}
				return true
			})
			if listenIdx < 0 {
				add("startH3Stack: no `x, err := net.ListenPacket(...)` top-level statement")
			}
			if storeIdx < 0 {
				add("startH3Stack: h3Advertised.Store(true) must be a TOP-LEVEL statement of the function (not inside a branch)")
			} else if listenIdx >= 0 && storeIdx <= listenIdx+1 {
				add("startH3Stack: h3Advertised.Store(true) (stmt %d) must come after the net.ListenPacket assignment (stmt %d) and its error check", storeIdx, listenIdx)
			}
			if goStoreFalse < 1 {
				add("startH3Stack: the serving goroutine must clear h3Advertised (Store(false)) when Serve fails")
			}
		}
	}
	// m5: main() wiring
	mf := files["main.go"]
	if mf == nil {
		add("main.go missing")
		return out
	}
	var mainFn *ast.FuncDecl
	for _, d := range mf.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "main" {
			mainFn = fd
		}
	}
	if mainFn == nil {
		add("main.go: main not found")
		return out
	}
	assigns, bare := 0, 0
	ast.Inspect(mainFn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Rhs) == 1 {
				if ce, ok := s.Rhs[0].(*ast.CallExpr); ok && strings.HasPrefix(exprText(fset, ce), "startH3Stack(") {
					if s.Tok == token.DEFINE && exprText(fset, &ast.CallExpr{Fun: ast.NewIdent("x"), Args: s.Lhs}) == "x(httpsServer, http3Server)" {
						assigns++
					} else {
						add("main.go: the startH3Stack result must be assigned to exactly `httpsServer, http3Server :=` (found `%s %s`)", exprText(fset, &ast.CallExpr{Fun: ast.NewIdent("x"), Args: s.Lhs}), s.Tok)
					}
				}
			}
		case *ast.ExprStmt:
			if ce, ok := s.X.(*ast.CallExpr); ok && strings.HasPrefix(exprText(fset, ce), "startH3Stack(") {
				bare++
			}
		}
		return true
	})
	if assigns != 1 || bare != 0 {
		add("main.go: main() must keep the startH3Stack results exactly once (found %d assignments, %d discarded calls)", assigns, bare)
	}
	guarded := func(cond, call string) bool {
		hit := false
		ast.Inspect(mainFn.Body, func(n ast.Node) bool {
			is, ok := n.(*ast.IfStmt)
			if !ok || exprText(fset, is.Cond) != cond || is.Init != nil {
				return true
			}
			ast.Inspect(is.Cond, func(ast.Node) bool { return true })
			ast.Inspect(is.Body, func(m ast.Node) bool {
				if ce, ok := m.(*ast.CallExpr); ok && strings.HasPrefix(exprText(fset, ce), call) {
					hit = true
				}
				return true
			})
			return true
		})
		return hit
	}
	if !guarded("httpsServer != nil", "httpsServer.Shutdown(shutdownCtx)") {
		add("main.go: main() must call httpsServer.Shutdown(shutdownCtx) inside `if httpsServer != nil`")
	}
	if !guarded("http3Server != nil", "http3Server.Close()") {
		add("main.go: main() must call http3Server.Close() inside `if http3Server != nil`")
	}
	return out
}

func readAltSvcSources(t *testing.T) map[string]string {
	t.Helper()
	m := map[string]string{}
	names, err := filepath.Glob("*.go")
	require.NoError(t, err)
	require.NotEmpty(t, names, "control: the package sources are found")
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		require.NoError(t, err)
		m[n] = string(b)
	}
	require.Contains(t, m, "main.go")
	require.Contains(t, m, "main_h3.go")
	return m
}

func TestAltSvcAdvertisementAndWiringPins(t *testing.T) {
	srcs := readAltSvcSources(t)
	require.Empty(t, checkAltSvcSources(srcs))
}

// Mutants of the pin itself: each textual edit of the real sources must be reported.
func TestAltSvcPinsKillMutants(t *testing.T) {
	srcs := readAltSvcSources(t)
	require.Empty(t, checkAltSvcSources(srcs), "control: the unmutated sources pass")
	const call = "httpsServer, http3Server := startH3Stack(logger, srv.Handler, cfg.Server.Host, h3Port, getOrCreateSelfSignedCert)"
	const store = "\th3Advertised.Store(true) // after the bind, before serving; cleared if the server fails\n"
	for _, tc := range []struct{ name, file, from, to string }{
		{"M7_store_before_listen", "main_h3.go", store, ""},
		{"M7b_store_moved_before_bind", "main_h3.go", "\tudpConn, err := net.ListenPacket", "\th3Advertised.Store(true)\n\tudpConn, err := net.ListenPacket"},
		{"M7c_second_store_true_in_main_h3", "main_h3.go", "\t\t\th3Advertised.Store(false)\n", "\t\t\th3Advertised.Store(true)\n"},
		{"M7d_store_true_in_other_file", "main.go", "func main() {", "func main() {\n\th3Advertised.Store(true)"},
		{"M7e_store_true_nested", "main_h3.go", store, "\tif http3Server != nil {\n\t\th3Advertised.Store(true)\n\t}\n"},
		{"M7f_swap_not_store", "main_h3.go", store, "\th3Advertised.Swap(true)\n"},
		{"M1s_store_false_removed", "main_h3.go", "\t\t\th3Advertised.Store(false)\n", ""},
		{"M2_result_discarded_blank", "main.go", call, "_, _ = startH3Stack(logger, srv.Handler, cfg.Server.Host, h3Port, getOrCreateSelfSignedCert)\n\tvar httpsServer *http.Server\n\tvar http3Server *http3.Server"},
		{"M2b_first_result_blank", "main.go", call, "_, http3Server := startH3Stack(logger, srv.Handler, cfg.Server.Host, h3Port, getOrCreateSelfSignedCert)\n\tvar httpsServer *http.Server"},
		{"M2c_call_without_assignment", "main.go", call, "startH3Stack(logger, srv.Handler, cfg.Server.Host, h3Port, getOrCreateSelfSignedCert)\n\tvar httpsServer *http.Server\n\tvar http3Server *http3.Server"},
		{"M2d_https_shutdown_unguarded", "main.go", "if httpsServer != nil {", "if true {"},
		{"M2e_h3_close_unguarded", "main.go", "if http3Server != nil {", "if true {"},
		{"M2f_h3_close_removed", "main.go", "http3Server.Close()", "nil"},
		{"M2g_https_shutdown_removed", "main.go", "httpsServer.Shutdown(shutdownCtx)", "nil"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mut := map[string]string{}
			for k, v := range srcs {
				mut[k] = v
			}
			require.Contains(t, mut[tc.file], tc.from, "control: the mutation target text exists")
			mut[tc.file] = strings.Replace(mut[tc.file], tc.from, tc.to, 1)
			require.NotEmpty(t, checkAltSvcSources(mut), "mutant %s must be reported by the pin", tc.name)
		})
	}
}
