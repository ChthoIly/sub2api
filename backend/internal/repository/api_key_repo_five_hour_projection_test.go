package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// GetByKeyForAuth uses an explicit group projection for the auth hot path.
// This regression test guards the five-hour subscription limit column, whose
// omission made the limit invisible to both the auth middleware and billing
// checks even though it was stored in PostgreSQL.
func TestAPIKeyRepository_GetByKeyForAuth_PreservesFiveHourLimit(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-five-hour@test.com")

	group, err := client.Group.Create().
		SetName("g-auth-five-hour-unit").
		SetPlatform(service.PlatformAnthropic).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		SetRateMultiplier(1).
		SetFiveHourLimitUsd(1.25).
		Save(ctx)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-getbykey-auth-five-hour-unit",
		Name:    "Five Hour Projection Key",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, got.Group)
	require.NotNil(t, got.Group.FiveHourLimitUSD, "five_hour_limit_usd must be selected by GetByKeyForAuth")
	require.InDelta(t, 1.25, *got.Group.FiveHourLimitUSD, 1e-9)
}
