// Package realfs holds the real-service tests of the generic scanner (PA-03, WP-12): the scanner and the settings contract against the
// pure-ftpd and WebDAV containers of docker-compose.test-infra.yml. The tests carry the build tag `realfs` and FAIL (never skip) when the
// stack environment is missing, so a green run is always a run against real servers. Run through scripts/test-infra/run_client.sh, see
// docs/scripts/generic_scanner.md.
package realfs
