import http.server, sys, socketserver
log=open(sys.argv[2],'a')
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        log.write(f"HIT {self.command} {self.path}\n"); log.flush()
        if self.path.startswith('/redir'):
            self.send_response(302); self.send_header('Location','http://169.254.169.254/latest/meta-data/'); self.end_headers(); return
        self.send_response(200); self.send_header('Content-Type','text/plain'); self.end_headers(); self.wfile.write(b'ok')
    do_POST=do_PUT=do_HEAD=do_OPTIONS=do_PROPFIND=do_GET
    def log_message(self,*a): pass
socketserver.TCPServer.allow_reuse_address=True
http.server.ThreadingHTTPServer((sys.argv[1],int(sys.argv[3])),H).serve_forever()
