package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fiveHourBillingCacheFake struct {
	BillingCache
	fiveHourUsage float64
}

func (f *fiveHourBillingCacheFake) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	return &SubscriptionCacheData{
		Status:        SubscriptionStatusActive,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		FiveHourUsage: f.fiveHourUsage,
	}, nil
}

func TestCheckSubscriptionEligibility_UsesFreshRedisFiveHourUsage(t *testing.T) {
	limit := 1.0
	group := &Group{ID: 91, SubscriptionType: SubscriptionTypeSubscription, FiveHourLimitUSD: &limit}

	cases := []struct {
		name        string
		redisUsage  float64
		snapshotUse float64
		wantErr     bool
	}{
		{name: "redis usage already exhausted while L1 snapshot is stale", redisUsage: 1.01, snapshotUse: 0, wantErr: true},
		{name: "exact limit is exhausted without a snapshot", redisUsage: 1, snapshotUse: 0, wantErr: true},
		{name: "both snapshots below limit", redisUsage: 0.2, snapshotUse: 0.5, wantErr: false},
		{name: "snapshot is fresher than redis", redisUsage: 0.2, snapshotUse: 1.01, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &BillingCacheService{cache: &fiveHourBillingCacheFake{fiveHourUsage: tc.redisUsage}}
			sub := &UserSubscription{FiveHourUsageUSD: tc.snapshotUse}
			err := svc.checkSubscriptionEligibility(context.Background(), 7, group, sub)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrFiveHourLimitExceeded)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
