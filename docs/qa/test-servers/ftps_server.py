# Explicit FTPS (AUTH TLS) test server on top of pyftpdlib. Corpus read-only except rw/ (enforced by the :ro bind mount).
import os
from pyftpdlib.authorizers import DummyAuthorizer
from pyftpdlib.handlers import TLS_FTPHandler
from pyftpdlib.servers import FTPServer
a = DummyAuthorizer()
a.add_user(os.environ["TS_USER"], os.environ["TS_PASS"], "/srv/corpus", perm="elradfmwMT")
h = TLS_FTPHandler
h.authorizer = a
h.certfile = "/tls/cert.pem"; h.keyfile = "/tls/key.pem"
h.tls_control_required = True; h.tls_data_required = True
h.masquerade_address = os.environ["TS_HOST"]
h.passive_ports = range(int(os.environ["LO"]), int(os.environ["HI"]) + 1)
FTPServer(("0.0.0.0", int(os.environ["PORT"])), h).serve_forever()
