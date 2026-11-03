package modules

import (
	"context"
	"testing"
	"time"

	"digital.vasic.memory/pkg/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ModuleInfo tests
// ---------------------------------------------------------------------------

func TestModuleInfo_Fields(t *testing.T) {
	info := ModuleInfo{
		Name:        "digital.vasic.database",
		Description: "Generic connection pool",
		Initialized: true,
	}
	assert.Equal(t, "digital.vasic.database", info.Name)
	assert.Equal(t, "Generic connection pool", info.Description)
	assert.True(t, info.Initialized)
}

// ---------------------------------------------------------------------------
// RegisterModules tests
// ---------------------------------------------------------------------------

func TestRegisterModules_ReturnsNonNil(t *testing.T) {
	reg := RegisterModules()
	require.NotNil(t, reg, "RegisterModules must return a non-nil *Registry")
}

func TestRegisterModules_AllModulesRegistered(t *testing.T) {
	reg := RegisterModules()

	// The function registers exactly 12 modules
	assert.Equal(t, 12, len(reg.Modules),
		"expected 12 modules to be registered")
}

func TestRegisterModules_AllModulesInitialized(t *testing.T) {
	reg := RegisterModules()

	for _, mod := range reg.Modules {
		assert.True(t, mod.Initialized,
			"module %q should be marked as initialized", mod.Name)
		assert.NotEmpty(t, mod.Name,
			"module Name should not be empty")
		assert.NotEmpty(t, mod.Description,
			"module %q Description should not be empty", mod.Name)
	}
}

func TestRegisterModules_UniqueModuleNames(t *testing.T) {
	reg := RegisterModules()

	seen := make(map[string]bool)
	for _, mod := range reg.Modules {
		assert.False(t, seen[mod.Name],
			"duplicate module name: %s", mod.Name)
		seen[mod.Name] = true
	}
}

func TestRegisterModules_ExpectedModuleNames(t *testing.T) {
	reg := RegisterModules()

	expectedModules := []string{
		"digital.vasic.database",
		"digital.vasic.lazy",
		"digital.vasic.media",
		"digital.vasic.memory",
		"digital.vasic.middleware",
		"digital.vasic.observability",
		"digital.vasic.ratelimiter",
		"digital.vasic.recovery",
		"digital.vasic.security",
		"digital.vasic.storage",
		"digital.vasic.streaming",
		"digital.vasic.watcher",
	}

	names := make(map[string]bool)
	for _, mod := range reg.Modules {
		names[mod.Name] = true
	}

	for _, expected := range expectedModules {
		assert.True(t, names[expected],
			"expected module %q to be registered", expected)
	}
}

// ---------------------------------------------------------------------------
// Registry field tests
// ---------------------------------------------------------------------------

func TestRegisterModules_HealthAggregator(t *testing.T) {
	reg := RegisterModules()
	assert.NotNil(t, reg.HealthAggregator,
		"HealthAggregator should be initialized")
}

func TestRegisterModules_MemoryMonitor(t *testing.T) {
	reg := RegisterModules()
	assert.NotNil(t, reg.MemoryMonitor,
		"MemoryMonitor should be initialized")
}

func TestRegisterModules_ResilienceFacade(t *testing.T) {
	reg := RegisterModules()
	assert.NotNil(t, reg.ResilienceFacade,
		"ResilienceFacade should be initialized")
}

func TestRegisterModules_GuardrailEngine(t *testing.T) {
	reg := RegisterModules()
	assert.NotNil(t, reg.GuardrailEngine,
		"GuardrailEngine should be initialized")
}

func TestRegisterModules_StorageResolver(t *testing.T) {
	reg := RegisterModules()
	assert.NotNil(t, reg.StorageResolver,
		"StorageResolver should be initialized")
}

func TestRegisterModules_TransportFactory(t *testing.T) {
	reg := RegisterModules()
	assert.NotNil(t, reg.TransportFactory,
		"TransportFactory should be initialized")
}

func TestRegisterModules_MediaDetector(t *testing.T) {
	reg := RegisterModules()
	assert.NotNil(t, reg.MediaDetector,
		"MediaDetector should be initialized")
}

// ---------------------------------------------------------------------------
// GetModuleInfo tests
// ---------------------------------------------------------------------------

func TestGetModuleInfo_ReturnsCopy(t *testing.T) {
	reg := RegisterModules()

	info1 := reg.GetModuleInfo()
	info2 := reg.GetModuleInfo()

	// Both should have same content
	assert.Equal(t, len(info1), len(info2))
	for i := range info1 {
		assert.Equal(t, info1[i].Name, info2[i].Name)
	}

	// Modifying one should not affect the other (copy semantics)
	if len(info1) > 0 {
		info1[0].Name = "modified"
		assert.NotEqual(t, info1[0].Name, info2[0].Name,
			"GetModuleInfo should return a copy, not the original slice")
	}
}

func TestGetModuleInfo_MatchesModules(t *testing.T) {
	reg := RegisterModules()

	info := reg.GetModuleInfo()
	assert.Equal(t, len(reg.Modules), len(info))

	for i, mod := range reg.Modules {
		assert.Equal(t, mod.Name, info[i].Name)
		assert.Equal(t, mod.Description, info[i].Description)
		assert.Equal(t, mod.Initialized, info[i].Initialized)
	}
}

// ---------------------------------------------------------------------------
// StartBackgroundServices tests
// ---------------------------------------------------------------------------

func TestStartBackgroundServices(t *testing.T) {
	reg := RegisterModules()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not panic
	reg.StartBackgroundServices(ctx)

	// Give background services a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context to stop background services
	cancel()

	// Allow goroutines to finish
	time.Sleep(10 * time.Millisecond)
}

func TestStartBackgroundServices_NilMemoryMonitor(t *testing.T) {
	reg := &Registry{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not panic with nil MemoryMonitor
	reg.StartBackgroundServices(ctx)
}

// ---------------------------------------------------------------------------
// Stop tests
// ---------------------------------------------------------------------------

func TestStop(t *testing.T) {
	reg := RegisterModules()

	// Start background services
	ctx, cancel := context.WithCancel(context.Background())
	reg.StartBackgroundServices(ctx)

	time.Sleep(10 * time.Millisecond)
	cancel()

	// Stop should not panic
	reg.Stop()
}

func TestStop_NilFields(t *testing.T) {
	reg := &Registry{}

	// Should not panic with nil fields
	reg.Stop()
}

func TestStop_Idempotent(t *testing.T) {
	reg := RegisterModules()

	// Double stop should not panic
	reg.Stop()
	reg.Stop()
}

// ---------------------------------------------------------------------------
// registryLogger tests
// ---------------------------------------------------------------------------

func TestRegistryLogger_Info(t *testing.T) {
	l := &registryLogger{}
	// Should not panic
	l.Info("test message", "key", "value")
}

func TestRegistryLogger_Warn(t *testing.T) {
	l := &registryLogger{}
	// Should not panic
	l.Warn("warning message", "key", "value")
}

func TestRegistryLogger_Debug(t *testing.T) {
	l := &registryLogger{}
	// Should not panic (debug is suppressed)
	l.Debug("debug message", "key", "value")
}

func TestClassifyMemoryAlert(t *testing.T) {
	// S1: the observed false positive - heap ratio 1.45, 205 goroutines (flagged only by the goroutine criterion)
	got := classifyMemoryAlert(memory.LeakReport{HeapGrowthRatio: 1.45, GoroutineCount: 205, InitialGoroutines: 29}, memoryHeapThresholdRatio)
	assert.Equal(t, memoryTriggerGoroutines, got)
	// a real heap blow-up stays an alert
	got = classifyMemoryAlert(memory.LeakReport{HeapGrowthRatio: 12.5, GoroutineCount: 30, InitialGoroutines: 29}, memoryHeapThresholdRatio)
	assert.Equal(t, memoryTriggerHeap, got)
	// boundary: exactly the threshold is not above it
	got = classifyMemoryAlert(memory.LeakReport{HeapGrowthRatio: memoryHeapThresholdRatio}, memoryHeapThresholdRatio)
	assert.Equal(t, memoryTriggerGoroutines, got)
}
