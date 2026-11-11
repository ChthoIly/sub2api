package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type dashboardActivityTimelineRepoStub struct {
	UsageLogRepository
	query  usagestats.ActivityTimelineQuery
	result *usagestats.ActivityTimelineResponse
}

func (r *dashboardActivityTimelineRepoStub) GetActivityTimeline(_ context.Context, query usagestats.ActivityTimelineQuery) (*usagestats.ActivityTimelineResponse, error) {
	r.query = query
	return r.result, nil
}

type dashboardActivityTimelineUnsupportedRepo struct {
	UsageLogRepository
}

func TestDashboardServiceGetActivityTimelineUsesOptionalRepository(t *testing.T) {
	query := usagestats.ActivityTimelineQuery{
		Scope:      usagestats.ActivityTimelineScopeUser,
		Timezone:   "Asia/Shanghai",
		Limit:      20,
		GapMinutes: 30,
	}
	expected := &usagestats.ActivityTimelineResponse{Scope: query.Scope, Entities: []usagestats.ActivityTimelineEntity{}}
	repo := &dashboardActivityTimelineRepoStub{result: expected}
	dashboardService := NewDashboardService(repo, nil, nil, nil)

	result, err := dashboardService.GetActivityTimeline(context.Background(), query)

	require.NoError(t, err)
	require.Same(t, expected, result)
	require.Equal(t, query, repo.query)
}

func TestDashboardServiceGetActivityTimelineUnsupportedRepository(t *testing.T) {
	dashboardService := NewDashboardService(&dashboardActivityTimelineUnsupportedRepo{}, nil, nil, nil)

	result, err := dashboardService.GetActivityTimeline(context.Background(), usagestats.ActivityTimelineQuery{})

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrActivityTimelineUnsupported)
	require.True(t, errors.Is(err, ErrActivityTimelineUnsupported))
}
