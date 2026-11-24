package securityaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultChunkCacheTTL bounds how long a scanned chunk verdict is reused in
	// Redis for incremental agentic auditing.
	DefaultChunkCacheTTL = 12 * time.Hour

	chunkCacheKeyPrefix = "prompt_audit:chunk_cache:v2:"
)

// ChunkResultCache stores normalized per-chunk scan results keyed by content
// hash, guard model set, and enabled scanner policy so multi-turn agentic
// requests only scan newly added chunks.
type ChunkResultCache interface {
	GetChunkResult(ctx context.Context, key string) (*NormalizedResult, bool, error)
	SetChunkResult(ctx context.Context, key string, result *NormalizedResult, ttl time.Duration) error
}

// BuildChunkCacheKey derives a deterministic cache key for a chunk under the
// active guard model(s) and enabled scanners. Switching guard models (e.g.
// 0.6b -> 8b) or changing enabled scanners automatically invalidates prior
// cache entries.
func BuildChunkCacheKey(chunk string, endpoints []ActiveEndpoint, enabledScanners []string) string {
	modelSet := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		model := strings.TrimSpace(ep.Model)
		if model != "" {
			modelSet[model] = struct{}{}
		}
	}
	models := sortedKeys(modelSet)

	scannerSet := make(map[string]struct{}, len(enabledScanners))
	for _, s := range enabledScanners {
		normalized := NormalizeCategory(s)
		if normalized != "" {
			scannerSet[normalized] = struct{}{}
		}
	}
	scanners := sortedKeys(scannerSet)
	sort.Strings(models)

	hasher := sha256.New()
	_, _ = hasher.Write([]byte(strings.Join(models, ",")))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(strings.Join(scanners, ",")))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(chunk))
	return chunkCacheKeyPrefix + hex.EncodeToString(hasher.Sum(nil))
}

func (s *RedisPayloadStore) GetChunkResult(ctx context.Context, key string) (*NormalizedResult, bool, error) {
	if s == nil || s.client == nil || strings.TrimSpace(key) == "" {
		return nil, false, nil
	}
	raw, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var result NormalizedResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false, err
	}
	return cloneNormalizedResult(&result), true, nil
}

func (s *RedisPayloadStore) SetChunkResult(ctx context.Context, key string, result *NormalizedResult, ttl time.Duration) error {
	if s == nil || s.client == nil || strings.TrimSpace(key) == "" || result == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = DefaultChunkCacheTTL
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, encoded, ttl).Err()
}

func cloneNormalizedResult(src *NormalizedResult) *NormalizedResult {
	if src == nil {
		return nil
	}
	cp := *src
	if src.Categories != nil {
		cp.Categories = append([]string(nil), src.Categories...)
	}
	if src.MatchedScanners != nil {
		cp.MatchedScanners = append([]string(nil), src.MatchedScanners...)
	}
	if src.UnknownCategories != nil {
		cp.UnknownCategories = append([]string(nil), src.UnknownCategories...)
	}
	if src.ScannerScores != nil {
		cp.ScannerScores = make(map[string]float64, len(src.ScannerScores))
		for k, v := range src.ScannerScores {
			cp.ScannerScores[k] = v
		}
	}
	if src.ScannerEvidence != nil {
		cp.ScannerEvidence = make(map[string]string, len(src.ScannerEvidence))
		for k, v := range src.ScannerEvidence {
			cp.ScannerEvidence[k] = v
		}
	}
	return &cp
}
