# SMB discovery routes: host, port and share rules

Applies to `POST /api/v1/smb/discover`, `/smb/test`, `/smb/browse` and `/smb/probe` (admin only). Credentials come from the request.

## Host rules (one shared check, `services.VetSMBHostIPs` / `VetSMBHost`)
- `host` must be an IPv4 literal or a plain DNS name (letters, digits, `-`, `_`, `.`, at most 253 characters). Brackets, colons (IPv6 literals, `host:port`), `@`, slashes, spaces and `%` give HTTP 400.
- The name is resolved once per request, EVERY resolved address is checked, and the connection is dialed to those vetted IPs (in resolver order, at most 4, on the validated port, sharing ONE connect deadline) - the name is never looked up a second time, so DNS rebinding between check and connect cannot redirect the dial (SMB-FU-9). The SMB session runs on that connection; the host name is not needed for NTLM. The services (not only the handlers) apply this check. A host that does not resolve (lookup error or empty answer) is refused with HTTP 400 "Host not allowed". This is a change from the earlier behaviour, which let unresolvable names through to the dial.
- Refused addresses: loopback, link-local, multicast, unspecified, 0.0.0.0/8, 255.255.255.255, cloud metadata addresses (169.254.169.254, 100.100.100.200, 168.63.129.16, fd00:ec2::254), and IPv6 forms that embed a refused IPv4 address (IPv4-mapped, IPv4-compatible, SIIT `::ffff:0:a.b.c.d`, NAT64 `64:ff9b::/96`, 6to4 `2002::/16`); local-use NAT64 `64:ff9b:1::/48` is refused as a whole. RFC1918 ranges (10.x, 172.16/12, 192.168.x) and CGNAT/Tailscale 100.64.0.0/10 stay allowed (operator decision 2026-11-11); only the Alibaba metadata address 100.100.100.200 inside that range is refused.

## Port rules
- Default allowed port: 445 (an omitted or 0 port means 445).
- `CATALOG_SMB_ALLOWED_PORTS` adds ports: a comma list, each 1-65535, for example `445,1445`. Any invalid entry makes the whole value invalid; that value is then dropped (445 and the configured roots stay) and one warning is logged per distinct invalid value.
- A non-default port of a configured SMB storage root (`config.SMB.Hosts[]`, port 1-65535) is allowed ONLY for that root's host (same name, or a common resolved address); it is not opened for any other host (SMB-FU-12). Ports from `CATALOG_SMB_ALLOWED_PORTS` stay global. Malformed entries are ignored. The configuration is read once at start-up: a root created at runtime needs a restart before discovery accepts its non-default port.

## Share and path rules
- `share` is `share` or `share/sub/dir` (`smb.SplitShare`). Administrative shares (IPC$, ADMIN$, C$ and other drive shares), `..`, backslashes and UNC spellings give HTTP 400. Only the share name is mounted; the sub-directory is enforced by `confinedShare`.
- `path` may not contain `..` segments, a `:` in any segment (alternate data streams), a segment made only of dots and spaces (other than `.`), or a Windows device name (CON, PRN, AUX, NUL, CONIN$, CONOUT$, CLOCK$, COM0-9, LPT0-9, COM/LPT with superscript 1-3; with or without extension) as any segment (SMB-FU-10).
- Discovery lists only shares that SplitShare accepts.

## Errors
Client input errors are HTTP 400 with a fixed message. Connection, mount and read failures are HTTP 500 with a fixed message; details go to the server log only.

Open items: see SMB-FU-11, SMB-FU-13 and later in `FOLLOW_UPS_NEXT_PHASE.md`.
