package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotFiveHourLimitRoundtrip(t *testing.T) {
	groupID := int64(51)
	limit := 1.25
	apiKey := &APIKey{
		ID:      83,
		UserID:  41,
		GroupID: &groupID,
		Name:    "five-hour-auth-roundtrip",
		Status:  StatusActive,
		User: &User{
			ID:          41,
			Email:       "five-hour@test.local",
			Status:      StatusActive,
			Concurrency: 5,
		},
		Group: &Group{
			ID:               groupID,
			Name:             "five-hour-roundtrip",
			Platform:         PlatformAnthropic,
			Status:           StatusActive,
			Hydrated:         true,
			RateMultiplier:   1,
			SubscriptionType: SubscriptionTypeSubscription,
			FiveHourLimitUSD: &limit,
		},
	}

	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.NotNil(t, snapshot.Group)
	require.NotNil(t, snapshot.Group.FiveHourLimitUSD)

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	var restored APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &restored))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &restored)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.NotNil(t, materialized.Group.FiveHourLimitUSD)
	require.InDelta(t, limit, *materialized.Group.FiveHourLimitUSD, 1e-12)
}

func TestAPIKeyService_RejectsV25SnapshotWithoutFiveHourLimit(t *testing.T) {
	svc := &APIKeyService{}
	materialized, used, err := svc.applyAuthCacheEntry("sk-v25", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{Version: 25},
	})
	require.NoError(t, err)
	require.False(t, used, "v25 auth snapshots must be evicted after five_hour_limit_usd was added")
	require.Nil(t, materialized)
}
