package netguard

// SECQ-1 A round-3 follow-ups S1A3-1 (positive control for the non-embedded IPv6 path of allowListedUnwrapped) and the
// characterisation tests that pin the residuals documented for S1A3-2 (NAT64 with a network-specific prefix) and S1A3-3
// (ISATAP interface identifiers). A change that closes a residual must flip these tests and the documentation together.

import (
	"bytes"
	"go/format"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// S1A3-1: under DENY_PRIVATE an allow-listed IPv6 private address that carries NO embedded IPv4 (ULA, ::1) is admitted,
// a ULA outside the allow-list is still refused. Kills the mutant `return allowListed(ip)` -> `return false`.
func TestS1A3_1_AllowListedNonEmbeddedIPv6AdmittedUnderDenyPrivate(t *testing.T) {
	for _, c := range []struct{ entry, in, out string }{
		{"fd12:3456::/32", "fd12:3456::5", "fd12:9999::5"}, // ULA
		{"::1/128", "::1", "fd12:3456::5"},                 // loopback, re-admitted by allow-list
	} {
		envClean(t)
		t.Setenv(DenyPrivateEnv, "1")
		t.Setenv(AllowCIDRsEnv, c.entry)
		in, out := net.ParseIP(c.in), net.ParseIP(c.out)
		if got := allowListedUnwrapped(in); !got {
			t.Errorf("%s: allowListedUnwrapped(%s) = false, want true", c.entry, c.in)
		}
		if !permitted(in, ClassPrivate, true) || IsBlockedOperatorIP(in) {
			t.Errorf("%s: allow-listed %s refused under DENY_PRIVATE", c.entry, c.in)
		}
		if allowListedUnwrapped(out) || permitted(out, ClassPrivate, true) || !IsBlockedOperatorIP(out) {
			t.Errorf("%s: control: %s outside the allow-list admitted under DENY_PRIVATE", c.entry, c.out)
		}
		// control: without DENY_PRIVATE a ULA is private-class and admitted either way
		t.Setenv(DenyPrivateEnv, "")
		if c.in != "::1" && IsBlockedOperatorIP(net.ParseIP("fd12:9999::5")) {
			t.Errorf("control: ULA refused without DENY_PRIVATE")
		}
	}
}

// S1A3-2: NAT64 with an operator network-specific prefix (RFC 6052 section 2.2, e.g. 2001:db8:64::/96 style, here a
// global-unicast stand-in) cannot be recognised without the prefix being configured: the address is public-class.
// Documented residual; operators with such a gateway must keep the embedded private ranges unreachable at the gateway.
func TestS1A3_2_NetworkSpecificNAT64PrefixIsPublicResidual(t *testing.T) {
	envClean(t)
	for _, s := range []string{"2a00:1450:64::c0a8:105", "2a00:1450:64::7f00:1"} {
		if got := ClassOf(net.ParseIP(s)); got != ClassPublic {
			t.Errorf("ClassOf(%s) = %v, documented residual (S1A3-2) is ClassPublic", s, got)
		}
	}
	// control: the well-known prefix IS unwrapped
	if got := ClassOf(net.ParseIP("64:ff9b::c0a8:105")); got != ClassPrivate {
		t.Errorf("control: 64:ff9b::c0a8:105 = %v, want ClassPrivate", got)
	}
	if got := ClassOf(net.ParseIP("64:ff9b::7f00:1")); got != ClassNever {
		t.Errorf("control: 64:ff9b::7f00:1 = %v, want ClassNever", got)
	}
}

// S1A3-3: ISATAP interface identifiers (::0:5efe:a.b.c.d / ::200:5efe:a.b.c.d) under a global prefix are public-class.
func TestS1A3_3_ISATAPInterfaceIDIsPublicResidual(t *testing.T) {
	envClean(t)
	for _, s := range []string{"2a00:1450::5efe:c0a8:105", "2a00:1450::200:5efe:c0a8:105", "2a00:1450::5efe:7f00:1"} {
		if got := ClassOf(net.ParseIP(s)); got != ClassPublic {
			t.Errorf("ClassOf(%s) = %v, documented residual (S1A3-3) is ClassPublic", s, got)
		}
	}
	if got := ClassOf(net.ParseIP("2a00:1450::1")); got != ClassPublic {
		t.Errorf("control: plain global IPv6 = %v, want ClassPublic", got)
	}
}

// S1A3-4: every source file of the package is gofmt-clean (a lint-class regression guard; formatting drift in the
// reviewed security package is a review-noise defect).
func TestS1A3_4_PackageSourcesAreGofmtClean(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no source files found (glob err=%v): the check would be blind", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out, err := format.Source(src)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if !bytes.Equal(src, out) {
			t.Errorf("%s is not gofmt-clean", f)
		}
	}
}

// OBF-3: the claims of docs/ENV_VARIABLES.md about CATALOGIZER_OUTBOUND_ALLOW_CIDRS and CATALOGIZER_OPERATOR_DENY_PRIVATE,
// checked against the code: which client classes honour the allow-list, which addresses it can never re-admit, and what
// the operator class does with loopback under DENY_PRIVATE.
func TestOBF3_AllowListDocumentedBehaviourMatchesCode(t *testing.T) {
	envClean(t)
	t.Setenv(AllowCIDRsEnv, "fd00:ec2::/32,fd20:ce::/32,100.64.0.0/10,168.63.129.0/24,169.254.0.0/16,127.0.0.0/8,192.168.1.0/24")
	meta := []string{"fd00:ec2::254", "fd20:ce::254", "100.100.100.200"}
	for _, s := range meta {
		ip := net.ParseIP(s)
		if !IsBlockedOperatorIP(ip) {
			t.Errorf("%s: operator class must refuse the metadata address even when an entry covers it", s)
		}
		if IsBlockedIPStrict(ip) == false {
			t.Errorf("%s: STRICT class must ignore the allow-list", s)
		}
		// documented residual of the sync class (netguard.Transport): a covering entry re-admits it
		if IsBlockedIP(ip) {
			t.Errorf("%s: sync class unexpectedly refuses it; docs/ENV_VARIABLES.md documents the re-admission, change both together", s)
		}
	}
	// the surrounding ULA space is not metadata
	if IsBlockedOperatorIP(net.ParseIP("fd00:ec2::1")) {
		t.Error("fd00:ec2::1 is ordinary ULA space and must be admitted by the covering entry")
	}
	// never re-admitted by any class, whatever the allow-list says
	for _, s := range []string{"169.254.169.254", "168.63.129.16", "fe80::1", "0.0.0.0", "224.0.0.1", "255.255.255.255"} {
		ip := net.ParseIP(s)
		if !IsBlockedIP(ip) || !IsBlockedIPStrict(ip) || !IsBlockedOperatorIP(ip) {
			t.Errorf("%s must be refused by every class even with a covering allow-list", s)
		}
	}
	// sync class honours the allow-list, STRICT ignores it
	lan := net.ParseIP("192.168.1.5")
	if IsBlockedIP(lan) || !IsBlockedIPStrict(lan) {
		t.Errorf("192.168.1.5: sync class must honour the allow-list (blocked=%v) and STRICT must ignore it (blocked=%v)", IsBlockedIP(lan), IsBlockedIPStrict(lan))
	}
}

func TestOBF3_DenyPrivateLoopbackOnOperatorPathDocumented(t *testing.T) {
	for _, deny := range []string{"", "1"} {
		envClean(t)
		t.Setenv(DenyPrivateEnv, deny)
		lo := net.ParseIP("127.0.0.1")
		if !IsBlockedOperatorIP(lo) {
			t.Errorf("deny=%q: loopback must be refused by default on the operator path", deny)
		}
		t.Setenv(AllowCIDRsEnv, "127.0.0.1/32")
		if IsBlockedOperatorIP(lo) {
			t.Errorf("deny=%q: allow-listed loopback must be re-admitted on the operator path (docs say refused-by-default, not never)", deny)
		}
	}
}
