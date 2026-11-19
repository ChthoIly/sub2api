package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestClaudeSubscriptionLimitRepositoryQueries(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewClaudeSubscriptionLimitRepository(db)
	ctx := context.Background()

	t.Run("ListBoundAccountsByGroupIDs", func(t *testing.T) {
		reset5h := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
		reset7d := time.Now().UTC().Add(3 * 24 * time.Hour).Truncate(time.Second)
		rows := sqlmock.NewRows([]string{
			"group_id", "id", "name", "email", "platform", "type", "status",
			"util_5h", "util_7d", "session_window_end", "reset_7d_sec",
		}).
			AddRow(int64(1), int64(10), "Claude-A", "a@example.com", "anthropic", "oauth", "active", 0.52, 0.76, reset5h, float64(reset7d.Unix())).
			AddRow(int64(2), int64(10), "Claude-A", "a@example.com", "anthropic", "oauth", "active", 0.52, 0.76, reset5h, float64(reset7d.Unix()*1000)).
			AddRow(int64(2), int64(11), "Claude-B", "b@example.com", "anthropic", "setup-token", "disabled", nil, nil, nil, nil)

		mock.ExpectQuery(`FROM account_groups ag`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(rows)

		got, err := repo.ListBoundAccountsByGroupIDs(ctx, []int64{1, 2})
		require.NoError(t, err)
		require.Len(t, got[1], 1)
		require.Len(t, got[2], 2)
		require.NotNil(t, got[1][0].FiveHourUtilizationPercent)
		require.InDelta(t, 52, *got[1][0].FiveHourUtilizationPercent, 1e-8)
		require.NotNil(t, got[1][0].SevenDayUtilizationPercent)
		require.InDelta(t, 76, *got[1][0].SevenDayUtilizationPercent, 1e-8)
		require.NotNil(t, got[2][0].SevenDayResetsAt)
		require.Equal(t, reset7d.Unix(), got[2][0].SevenDayResetsAt.Unix())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ListActiveSourceAccountIDs", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"account_id"}).AddRow(int64(10)).AddRow(int64(25))
		mock.ExpectQuery(`HAVING COUNT\(\*\) = 1`).WillReturnRows(rows)

		ids, err := repo.ListActiveSourceAccountIDs(ctx)
		require.NoError(t, err)
		require.Equal(t, []int64{10, 25}, ids)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ListEnabledGroupConfigsBySourceAccount", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "name", "platform", "status", "subscription_count",
			"five_hour_limit_usd", "weekly_limit_usd",
			"auto_enabled", "initial_five_hour_full_usd", "initial_seven_day_full_usd",
			"five_hour_cap_percent", "seven_day_cap_percent",
			"five_hour_share_count", "seven_day_share_count",
			"five_hour_offset_usd", "seven_day_offset_usd",
		}).AddRow(
			int64(6), "Claude Pro", "anthropic", "active", int64(3),
			20.0, 240.0,
			true, 160.0, 2000.0,
			90.0, 90.0,
			5, 5,
			0.0, 0.0,
		)
		mock.ExpectQuery(`HAVING COUNT\(\*\) = 1`).
			WithArgs(int64(10)).
			WillReturnRows(rows)

		cfgs, err := repo.ListEnabledGroupConfigsBySourceAccount(ctx, 10)
		require.NoError(t, err)
		require.Len(t, cfgs, 1)
		require.Equal(t, int64(6), cfgs[0].GroupID)
		require.Equal(t, "anthropic", cfgs[0].GroupPlatform)
		require.InDelta(t, 160.0, cfgs[0].InitialFiveHourFullUSD, 1e-8)
		require.InDelta(t, 2000.0, cfgs[0].InitialSevenDayFullUSD, 1e-8)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpsertGroupConfig", func(t *testing.T) {
		mock.ExpectExec(`INSERT INTO claude_subscription_limit_configs`).
			WithArgs(int64(6), true, 160.0, 2000.0, 90.0, 85.0, 5, 4, 1.5, -2.0).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpsertGroupConfig(ctx, &service.ClaudeSubscriptionGroupConfig{
			GroupID:                6,
			AutoEnabled:            true,
			InitialFiveHourFullUSD: 160.0,
			InitialSevenDayFullUSD: 2000.0,
			FiveHourCapPercent:     90.0,
			SevenDayCapPercent:     85.0,
			FiveHourShareCount:     5,
			SevenDayShareCount:     4,
			FiveHourOffsetUSD:      1.5,
			SevenDayOffsetUSD:      -2.0,
		})
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetLatestEffectiveEstimate", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		rows := sqlmock.NewRows([]string{
			"estimated_full_usd", "window_start", "window_end", "threshold_percent", "sampled_at",
		}).AddRow(142.5, now.Add(-2*time.Hour), now.Add(3*time.Hour), 75, now)

		mock.ExpectQuery(`FROM claude_subscription_quota_estimates`).
			WithArgs(int64(10), "5h", 50).
			WillReturnRows(rows)

		ref, err := repo.GetLatestEffectiveEstimate(ctx, 10, "5h", 50)
		require.NoError(t, err)
		require.NotNil(t, ref)
		require.InDelta(t, 142.5, ref.EstimatedFullUSD, 1e-8)
		require.Equal(t, 75, ref.ThresholdPercent)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
