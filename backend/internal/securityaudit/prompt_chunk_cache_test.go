package securityaudit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memoryChunkCache struct {
	mu      sync.Mutex
	entries map[string]*NormalizedResult
}

func newMemoryChunkCache() *memoryChunkCache {
	return &memoryChunkCache{entries: map[string]*NormalizedResult{}}
}

func (m *memoryChunkCache) GetChunkResult(_ context.Context, key string) (*NormalizedResult, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res, ok := m.entries[key]
	if !ok {
		return nil, false, nil
	}
	return cloneNormalizedResult(res), true, nil
}

func (m *memoryChunkCache) SetChunkResult(_ context.Context, key string, result *NormalizedResult, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = cloneNormalizedResult(result)
	return nil
}

func TestBuildChunkCacheKeyInvalidatesOnModelOrScannerChange(t *testing.T) {
	epSmall := []ActiveEndpoint{{ID: "ep-1", Model: "sileader/qwen3guard:0.6b"}}
	epLarge := []ActiveEndpoint{{ID: "ep-1", Model: "qwen3guard:8b"}}

	k1 := BuildChunkCacheKey("hello world", epSmall, []string{"pii", "jailbreak"})
	k2 := BuildChunkCacheKey("hello world", epSmall, []string{"jailbreak", "PII"})
	k3 := BuildChunkCacheKey("hello world", epLarge, []string{"pii", "jailbreak"})
	k4 := BuildChunkCacheKey("hello world", epSmall, []string{"pii"})
	k5 := BuildChunkCacheKey("different text", epSmall, []string{"pii", "jailbreak"})

	require.Equal(t, k1, k2, "scanner ordering/casing should produce the same key")
	require.NotEqual(t, k1, k3, "switching guard model must invalidate cache")
	require.NotEqual(t, k1, k4, "changing enabled scanners must invalidate cache")
	require.NotEqual(t, k1, k5, "different chunk content must produce different key")
}

type cacheTestScannerFunc func(ctx context.Context, endpoint ActiveEndpoint, chunk string, enabledScanners []string) (*NormalizedResult, error)

func (f cacheTestScannerFunc) Scan(ctx context.Context, endpoint ActiveEndpoint, chunk string, enabledScanners []string) (*NormalizedResult, error) {
	return f(ctx, endpoint, chunk, enabledScanners)
}

func TestGuardEvaluatorReusesCachedPrefixChunksAcrossTurns(t *testing.T) {
	var calls atomic.Int32
	scanner := cacheTestScannerFunc(func(_ context.Context, ep ActiveEndpoint, chunk string, _ []string) (*NormalizedResult, error) {
		calls.Add(1)
		return &NormalizedResult{
			Decision: EventPass, RiskLevel: RiskLow, Action: ActionAllow, Safety: "Safe",
			GuardEndpointID: ep.ID, ScannerVersion: ep.Model, PolicyID: "priority", PolicyVersion: 1,
		}, nil
	})
	cache := newMemoryChunkCache()
	evaluator := NewGuardEvaluator(scanner, nil, nil)
	evaluator.SetChunkCache(cache)

	cfg := ActiveConfig{
		Enabled: true, BlockingEnabled: true, Scanners: AllScannerIDs,
		Endpoints: []ActiveEndpoint{{ID: "ep-1", Model: "qwen3guard:8b", InputLimit: 100, TimeoutMS: 3000, Enabled: true}},
	}
	sharedPrefix := "System prompt prefix " +
		"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefg"
	require.Equal(t, 100, len(sharedPrefix))

	// Turn 1: latest user + shared prefix (2 chunks). Both miss cache.
	turn1 := PromptSnapshot{ScanText: "turn 1 user" + promptAuditPrioritySeparator + sharedPrefix, PromptLength: len(sharedPrefix) + 11}
	decision1, err := evaluator.Evaluate(context.Background(), cfg, turn1)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, decision1.Kind)
	require.Equal(t, int32(2), calls.Load())

	// Turn 2: new latest user + same shared prefix. Prefix chunk hits cache; only latest user is scanned.
	turn2 := PromptSnapshot{ScanText: "turn 2 user" + promptAuditPrioritySeparator + sharedPrefix, PromptLength: len(sharedPrefix) + 11}
	decision2, err := evaluator.Evaluate(context.Background(), cfg, turn2)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, decision2.Kind)
	require.Equal(t, int32(3), calls.Load(), "shared prefix chunk should be served from cache")

	// Turn 3: new latest user + shared prefix + appended history turn (3 chunks:
	// latest user, cached 100-rune prefix, new tail). Only latest user + new tail scan.
	turn3 := PromptSnapshot{
		ScanText:     "turn 3 user" + promptAuditPrioritySeparator + sharedPrefix + "\n\nturn 2 assistant output",
		PromptLength: len(sharedPrefix) + 36,
	}
	decision3, err := evaluator.Evaluate(context.Background(), cfg, turn3)
	require.NoError(t, err)
	require.Equal(t, DecisionAllow, decision3.Kind)
	require.Equal(t, int32(5), calls.Load(), "100-rune prefix chunk should still hit cache when history grows")
}
