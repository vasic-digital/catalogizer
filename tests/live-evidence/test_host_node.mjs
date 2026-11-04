// Host handling of the Node listener lookup (constitution 11.4.289(D), gate-logic UNIT test): 'localhost' maps to 127.0.0.1, an exact address finds its listener,
// a bind to another address / an unresolvable named host finds NOTHING (fail closed), the empty host is "any address". Known limit (follow-up): [::1]-only servers.
import assert from 'node:assert/strict';
import net from 'node:net';
import { findListener } from './index.mjs';
const srv = net.createServer(); await new Promise((r) => srv.listen(0, '127.0.0.1', r));
const port = srv.address().port; let n = 0;
try {
  for (const h of ['', '127.0.0.1', 'localhost']) { const a = findListener(port, h); assert.ok(a && a.pid === process.pid, `host '${h}' finds this process`); n++; }
  for (const h of ['10.255.255.1', 'no-such-host.invalid', '::1']) assert.equal(findListener(port, h), null, `host '${h}' must not find the 127.0.0.1 listener`), n++;
  assert.equal(findListener(port + 1 > 65535 ? 1 : port + 1, ''), null, 'control: an unused port finds nothing'); n++;
} finally { srv.close(); }
console.log(`node host tests: ${n} passed`);
