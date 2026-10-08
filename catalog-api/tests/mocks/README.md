# Mock Servers

**DEPRECATED FOR INTEGRATION TESTS**

These mock servers should ONLY be used for unit tests.

For integration tests, use the real containerized services of the per-run test infrastructure (`docker-compose.test-infra.yml`, started by `scripts/test-infra/up.sh`; see `docs/testing/real-service-stack.md`):

```bash
# Set environment variables for integration tests
scripts/test-infra/up.sh --build-id <id>                       # real services, random loopback ports, generated credentials
export CATALOGIZER_TEST_INFRA_ENV="$PWD/.audit/test-infra/catalogizer-test-<id>/env"
# an explicit *_TEST_SERVER (host or host:port) and *_TEST_USER / *_TEST_PASS still override a single value; NFS has no host port: NFS_TEST_SERVER names it
```

The integration tests in `tests/integration/protocol_connectivity_test.go` read the ports and credentials from that per-run env file (no fixed port, no literal credential) and SKIP when nothing is configured.

## Mock Files (Unit Tests Only)

- `ftp_mock_server.go` - Mock FTP server for unit testing
- `smb_mock_server.go` - Mock SMB server for unit testing  
- `webdav_mock_server.go` - Mock WebDAV server for unit testing
- `nfs_mock_server.go` - Mock NFS server for unit testing

**Do not use these in integration tests or challenges.**
