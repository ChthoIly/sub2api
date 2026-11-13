package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type windowStatsRecordingRepo struct {
	usageBatchLogRepoStub
	fiveHourStart time.Time
	sevenDayStart time.Time
	starts        []time.Time
}

func (r *windowStatsRecordingRepo) GetAccountWindowStats(_ context.Context, _ int64, start time.Time) (*usagestats.AccountStats, error) {
	r.starts = append(r.starts, start)
	switch {
	case start.Equal(r.fiveHourStart):
		return &usagestats.AccountStats{Requests: 5, Tokens: 50, Cost: 0.5, StandardCost: 0.5, UserCost: 0.4}, nil
	case start.Equal(r.sevenDayStart):
		return &usagestats.AccountStats{Requests: 7, Tokens: 70, Cost: 0.7, StandardCost: 0.7, UserCost: 0.6}, nil
	default:
		return &usagestats.AccountStats{}, nil
	}
}

func TestAddWindowStatsUsesSeparateFiveHourAndSevenDayWindows(t *testing.T) {
	now := time.Now()
	fiveHourStart := now.Add(-2 * time.Hour)
	fiveHourEnd := now.Add(3 * time.Hour)
	sevenDayReset := now.Add(24 * time.Hour)
	sevenDayStart := sevenDayReset.Add(-sevenDayUsageWindow)

	account := &Account{
		ID:                 42,
		SessionWindowStart: &fiveHourStart,
		SessionWindowEnd:   &fiveHourEnd,
	}
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 20},
		SevenDay: &UsageProgress{Utilization: 66, ResetsAt: &sevenDayReset},
	}
	repo := &windowStatsRecordingRepo{
		fiveHourStart: fiveHourStart,
		sevenDayStart: sevenDayStart,
	}
	svc := &AccountUsageService{usageLogRepo: repo, cache: NewUsageCache()}

	svc.addWindowStats(context.Background(), account, usage)

	require.Len(t, repo.starts, 2)
	require.Equal(t, fiveHourStart, repo.starts[0])
	require.Equal(t, sevenDayStart, repo.starts[1])
	require.NotNil(t, usage.FiveHour.WindowStats)
	require.NotNil(t, usage.SevenDay.WindowStats)
	require.Equal(t, int64(5), usage.FiveHour.WindowStats.Requests)
	require.Equal(t, int64(7), usage.SevenDay.WindowStats.Requests)
	require.InDelta(t, 0.7, usage.SevenDay.WindowStats.Cost, 1e-9)

	// The second call should reuse the per-account window stats cache.
	svc.addWindowStats(context.Background(), account, &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 20},
		SevenDay: &UsageProgress{Utilization: 66, ResetsAt: &sevenDayReset},
	})
	require.Len(t, repo.starts, 2)
}

func TestCurrentSevenDayWindowStartRequiresActiveReset(t *testing.T) {
	_, ok := currentSevenDayWindowStart(nil)
	require.False(t, ok)

	resetInPast := time.Now().Add(-time.Minute)
	_, ok = currentSevenDayWindowStart(&UsageProgress{ResetsAt: &resetInPast})
	require.False(t, ok)

	resetInFuture := time.Now().Add(time.Hour)
	start, ok := currentSevenDayWindowStart(&UsageProgress{ResetsAt: &resetInFuture})
	require.True(t, ok)
	require.Equal(t, resetInFuture.Add(-7*24*time.Hour), start)
}
