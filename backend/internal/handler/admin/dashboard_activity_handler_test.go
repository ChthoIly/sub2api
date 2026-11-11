package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type activityTimelineRepoStub struct {
	service.UsageLogRepository
	query  usagestats.ActivityTimelineQuery
	result *usagestats.ActivityTimelineResponse
}

func (r *activityTimelineRepoStub) GetActivityTimeline(_ context.Context, query usagestats.ActivityTimelineQuery) (*usagestats.ActivityTimelineResponse, error) {
	r.query = query
	if r.result != nil {
		return r.result, nil
	}
	return &usagestats.ActivityTimelineResponse{Entities: []usagestats.ActivityTimelineEntity{}}, nil
}

func newActivityTimelineTestRouter(repo *activityTimelineRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	dashboardService := service.NewDashboardService(repo, nil, nil, nil)
	handler := NewDashboardHandler(dashboardService, nil)
	router := gin.New()
	router.GET("/admin/dashboard/activity-timeline", handler.GetActivityTimeline)
	return router
}

func TestGetActivityTimeline_DefaultsAndResponseShape(t *testing.T) {
	repo := &activityTimelineRepoStub{
		result: &usagestats.ActivityTimelineResponse{
			Entities: []usagestats.ActivityTimelineEntity{
				{
					EntityID:              1,
					Name:                  "Claude Ultra",
					Detail:                "user@example.com",
					TotalRequests:         110,
					ActiveDays:            2,
					TotalActiveHours:      11,
					TotalCoverageSeconds:  4633,
					LongestSegmentSeconds: 3000,
					Days: []usagestats.ActivityTimelineDay{
						{
							Date:                  "2026-11-06",
							Requests:              100,
							ActiveHours:           8,
							SegmentCount:          2,
							CoverageSeconds:       4513,
							LongestSegmentSeconds: 3000,
							Segments: []usagestats.ActivityTimelineSegment{
								{Start: "04:26:57", End: "04:52:10", Requests: 84, DurationSeconds: 1513},
								{Start: "05:30:00", End: "06:20:00", Requests: 16, DurationSeconds: 3000},
							},
						},
					},
				},
			},
		},
	}
	router := newActivityTimelineTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/activity-timeline?start_date=2026-11-06&end_date=2026-11-10&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, usagestats.ActivityTimelineScopeUser, repo.query.Scope)
	require.Equal(t, int64(0), repo.query.EntityID)
	require.Equal(t, 20, repo.query.Limit)
	require.Equal(t, 30, repo.query.GapMinutes)
	require.Equal(t, "Asia/Shanghai", repo.query.Timezone)
	require.Equal(t, "2026-11-06", repo.query.StartTime.Format("2006-01-02"))
	require.Equal(t, "2026-11-11", repo.query.EndTime.Format("2006-01-02"))

	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)

	var data map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	for _, key := range []string{"scope", "start_date", "end_date", "timezone", "gap_minutes", "entities"} {
		require.Contains(t, data, key)
	}
	require.Equal(t, "user", data["scope"])
	require.Equal(t, "2026-11-06", data["start_date"])
	require.Equal(t, "2026-11-10", data["end_date"])
	require.Equal(t, "Asia/Shanghai", data["timezone"])
	require.Equal(t, float64(30), data["gap_minutes"])

	entities, ok := data["entities"].([]any)
	require.True(t, ok)
	require.Len(t, entities, 1)
	entity, ok := entities[0].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{
		"entity_id", "name", "detail", "total_requests", "active_days",
		"total_active_hours", "total_coverage_seconds", "longest_segment_seconds", "days",
	} {
		require.Contains(t, entity, key)
	}
	require.Equal(t, float64(1), entity["entity_id"])
	require.Equal(t, "Claude Ultra", entity["name"])

	days, ok := entity["days"].([]any)
	require.True(t, ok)
	require.Len(t, days, 1)
	day, ok := days[0].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{
		"date", "requests", "active_hours", "segment_count",
		"coverage_seconds", "longest_segment_seconds", "segments",
	} {
		require.Contains(t, day, key)
	}

	segments, ok := day["segments"].([]any)
	require.True(t, ok)
	require.Len(t, segments, 2)
	segment, ok := segments[0].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{"start", "end", "requests", "duration_seconds"} {
		require.Contains(t, segment, key)
	}
	require.Equal(t, "04:26:57", segment["start"])
	require.Equal(t, "04:52:10", segment["end"])
	require.Equal(t, float64(84), segment["requests"])
	require.Equal(t, float64(1513), segment["duration_seconds"])

	var timeline usagestats.ActivityTimelineResponse
	require.NoError(t, json.Unmarshal(envelope.Data, &timeline))
	require.Equal(t, "user", timeline.Scope)
	require.Equal(t, "2026-11-06", timeline.StartDate)
	require.Equal(t, "2026-11-10", timeline.EndDate)
	require.Equal(t, 30, timeline.GapMinutes)
	require.Len(t, timeline.Entities, 1)
	require.Equal(t, int64(110), timeline.Entities[0].TotalRequests)
}

func TestGetActivityTimeline_ScopeSwitchAndEntityFilter(t *testing.T) {
	repo := &activityTimelineRepoStub{}
	router := newActivityTimelineTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/activity-timeline?scope=account&entity_id=42&limit=7&gap_minutes=12&start_date=2026-11-06&end_date=2026-11-10&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, usagestats.ActivityTimelineScopeAccount, repo.query.Scope)
	require.Equal(t, int64(42), repo.query.EntityID)
	require.Equal(t, 7, repo.query.Limit)
	require.Equal(t, 12, repo.query.GapMinutes)

	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Scope      string                              `json:"scope"`
			StartDate  string                              `json:"start_date"`
			EndDate    string                              `json:"end_date"`
			Timezone   string                              `json:"timezone"`
			GapMinutes int                                 `json:"gap_minutes"`
			Entities   []usagestats.ActivityTimelineEntity `json:"entities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, "account", envelope.Data.Scope)
	require.Equal(t, "2026-11-06", envelope.Data.StartDate)
	require.Equal(t, "2026-11-10", envelope.Data.EndDate)
	require.Equal(t, "Asia/Shanghai", envelope.Data.Timezone)
	require.Equal(t, 12, envelope.Data.GapMinutes)
	require.NotNil(t, envelope.Data.Entities)
	require.Empty(t, envelope.Data.Entities)
}

func TestGetActivityTimeline_InvalidScope(t *testing.T) {
	repo := &activityTimelineRepoStub{}
	router := newActivityTimelineTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/activity-timeline?scope=model", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, repo.query.Limit)
}

func TestGetActivityTimeline_InvalidEntityID(t *testing.T) {
	repo := &activityTimelineRepoStub{}
	router := newActivityTimelineTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/activity-timeline?entity_id=-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, repo.query.Limit)
}

func TestGetActivityTimeline_LimitAndGapFallbacks(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLimit int
		wantGap   int
	}{
		{name: "defaults", query: "", wantLimit: 20, wantGap: 30},
		{name: "limit zero clamps", query: "limit=0", wantLimit: 1, wantGap: 30},
		{name: "limit high clamps", query: "limit=101", wantLimit: 100, wantGap: 30},
		{name: "limit invalid falls back", query: "limit=abc", wantLimit: 20, wantGap: 30},
		{name: "gap zero falls back", query: "gap_minutes=0", wantLimit: 20, wantGap: 30},
		{name: "gap high falls back", query: "gap_minutes=241", wantLimit: 20, wantGap: 30},
		{name: "gap max accepted", query: "gap_minutes=240", wantLimit: 20, wantGap: 240},
		{name: "gap invalid falls back", query: "gap_minutes=abc", wantLimit: 20, wantGap: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &activityTimelineRepoStub{}
			router := newActivityTimelineTestRouter(repo)
			req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/activity-timeline?"+tt.query, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.wantLimit, repo.query.Limit)
			require.Equal(t, tt.wantGap, repo.query.GapMinutes)
		})
	}
}

func TestGetActivityTimeline_InvalidTimezoneFallsBackToServerTimezone(t *testing.T) {
	repo := &activityTimelineRepoStub{}
	router := newActivityTimelineTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/activity-timeline?timezone=Not/ARealTimezone&start_date=2026-11-06&end_date=2026-11-10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	expectedTimezone := timezone.Name()
	if _, err := time.LoadLocation(expectedTimezone); err != nil {
		expectedTimezone = "UTC"
	}
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, expectedTimezone, repo.query.Timezone)
}

type activityTimelineUnsupportedRepo struct {
	service.UsageLogRepository
}

func TestGetActivityTimeline_UnsupportedRepositoryReturnsClearError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dashboardService := service.NewDashboardService(&activityTimelineUnsupportedRepo{}, nil, nil, nil)
	handler := NewDashboardHandler(dashboardService, nil)
	router := gin.New()
	router.GET("/admin/dashboard/activity-timeline", handler.GetActivityTimeline)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/dashboard/activity-timeline?start_date=2026-11-06&end_date=2026-11-10&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "not supported by the usage log repository")
}
