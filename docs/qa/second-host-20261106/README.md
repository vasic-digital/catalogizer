# Second-source rate-limit bucket check, stack 3, 2026-11-06 (scratch branch, not pushed)
Method: no physically distinct LAN host was available (Anton.local IS this host; LAN neighbours are Android phones/router, ssh closed).
Substitute: a temporary secondary address 192.168.1.250/32 on enp5s0 (sudo `ip addr add`, removed afterwards: `ip addr del 192.168.1.250/32 dev enp5s0`),
curl --interface 192.168.1.250 -> 192.168.1.115:33080. run_bucket.sh floods 35 bad logins from one source and probes the other, both directions; 3 runs (run_all.txt).
Result: 3/3 RESULT_PASS: each source keeps its own bucket; the API saw both 169.254.1.2 (primary host address, pasta alias) and 192.168.1.250 (api_seen_client_ips.txt).
Limits: the packets still traverse the local kernel (loopback), not a wire from another machine; proves pasta/nginx/TRUSTED_PROXIES preserve a non-primary source address, not a real remote NIC path.
No secrets in this directory.
