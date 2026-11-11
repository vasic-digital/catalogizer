package providers

import (
	"sync"
	"time"
)

// ProviderCooldown tracks temporary failures per provider and enforces
// a cooldown period before the provider can be used again.
type ProviderCooldown struct {
	mu        sync.RWMutex
	failures  map[string]time.Time // provider name -> last failure time
	cooldown  time.Duration
}

// NewProviderCooldown creates a cooldown tracker with the given duration.
func NewProviderCooldown(cooldown time.Duration) *ProviderCooldown {
	return &ProviderCooldown{
		failures: make(map[string]time.Time),
		cooldown: cooldown,
	}
}

// RecordFailure marks a provider as failed at the current time.
func (pc *ProviderCooldown) RecordFailure(provider string) {
	if pc == nil {
		return
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.failures[provider] = time.Now()
}

// IsOnCooldown returns true if the provider failed recently and the
// cooldown period has not yet elapsed.
func (pc *ProviderCooldown) IsOnCooldown(provider string) bool {
	if pc == nil {
		return false
	}
	pc.mu.RLock()
	lastFailure, ok := pc.failures[provider]
	pc.mu.RUnlock()
	if !ok {
		return false
	}
	return time.Since(lastFailure) < pc.cooldown
}

// Clear removes a provider from the cooldown list.
func (pc *ProviderCooldown) Clear(provider string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.failures, provider)
}
