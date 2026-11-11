package usagestats

import "time"

const (
	ActivityTimelineScopeUser    = "user"
	ActivityTimelineScopeAccount = "account"
)

// IsValidActivityTimelineScope reports whether scope is supported by the
// activity timeline endpoint.
func IsValidActivityTimelineScope(scope string) bool {
	switch scope {
	case ActivityTimelineScopeUser, ActivityTimelineScopeAccount:
		return true
	default:
		return false
	}
}

// ActivityTimelineQuery selects the activity timeline rows to aggregate.
type ActivityTimelineQuery struct {
	Scope      string
	StartTime  time.Time
	EndTime    time.Time
	Timezone   string
	EntityID   int64
	Limit      int
	GapMinutes int
}

// ActivityTimelineResponse is the admin dashboard activity timeline payload.
type ActivityTimelineResponse struct {
	Scope      string                   `json:"scope"`
	StartDate  string                   `json:"start_date"`
	EndDate    string                   `json:"end_date"`
	Timezone   string                   `json:"timezone"`
	GapMinutes int                      `json:"gap_minutes"`
	Entities   []ActivityTimelineEntity `json:"entities"`
}

// ActivityTimelineEntity contains daily active windows for one user or account.
type ActivityTimelineEntity struct {
	EntityID      int64  `json:"entity_id"`
	Name          string `json:"name"`
	Detail        string `json:"detail"`
	TotalRequests int64  `json:"total_requests"`
	ActiveDays    int64  `json:"active_days"`
	// TotalActiveHours counts distinct local (date, hour) buckets with requests.
	TotalActiveHours      int64                 `json:"total_active_hours"`
	TotalCoverageSeconds  int64                 `json:"total_coverage_seconds"`
	LongestSegmentSeconds int64                 `json:"longest_segment_seconds"`
	Days                  []ActivityTimelineDay `json:"days"`
}

// ActivityTimelineDay contains the active windows for one local calendar day.
type ActivityTimelineDay struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	// ActiveHours counts distinct local hour buckets with requests that day.
	ActiveHours           int64                     `json:"active_hours"`
	SegmentCount          int64                     `json:"segment_count"`
	CoverageSeconds       int64                     `json:"coverage_seconds"`
	LongestSegmentSeconds int64                     `json:"longest_segment_seconds"`
	Segments              []ActivityTimelineSegment `json:"segments"`
}

// ActivityTimelineSegment is a continuous sequence of requests where each
// inter-request gap does not exceed the requested gap threshold.
type ActivityTimelineSegment struct {
	Start           string `json:"start"`
	End             string `json:"end"`
	Requests        int64  `json:"requests"`
	DurationSeconds int64  `json:"duration_seconds"`
}
