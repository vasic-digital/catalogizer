package filesystem

// REVIEWER PROBE (unit-class test double, §11.4.289 C). Not part of the commit.
// A server that refuses STOR before any data flows (550/553 "overwrite not permitted", the proftpd AllowOverwrite-off shape)
// while DELE is allowed: CopyFile must NOT delete the pre-existing destination file it was refused to overwrite.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
)

type refuseStorFTP struct {
	ln      net.Listener
	mu      sync.Mutex
	deleted []string
}

func (s *refuseStorFTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	var wmu sync.Mutex
	say := func(f string, a ...interface{}) { wmu.Lock(); fmt.Fprintf(c, f+"\r\n", a...); wmu.Unlock() }
	say("220 ready")
	var dl net.Listener
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		f := strings.Fields(strings.TrimSpace(line) + " x")
		arg := ""
		if len(f) > 2 {
			arg = f[1]
		}
		switch strings.ToUpper(f[0]) {
		case "USER":
			say("331 pw")
		case "PASS":
			say("230 ok")
		case "TYPE":
			say("200 ok")
		case "PWD":
			say(`257 "/"`)
		case "EPSV":
			dl, _ = net.Listen("tcp", "127.0.0.1:0")
			say("229 ok (|||%d|)", dl.Addr().(*net.TCPAddr).Port)
		case "RETR":
			say("150 opening")
			dc, err := dl.Accept()
			dl.Close()
			if err != nil {
				return
			}
			go func() {
				dc.Write([]byte("new-content"))
				dc.(*net.TCPConn).CloseWrite()
				io.Copy(io.Discard, dc)
				dc.Close()
				say("226 done")
			}()
		case "MKD":
			say("550 exists")
		case "STOR":
			if dl != nil {
				dl.Close()
			}
			say("553 overwrite not permitted")
		case "DELE":
			s.mu.Lock()
			s.deleted = append(s.deleted, arg)
			s.mu.Unlock()
			say("250 deleted")
		case "QUIT":
			say("221 bye")
			return
		default:
			say("502 no")
		}
	}
}

func TestReviewerProbe_RefusedStorMustNotDeleteExistingDestination(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	s := &refuseStorFTP{ln: ln}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/existing.bin"); err == nil {
		t.Fatalf("CopyFile succeeded although STOR was refused")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.deleted {
		if d == "/dir/existing.bin" {
			t.Fatalf("CopyFile DELETED the pre-existing destination %s after the server refused to overwrite it (data loss)", d)
		}
	}
}
