package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fiveHourResetRepoStub struct {
	userSubRepoNoop

	resetCalled   bool
	expectedStart *time.Time
	newStart      *time.Time
	sub           *UserSubscription
}

func (r *fiveHourResetRepoStub) ResetFiveHourUsage(_ context.Context, _ int64, expectedStart, newStart *time.Time) error {
	r.resetCalled = true
	r.expectedStart = expectedStart
	r.newStart = newStart
	if r.sub != nil {
		r.sub.FiveHourUsageUSD = 0
		r.sub.FiveHourWindowStart = newStart
	}
	return nil
}

func TestUserSubscriptionNeedsFiveHourResetAt(t *testing.T) {
	start := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	sub := &UserSubscription{FiveHourWindowStart: &start}

	require.False(t, sub.NeedsFiveHourResetAt(start.Add(5*time.Hour-time.Nanosecond)))
	require.True(t, sub.NeedsFiveHourResetAt(start.Add(5*time.Hour)))
}

func TestCheckAndResetWindows_ResetsExpiredFiveHour(t *testing.T) {
	now := time.Date(2026, 11, 5, 15, 0, 0, 0, time.UTC)
	start := now.Add(-5*time.Hour - time.Second)
	repo := &fiveHourResetRepoStub{sub: &UserSubscription{ID: 1, UserID: 2, GroupID: 3}}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	svc.now = func() time.Time { return now }
	sub := &UserSubscription{
		ID:                  1,
		UserID:              2,
		GroupID:             3,
		FiveHourWindowStart: &start,
		FiveHourUsageUSD:    12.5,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.True(t, repo.resetCalled)
	require.Equal(t, &start, repo.expectedStart)
	require.NotNil(t, repo.newStart)
	require.Equal(t, now, *repo.newStart)
	require.Equal(t, now, *sub.FiveHourWindowStart)
	require.Zero(t, sub.FiveHourUsageUSD)
}

func TestValidateAndCheckLimits_RejectsExceededFiveHour(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	limit := 10.0
	sub := &UserSubscription{
		Status:              SubscriptionStatusActive,
		StartsAt:            now.Add(-time.Hour),
		ExpiresAt:           now.Add(time.Hour),
		FiveHourWindowStart: &windowStart,
		FiveHourUsageUSD:    limit + 0.01,
	}
	group := &Group{FiveHourLimitUSD: &limit}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.False(t, needsMaintenance)
	require.ErrorIs(t, err, ErrFiveHourLimitExceeded)
}

func TestValidateAndCheckLimits_ResetsExpiredFiveHourBeforeCheck(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-5*time.Hour - time.Second)
	limit := 10.0
	sub := &UserSubscription{
		Status:              SubscriptionStatusActive,
		StartsAt:            now.Add(-24 * time.Hour),
		ExpiresAt:           now.Add(time.Hour),
		FiveHourWindowStart: &windowStart,
		FiveHourUsageUSD:    limit + 20,
	}
	group := &Group{FiveHourLimitUSD: &limit}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.NoError(t, err)
	require.True(t, needsMaintenance)
	require.Zero(t, sub.FiveHourUsageUSD)
}

func TestUserSubscriptionCheckFiveHourLimit(t *testing.T) {
	limit := 10.0
	group := &Group{FiveHourLimitUSD: &limit}

	require.True(t, (&UserSubscription{FiveHourUsageUSD: 9.99}).CheckFiveHourLimit(group, 0))
	require.True(t, (&UserSubscription{FiveHourUsageUSD: 10}).CheckFiveHourLimit(group, 0))
	require.False(t, (&UserSubscription{FiveHourUsageUSD: 10.01}).CheckFiveHourLimit(group, 0))

	unlimited := &Group{}
	require.True(t, (&UserSubscription{FiveHourUsageUSD: 1000}).CheckFiveHourLimit(unlimited, 0))
}

func TestValidateAndCheckLimits_RejectsExactFiveHourLimit(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	limit := 10.0
	sub := &UserSubscription{
		Status:              SubscriptionStatusActive,
		StartsAt:            now.Add(-time.Hour),
		ExpiresAt:           now.Add(time.Hour),
		FiveHourWindowStart: &windowStart,
		FiveHourUsageUSD:    limit,
	}
	group := &Group{FiveHourLimitUSD: &limit}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.False(t, needsMaintenance)
	require.ErrorIs(t, err, ErrFiveHourLimitExceeded)
}
