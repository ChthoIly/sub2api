package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetActivityTimelineGroupsEntityDayAndSegments(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)

	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2026, 11, 6, 0, 0, 0, 0, location)
	end := time.Date(2026, 11, 11, 0, 0, 0, 0, location)
	query := usagestats.ActivityTimelineQuery{
		Scope:      usagestats.ActivityTimelineScopeUser,
		StartTime:  start,
		EndTime:    end,
		Timezone:   "Asia/Shanghai",
		EntityID:   7,
		Limit:      20,
		GapMinutes: 30,
	}

	mock.ExpectQuery(`(?s)WITH ranked_entities AS .*COUNT\(\*\)::bigint AS total_requests.*ORDER BY COUNT\(\*\) DESC, ul\.user_id ASC.*LIMIT \$3::int.*LAG\(ul\.created_at\) OVER.*AT TIME ZONE \$4::text.*make_interval\(mins => \$5::int\).*TO_CHAR\(seg\.segment_start AT TIME ZONE \$4::text, 'HH24:MI:SS'\).*FROM ranked_entities re LEFT JOIN users u ON u\.id = re\.entity_id.*ORDER BY re\.total_requests DESC, re\.entity_id ASC`).
		WithArgs(start, end, 20, "Asia/Shanghai", 30, int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"entity_id", "entity_name", "entity_detail", "total_requests", "activity_date",
			"active_hours", "segment_count", "coverage_seconds", "longest_segment_seconds",
			"segment_start", "segment_end", "segment_requests", "duration_seconds",
		}).
			// Deliberately out of order: the repository must sort entities, days,
			// and segments before returning the contract payload.
			AddRow(int64(9), "Claude Ultra", "ultra@example.com", int64(110), "2026-11-10", int64(3), int64(1), int64(120), int64(120), "10:00:00", "10:02:00", int64(10), int64(120)).
			AddRow(int64(3), "Second User", "second@example.com", int64(50), "2026-11-07", int64(1), int64(1), int64(0), int64(0), "08:00:00", "08:00:00", int64(50), int64(0)).
			AddRow(int64(9), "Claude Ultra", "ultra@example.com", int64(110), "2026-11-06", int64(8), int64(2), int64(4513), int64(3000), "05:30:00", "06:20:00", int64(16), int64(3000)).
			AddRow(int64(9), "Claude Ultra", "ultra@example.com", int64(110), "2026-11-06", int64(8), int64(2), int64(4513), int64(3000), "04:26:57", "04:52:10", int64(84), int64(1513)))

	result, err := repo.GetActivityTimeline(context.Background(), query)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.Equal(t, usagestats.ActivityTimelineScopeUser, result.Scope)
	require.Equal(t, "2026-11-06", result.StartDate)
	require.Equal(t, "2026-11-10", result.EndDate)
	require.Equal(t, "Asia/Shanghai", result.Timezone)
	require.Equal(t, 30, result.GapMinutes)
	require.Len(t, result.Entities, 2)

	// Total requests desc, entity_id asc.
	first := result.Entities[0]
	require.Equal(t, int64(9), first.EntityID)
	require.Equal(t, "Claude Ultra", first.Name)
	require.Equal(t, "ultra@example.com", first.Detail)
	require.Equal(t, int64(110), first.TotalRequests)
	require.Equal(t, int64(2), first.ActiveDays)
	require.Equal(t, int64(11), first.TotalActiveHours)
	require.Equal(t, int64(4633), first.TotalCoverageSeconds)
	require.Equal(t, int64(3000), first.LongestSegmentSeconds)
	require.Len(t, first.Days, 2)
	require.Equal(t, "2026-11-06", first.Days[0].Date)
	require.Equal(t, int64(100), first.Days[0].Requests)
	require.Equal(t, int64(2), first.Days[0].SegmentCount)
	require.Len(t, first.Days[0].Segments, 2)
	require.Equal(t, "04:26:57", first.Days[0].Segments[0].Start)
	require.Equal(t, int64(84), first.Days[0].Segments[0].Requests)
	require.Equal(t, int64(1513), first.Days[0].Segments[0].DurationSeconds)
	require.Equal(t, "05:30:00", first.Days[0].Segments[1].Start)
	require.Equal(t, "2026-11-10", first.Days[1].Date)
	require.Equal(t, int64(10), first.Days[1].Requests)

	second := result.Entities[1]
	require.Equal(t, int64(3), second.EntityID)
	require.Equal(t, int64(50), second.TotalRequests)
	require.Equal(t, int64(1), second.ActiveDays)
	require.Equal(t, int64(1), second.TotalActiveHours)
}

func TestUsageLogRepositoryGetActivityTimelineAccountScope(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)

	start := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	query := usagestats.ActivityTimelineQuery{
		Scope:      usagestats.ActivityTimelineScopeAccount,
		StartTime:  start,
		EndTime:    end,
		Timezone:   "UTC",
		Limit:      5,
		GapMinutes: 15,
	}

	mock.ExpectQuery(`(?s)WITH ranked_entities AS .*ul\.account_id AS entity_id.*COALESCE\(a\.platform, ''\) \|\| ' / ' \|\| COALESCE\(a\.type, ''\).*FROM ranked_entities re LEFT JOIN accounts a ON a\.id = re\.entity_id.*ORDER BY re\.total_requests DESC`).
		WithArgs(start, end, 5, "UTC", 15, int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{
			"entity_id", "entity_name", "entity_detail", "total_requests", "activity_date",
			"active_hours", "segment_count", "coverage_seconds", "longest_segment_seconds",
			"segment_start", "segment_end", "segment_requests", "duration_seconds",
		}))

	result, err := repo.GetActivityTimeline(context.Background(), query)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, usagestats.ActivityTimelineScopeAccount, result.Scope)
	require.NotNil(t, result.Entities)
	require.Empty(t, result.Entities)
}
