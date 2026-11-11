package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	defaultActivityTimelineLimit      = 20
	defaultActivityTimelineGapMinutes = 30
	maxActivityTimelineLimit          = 100
	maxActivityTimelineGapMinutes     = 240
)

// GetActivityTimeline handles daily active-time windows for users or accounts.
// GET /api/v1/admin/dashboard/activity-timeline
func (h *DashboardHandler) GetActivityTimeline(c *gin.Context) {
	scope := strings.TrimSpace(c.DefaultQuery("scope", usagestats.ActivityTimelineScopeUser))
	if !usagestats.IsValidActivityTimelineScope(scope) {
		response.Error(c, http.StatusBadRequest, "scope must be one of: user, account")
		return
	}

	entityID, err := parseActivityTimelineEntityID(c.Query("entity_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "entity_id must be a non-negative integer")
		return
	}

	timezoneName, location := resolveActivityTimelineTimezone(c.Query("timezone"))
	startTime, endTime := parseTimeRange(c)

	query := usagestats.ActivityTimelineQuery{
		Scope:      scope,
		StartTime:  startTime,
		EndTime:    endTime,
		Timezone:   timezoneName,
		EntityID:   entityID,
		Limit:      parseActivityTimelineLimit(c.Query("limit")),
		GapMinutes: parseActivityTimelineGapMinutes(c.Query("gap_minutes")),
	}

	timeline, err := h.dashboardService.GetActivityTimeline(c.Request.Context(), query)
	if err != nil {
		if errors.Is(err, service.ErrActivityTimelineUnsupported) {
			response.Error(c, http.StatusInternalServerError, "Activity timeline is not supported by the usage log repository")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to get activity timeline")
		return
	}

	// These fields describe the request and are owned by the handler so the
	// JSON contract stays stable regardless of the optional repo implementation.
	timeline.Scope = scope
	timeline.StartDate = startTime.In(location).Format("2006-01-02")
	timeline.EndDate = endTime.Add(-time.Nanosecond).In(location).Format("2006-01-02")
	timeline.Timezone = timezoneName
	timeline.GapMinutes = query.GapMinutes
	if timeline.Entities == nil {
		timeline.Entities = make([]usagestats.ActivityTimelineEntity, 0)
	}

	response.Success(c, timeline)
}

func parseActivityTimelineEntityID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	entityID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || entityID < 0 {
		return 0, errors.New("invalid entity_id")
	}
	return entityID, nil
}

func parseActivityTimelineLimit(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultActivityTimelineLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return defaultActivityTimelineLimit
	}
	if limit < 1 {
		return 1
	}
	if limit > maxActivityTimelineLimit {
		return maxActivityTimelineLimit
	}
	return limit
}

func parseActivityTimelineGapMinutes(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultActivityTimelineGapMinutes
	}
	gapMinutes, err := strconv.Atoi(raw)
	if err != nil || gapMinutes < 1 || gapMinutes > maxActivityTimelineGapMinutes {
		return defaultActivityTimelineGapMinutes
	}
	return gapMinutes
}

// resolveActivityTimelineTimezone validates the requested timezone before it is
// passed to PostgreSQL. Invalid or missing values fall back to the configured
// server timezone; if that fallback is not a loadable Go timezone, use UTC.
func resolveActivityTimelineTimezone(raw string) (string, *time.Location) {
	requested := strings.TrimSpace(raw)
	if requested != "" {
		if location, err := time.LoadLocation(requested); err == nil {
			return requested, location
		}
	}

	fallback := strings.TrimSpace(timezone.Name())
	if fallback != "" {
		if location, err := time.LoadLocation(fallback); err == nil {
			return fallback, location
		}
	}
	return "UTC", time.UTC
}
