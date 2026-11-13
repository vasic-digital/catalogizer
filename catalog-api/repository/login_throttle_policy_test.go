package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// S3: the account window can never exceed the stale-row purge window (a purged row restarts the count), so it is clamped.
func TestLoginThrottlePolicy_AccountDecayIsClampedToStaleAfter(t *testing.T) {
	p := LoginThrottlePolicy{AccountDecay: 26 * time.Hour}.withDefaults()
	assert.Equal(t, 24*time.Hour, p.AccountDecay, "26h AccountDecay with the default 24h StaleAfter is clamped to 24h")
	p = LoginThrottlePolicy{AccountDecay: 2 * time.Hour, StaleAfter: 6 * time.Hour}.withDefaults()
	assert.Equal(t, 2*time.Hour, p.AccountDecay, "a shorter AccountDecay is kept")
	p = LoginThrottlePolicy{AccountDecay: 48 * time.Hour, StaleAfter: 72 * time.Hour}.withDefaults()
	assert.Equal(t, 48*time.Hour, p.AccountDecay, "a longer StaleAfter keeps the longer AccountDecay")
}

// S6b: an over-long address key is cut to 64 characters (not fewer); real IPv6 text is at most 45 characters and is never cut.
func TestIPScope_KeepsUpTo64Characters(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += string(rune('a' + i%26))
	}
	assert.Equal(t, "ip:"+long[:64], ipScope(long))
	assert.Equal(t, "ip:2001:db8:1234:5678:9abc:def0:1234:5678", ipScope("2001:db8:1234:5678:9abc:def0:1234:5678"))
}
