package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const activityTimelineOrderBy = `
ORDER BY re.total_requests DESC, re.entity_id ASC, seg.activity_date ASC, seg.segment_start ASC, seg.segment_number ASC`

// activityTimelineQueryTemplate sessionizes requests per entity and local day.
// The ranking CTE is evaluated before request details so only the top entities
// are expanded into window-function rows. Entity select/join clauses are static
// strings selected by scope and are never built from caller input.
const activityTimelineQueryTemplate = `
WITH ranked_entities AS (
    SELECT
        ul.%[1]s AS entity_id,
        COUNT(*)::bigint AS total_requests
    FROM usage_logs ul
    WHERE ul.created_at >= $1
      AND ul.created_at < $2
      AND ul.%[1]s > 0
      AND ($6::bigint = 0 OR ul.%[1]s = $6::bigint)
    GROUP BY ul.%[1]s
    ORDER BY COUNT(*) DESC, ul.%[1]s ASC
    LIMIT $3::int
),
ordered_requests AS (
    SELECT
        ul.id,
        ul.%[1]s AS entity_id,
        ul.created_at,
        (ul.created_at AT TIME ZONE $4::text)::date AS activity_date,
        LAG(ul.created_at) OVER (
            PARTITION BY ul.%[1]s, (ul.created_at AT TIME ZONE $4::text)::date
            ORDER BY ul.created_at, ul.id
        ) AS previous_created_at
    FROM usage_logs ul
    JOIN ranked_entities re ON re.entity_id = ul.%[1]s
    WHERE ul.created_at >= $1
      AND ul.created_at < $2
      AND ul.%[1]s > 0
      AND ($6::bigint = 0 OR ul.%[1]s = $6::bigint)
),
sessionized_requests AS (
    SELECT
        id,
        entity_id,
        created_at,
        activity_date,
        SUM(CASE
            WHEN previous_created_at IS NULL THEN 1
            WHEN created_at - previous_created_at > make_interval(mins => $5::int) THEN 1
            ELSE 0
        END) OVER (
            PARTITION BY entity_id, activity_date
            ORDER BY created_at, id
            ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
        ) AS segment_number
    FROM ordered_requests
),
segments AS (
    SELECT
        entity_id,
        activity_date,
        segment_number,
        COUNT(*)::bigint AS requests,
        MIN(created_at) AS segment_start,
        MAX(created_at) AS segment_end,
        GREATEST(FLOOR(EXTRACT(EPOCH FROM (MAX(created_at) - MIN(created_at))))::bigint, 0) AS duration_seconds
    FROM sessionized_requests
    GROUP BY entity_id, activity_date, segment_number
),
day_segments AS (
    SELECT
        entity_id,
        activity_date,
        COUNT(*)::bigint AS segment_count,
        SUM(duration_seconds)::bigint AS coverage_seconds,
        MAX(duration_seconds)::bigint AS longest_segment_seconds
    FROM segments
    GROUP BY entity_id, activity_date
),
day_hours AS (
    SELECT
        entity_id,
        activity_date,
        COUNT(DISTINCT DATE_TRUNC('hour', created_at AT TIME ZONE $4::text))::bigint AS active_hours
    FROM ordered_requests
    GROUP BY entity_id, activity_date
)
SELECT
    re.entity_id,
    %[2]s AS entity_name,
    %[3]s AS entity_detail,
    re.total_requests,
    TO_CHAR(seg.activity_date, 'YYYY-MM-DD') AS activity_date,
    dh.active_hours,
    ds.segment_count,
    ds.coverage_seconds,
    ds.longest_segment_seconds,
    TO_CHAR(seg.segment_start AT TIME ZONE $4::text, 'HH24:MI:SS') AS segment_start,
    TO_CHAR(seg.segment_end AT TIME ZONE $4::text, 'HH24:MI:SS') AS segment_end,
    seg.requests AS segment_requests,
    seg.duration_seconds
FROM ranked_entities re
LEFT JOIN %[4]s
JOIN segments seg ON seg.entity_id = re.entity_id
JOIN day_segments ds ON ds.entity_id = re.entity_id AND ds.activity_date = seg.activity_date
JOIN day_hours dh ON dh.entity_id = re.entity_id AND dh.activity_date = seg.activity_date
` + activityTimelineOrderBy

// GetActivityTimeline returns sessionized daily active windows for the top
// users or accounts in the requested time range. Sessions are computed in SQL
// and grouped by local calendar day; only the returned segment rows are grouped
// in Go.
func (r *usageLogRepository) GetActivityTimeline(ctx context.Context, query usagestats.ActivityTimelineQuery) (*usagestats.ActivityTimelineResponse, error) {
	if !usagestats.IsValidActivityTimelineScope(query.Scope) {
		return nil, fmt.Errorf("unsupported activity timeline scope %q", query.Scope)
	}
	if !query.EndTime.After(query.StartTime) {
		return nil, fmt.Errorf("activity timeline end time must be after start time")
	}
	if query.EntityID < 0 {
		return nil, fmt.Errorf("activity timeline entity_id must be non-negative")
	}

	location, err := time.LoadLocation(query.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid activity timeline timezone %q: %w", query.Timezone, err)
	}

	if query.Limit < 1 {
		query.Limit = 1
	} else if query.Limit > 100 {
		query.Limit = 100
	}
	if query.GapMinutes < 1 || query.GapMinutes > 240 {
		query.GapMinutes = 30
	}

	var (
		entityColumn string
		nameExpr     string
		detailExpr   string
		joinClause   string
	)
	switch query.Scope {
	case usagestats.ActivityTimelineScopeUser:
		entityColumn = "user_id"
		nameExpr = `COALESCE(NULLIF(BTRIM(u.username), ''), NULLIF(BTRIM(u.email), ''), '#' || re.entity_id::text)`
		detailExpr = `COALESCE(u.email, '')`
		joinClause = `users u ON u.id = re.entity_id`
	case usagestats.ActivityTimelineScopeAccount:
		entityColumn = "account_id"
		nameExpr = `COALESCE(NULLIF(BTRIM(a.name), ''), '#' || re.entity_id::text)`
		detailExpr = `COALESCE(a.platform, '') || ' / ' || COALESCE(a.type, '')`
		joinClause = `accounts a ON a.id = re.entity_id`
	}

	sqlQuery := fmt.Sprintf(activityTimelineQueryTemplate, entityColumn, nameExpr, detailExpr, joinClause)
	rows, err := r.sql.QueryContext(
		ctx,
		sqlQuery,
		query.StartTime,
		query.EndTime,
		query.Limit,
		query.Timezone,
		query.GapMinutes,
		query.EntityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	response := &usagestats.ActivityTimelineResponse{
		Scope:      query.Scope,
		StartDate:  query.StartTime.In(location).Format("2006-01-02"),
		EndDate:    query.EndTime.Add(-time.Nanosecond).In(location).Format("2006-01-02"),
		Timezone:   query.Timezone,
		GapMinutes: query.GapMinutes,
		Entities:   make([]usagestats.ActivityTimelineEntity, 0),
	}

	entityIndexes := make(map[int64]int)
	dayIndexes := make(map[string]int)

	for rows.Next() {
		var (
			entityID              int64
			name                  string
			detail                string
			totalRequests         int64
			date                  string
			activeHours           int64
			segmentCount          int64
			coverageSeconds       int64
			longestSegmentSeconds int64
			segmentStart          string
			segmentEnd            string
			segmentRequests       int64
			segmentDuration       int64
		)
		if err := rows.Scan(
			&entityID,
			&name,
			&detail,
			&totalRequests,
			&date,
			&activeHours,
			&segmentCount,
			&coverageSeconds,
			&longestSegmentSeconds,
			&segmentStart,
			&segmentEnd,
			&segmentRequests,
			&segmentDuration,
		); err != nil {
			return nil, err
		}

		entityIdx, ok := entityIndexes[entityID]
		if !ok {
			entityIdx = len(response.Entities)
			entityIndexes[entityID] = entityIdx
			response.Entities = append(response.Entities, usagestats.ActivityTimelineEntity{
				EntityID:      entityID,
				Name:          name,
				Detail:        detail,
				TotalRequests: totalRequests,
				Days:          make([]usagestats.ActivityTimelineDay, 0),
			})
		}

		entity := &response.Entities[entityIdx]
		dayKey := fmt.Sprintf("%d|%s", entityID, date)
		dayIdx, ok := dayIndexes[dayKey]
		if !ok {
			dayIdx = len(entity.Days)
			dayIndexes[dayKey] = dayIdx
			entity.Days = append(entity.Days, usagestats.ActivityTimelineDay{
				Date:                  date,
				ActiveHours:           activeHours,
				SegmentCount:          segmentCount,
				CoverageSeconds:       coverageSeconds,
				LongestSegmentSeconds: longestSegmentSeconds,
				Segments:              make([]usagestats.ActivityTimelineSegment, 0),
			})
		}

		day := &entity.Days[dayIdx]
		day.Requests += segmentRequests
		day.Segments = append(day.Segments, usagestats.ActivityTimelineSegment{
			Start:           segmentStart,
			End:             segmentEnd,
			Requests:        segmentRequests,
			DurationSeconds: segmentDuration,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range response.Entities {
		entity := &response.Entities[i]
		for j := range entity.Days {
			day := &entity.Days[j]
			sort.Slice(day.Segments, func(a, b int) bool {
				if day.Segments[a].Start == day.Segments[b].Start {
					return day.Segments[a].End < day.Segments[b].End
				}
				return day.Segments[a].Start < day.Segments[b].Start
			})
			entity.ActiveDays++
			entity.TotalActiveHours += day.ActiveHours
			entity.TotalCoverageSeconds += day.CoverageSeconds
			if day.LongestSegmentSeconds > entity.LongestSegmentSeconds {
				entity.LongestSegmentSeconds = day.LongestSegmentSeconds
			}
		}
		sort.Slice(entity.Days, func(a, b int) bool {
			return entity.Days[a].Date < entity.Days[b].Date
		})
	}

	sort.Slice(response.Entities, func(a, b int) bool {
		if response.Entities[a].TotalRequests == response.Entities[b].TotalRequests {
			return response.Entities[a].EntityID < response.Entities[b].EntityID
		}
		return response.Entities[a].TotalRequests > response.Entities[b].TotalRequests
	})

	return response, nil
}
