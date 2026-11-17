package main

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/quic-go/quic-go/http3"
	"go.uber.org/zap"
)

// h3Port is the fixed HTTPS (TCP, HTTP/2) and HTTP/3 (UDP, QUIC) port. The Alt-Svc value in
// altSvcMiddleware names the same port.
const h3Port = 28443

// h3Serve runs the HTTP/3 server on the bound UDP socket. It is a variable only so that a test
// can make Serve fail after the bind (ALTSVC3-R-m1); production code never reassigns it.
var h3Serve = func(s *http3.Server, c net.PacketConn) error { return s.Serve(c) }

// startH3Stack loads the TLS certificate, binds TCP and UDP on host:port, and starts the
// HTTPS (HTTP/2) and HTTP/3 servers. It sets h3Advertised only AFTER the UDP bind is
// confirmed (the sockets are open, so a client that follows the advertisement reaches a
// listener); a certificate error or a UDP bind failure leaves it false and is logged.
// If a server later stops with an error other than a normal close, the flag is cleared
// again. The UDP socket is closed when Serve returns (http3.Server.Close does not close a socket
// it was handed, ALTSVC3-R-n1). It returns the servers that were started (nil when not started), for shutdown.
func startH3Stack(logger *zap.Logger, router http.Handler, host string, port int,
	loadCert func() (tls.Certificate, error)) (*http.Server, *http3.Server) {
	cert, err := loadCert()
	if err != nil {
		logger.Error("Failed to get TLS certificate for HTTP/3", zap.Error(err))
		return nil, nil
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3", "h2", "http/1.1"},
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	var httpsServer *http.Server
	if tcpLn, err := net.Listen("tcp", addr); err != nil {
		logger.Error("HTTPS (HTTP/2 with TLS) bind failed", zap.String("address", addr), zap.Error(err))
	} else {
		httpsServer = &http.Server{Addr: addr, Handler: router, TLSConfig: tlsConfig, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			logger.Info("Starting HTTPS server (HTTP/2 with TLS)", zap.String("address", addr))
			if err := httpsServer.ServeTLS(tcpLn, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("HTTPS server failed", zap.Error(err))
			}
		}()
	}

	udpConn, err := net.ListenPacket("udp", addr)
	if err != nil {
		logger.Error("HTTP/3 (QUIC) bind failed; Alt-Svc is not advertised", zap.String("address", addr), zap.Error(err))
		return httpsServer, nil
	}
	http3Server := &http3.Server{Addr: addr, Handler: router, TLSConfig: tlsConfig}
	h3Advertised.Store(true) // after the bind, before serving; cleared if the server fails
	go func() {
		logger.Info("Starting HTTP/3 (QUIC) server", zap.String("address", addr))
		defer udpConn.Close() // Close/Shutdown of http3Server does not close a socket passed to Serve
		if err := h3Serve(http3Server, udpConn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h3Advertised.Store(false)
			logger.Error("HTTP/3 server failed", zap.Error(err))
		}
	}()
	return httpsServer, http3Server
}
