package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// ErrActivityTimelineUnsupported is returned when the configured usage log
// repository does not implement ActivityTimelineRepository.
var ErrActivityTimelineUnsupported = errors.New("activity timeline is not supported by the usage log repository")

// ActivityTimelineRepository is an optional extension to UsageLogRepository.
// Keeping it separate avoids forcing every existing usage-log test stub to
// implement the activity timeline method.
type ActivityTimelineRepository interface {
	GetActivityTimeline(ctx context.Context, query usagestats.ActivityTimelineQuery) (*usagestats.ActivityTimelineResponse, error)
}

// GetActivityTimeline returns sessionized daily active windows for users or
// accounts. It type-asserts the optional repository capability on demand.
func (s *DashboardService) GetActivityTimeline(ctx context.Context, query usagestats.ActivityTimelineQuery) (*usagestats.ActivityTimelineResponse, error) {
	repo, ok := s.usageRepo.(ActivityTimelineRepository)
	if !ok {
		return nil, fmt.Errorf("%w (%T)", ErrActivityTimelineUnsupported, s.usageRepo)
	}

	timeline, err := repo.GetActivityTimeline(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get activity timeline: %w", err)
	}
	if timeline == nil {
		return nil, errors.New("get activity timeline: repository returned nil response")
	}
	return timeline, nil
}
