package service

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestClaudeEstimateFullQuota(t *testing.T) {
	require.InDelta(t, 120, claudeEstimateFullQuota(60, 50), 1e-8)
	require.InDelta(t, 1064.04255313, claudeEstimateFullQuota(1021.480851, 96), 1e-6)
	require.Zero(t, claudeEstimateFullQuota(0, 50))
	require.Zero(t, claudeEstimateFullQuota(10, 0))
}

func TestClaudeCalculateGroupLimit(t *testing.T) {
	require.InDelta(t, 18, claudeCalculateGroupLimit(100, 90, 5, 0), 1e-8)
	require.InDelta(t, 9, claudeCalculateGroupLimit(100, 90, 10, 0), 1e-8)
	require.InDelta(t, 216, claudeCalculateGroupLimit(1200, 90, 5, 0), 1e-8)
	require.InDelta(t, 22.5, claudeCalculateGroupLimit(100, 90, 4, 0), 1e-8)
	require.InDelta(t, 20.5, claudeCalculateGroupLimit(100, 90, 4, -2), 1e-8)
	require.InDelta(t, claudeLimitMinimumPositive, claudeCalculateGroupLimit(100, 90, 4, -100), 1e-12)
}

func TestClaudeThresholdForUtilization(t *testing.T) {
	require.Equal(t, 0, claudeThresholdForUtilization(0))
	require.Equal(t, 0, claudeThresholdForUtilization(20))
	require.Equal(t, 0, claudeThresholdForUtilization(49.999))
	require.Equal(t, 50, claudeThresholdForUtilization(50))
	require.Equal(t, 50, claudeThresholdForUtilization(74.999))
	require.Equal(t, 75, claudeThresholdForUtilization(75))
	require.Equal(t, 75, claudeThresholdForUtilization(84.999))
	require.Equal(t, 85, claudeThresholdForUtilization(85))
	require.Equal(t, 85, claudeThresholdForUtilization(100))
}

type claudeLimitRepoStub struct {
	ClaudeSubscriptionLimitRepository
	sourceOptions     []ClaudeSubscriptionSourceAccountOption
	groups            map[int64]ClaudeSubscriptionGroupConfig
	groupOrder        []int64
	boundByGroup      map[int64][]ClaudeSubscriptionBoundAccountDetail
	enabledBySource   map[int64][]ClaudeSubscriptionGroupConfig
	activeSourceIDs   []int64
	cycles            map[string]*ClaudeSubscriptionQuotaCycle
	effectiveByKey    map[string]*ClaudeSubscriptionQuotaEstimateRef
	recordInsertReply *bool
	recordedEstimates []struct {
		sample    ClaudeSubscriptionQuotaWindowSample
		threshold int
		fullUSD   float64
	}
	appliedInputs []*ClaudeSubscriptionGroupLimitApplyInput
	upserted      []ClaudeSubscriptionGroupConfig
	advanceCalls  int
}

func newClaudeLimitRepoStub() *claudeLimitRepoStub {
	return &claudeLimitRepoStub{
		groups:          make(map[int64]ClaudeSubscriptionGroupConfig),
		boundByGroup:    make(map[int64][]ClaudeSubscriptionBoundAccountDetail),
		enabledBySource: make(map[int64][]ClaudeSubscriptionGroupConfig),
		cycles:          make(map[string]*ClaudeSubscriptionQuotaCycle),
		effectiveByKey:  make(map[string]*ClaudeSubscriptionQuotaEstimateRef),
	}
}

func (s *claudeLimitRepoStub) EnsureGroupConfigs(context.Context) error { return nil }

func (s *claudeLimitRepoStub) ListSourceAccountOptions(context.Context) ([]ClaudeSubscriptionSourceAccountOption, error) {
	return s.sourceOptions, nil
}

func (s *claudeLimitRepoStub) ListGroupConfigs(context.Context) ([]ClaudeSubscriptionGroupConfig, error) {
	out := make([]ClaudeSubscriptionGroupConfig, 0, len(s.groupOrder))
	for _, id := range s.groupOrder {
		out = append(out, s.groups[id])
	}
	return out, nil
}

func (s *claudeLimitRepoStub) GetGroupConfig(_ context.Context, groupID int64) (*ClaudeSubscriptionGroupConfig, error) {
	cfg, ok := s.groups[groupID]
	if !ok {
		return nil, infraerrors.NotFound("CLAUDE_SUBSCRIPTION_LIMIT_GROUP_NOT_FOUND", "not found")
	}
	cp := cfg
	return &cp, nil
}

func (s *claudeLimitRepoStub) UpsertGroupConfig(_ context.Context, cfg *ClaudeSubscriptionGroupConfig) error {
	cp := *cfg
	s.groups[cfg.GroupID] = cp
	s.upserted = append(s.upserted, cp)
	return nil
}

func (s *claudeLimitRepoStub) ListBoundAccountsByGroupIDs(_ context.Context, groupIDs []int64) (map[int64][]ClaudeSubscriptionBoundAccountDetail, error) {
	out := make(map[int64][]ClaudeSubscriptionBoundAccountDetail, len(groupIDs))
	for _, id := range groupIDs {
		if items, ok := s.boundByGroup[id]; ok {
			out[id] = items
		}
	}
	return out, nil
}

func (s *claudeLimitRepoStub) ListActiveSourceAccountIDs(context.Context) ([]int64, error) {
	return s.activeSourceIDs, nil
}

func (s *claudeLimitRepoStub) ListEnabledGroupConfigsBySourceAccount(_ context.Context, sourceAccountID int64) ([]ClaudeSubscriptionGroupConfig, error) {
	return s.enabledBySource[sourceAccountID], nil
}

func (s *claudeLimitRepoStub) EnsureQuotaCycle(_ context.Context, sourceAccountID int64, windowType string, windowStart, windowEnd time.Time) (*ClaudeSubscriptionQuotaCycle, error) {
	key := claudeQuotaStateKey(sourceAccountID, windowType)
	cycle, ok := s.cycles[key]
	if !ok || !cycle.WindowStart.Equal(windowStart) {
		cycle = &ClaudeSubscriptionQuotaCycle{
			SourceAccountID:      sourceAccountID,
			WindowType:           windowType,
			WindowStart:          windowStart,
			WindowEnd:            windowEnd,
			Status:               "active",
			LastThresholdPercent: 0,
		}
		s.cycles[key] = cycle
	}
	cp := *cycle
	return &cp, nil
}

func (s *claudeLimitRepoStub) RecordQuotaEstimate(_ context.Context, sample *ClaudeSubscriptionQuotaWindowSample, threshold int, full float64) (bool, error) {
	if s.recordInsertReply != nil && !*s.recordInsertReply {
		return false, nil
	}
	s.recordedEstimates = append(s.recordedEstimates, struct {
		sample    ClaudeSubscriptionQuotaWindowSample
		threshold int
		fullUSD   float64
	}{
		sample:    *sample,
		threshold: threshold,
		fullUSD:   full,
	})
	s.effectiveByKey[claudeQuotaStateKey(sample.SourceAccountID, sample.WindowType)] = &ClaudeSubscriptionQuotaEstimateRef{
		EstimatedFullUSD: full,
		WindowStart:      sample.WindowStart,
		WindowEnd:        sample.WindowEnd,
		ThresholdPercent: threshold,
		SampledAt:        sample.SampledAt,
	}
	return true, nil
}

func (s *claudeLimitRepoStub) AdvanceQuotaCycle(_ context.Context, sample *ClaudeSubscriptionQuotaWindowSample, threshold int, full float64) error {
	s.advanceCalls++
	key := claudeQuotaStateKey(sample.SourceAccountID, sample.WindowType)
	if cycle, ok := s.cycles[key]; ok && cycle.LastThresholdPercent < threshold {
		cycle.LastThresholdPercent = threshold
		est := full
		cycle.LatestEstimatedFullUSD = &est
	}
	return nil
}

func (s *claudeLimitRepoStub) GetLatestEffectiveEstimate(_ context.Context, sourceAccountID int64, windowType string, minThreshold int) (*ClaudeSubscriptionQuotaEstimateRef, error) {
	ref := s.effectiveByKey[claudeQuotaStateKey(sourceAccountID, windowType)]
	if ref == nil || ref.ThresholdPercent < minThreshold || ref.EstimatedFullUSD <= 0 {
		return nil, nil
	}
	cp := *ref
	return &cp, nil
}

func (s *claudeLimitRepoStub) GetCurrentCycle(_ context.Context, sourceAccountID int64, windowType string) (*ClaudeSubscriptionQuotaCycle, error) {
	cycle := s.cycles[claudeQuotaStateKey(sourceAccountID, windowType)]
	if cycle == nil {
		return nil, nil
	}
	cp := *cycle
	return &cp, nil
}

func (s *claudeLimitRepoStub) ApplyGroupWindowLimit(_ context.Context, input *ClaudeSubscriptionGroupLimitApplyInput) (*ClaudeSubscriptionGroupLimitApplyResult, error) {
	cp := *input
	s.appliedInputs = append(s.appliedInputs, &cp)
	return &ClaudeSubscriptionGroupLimitApplyResult{
		NewLimitUSD: input.NewLimitUSD,
		Changed:     true,
	}, nil
}

type claudeUsageStatsStub struct {
	UsageLogRepository
	cost float64
}

func (s *claudeUsageStatsStub) GetAccountWindowStats(context.Context, int64, time.Time) (*usagestats.AccountStats, error) {
	return &usagestats.AccountStats{Cost: s.cost}, nil
}

type claudeAuthInvalidatorStub struct {
	groups []int64
}

func (s *claudeAuthInvalidatorStub) InvalidateAuthCacheByKey(context.Context, string)   {}
func (s *claudeAuthInvalidatorStub) InvalidateAuthCacheByUserID(context.Context, int64) {}
func (s *claudeAuthInvalidatorStub) InvalidateAuthCacheByGroupID(_ context.Context, groupID int64) {
	s.groups = append(s.groups, groupID)
}

func TestClaudeProcessQuotaSampleImmediatelyAppliesAndRefreshesAt50_75_85(t *testing.T) {
	now := time.Now().UTC()
	windowStart := now.Add(-2 * time.Hour)
	windowEnd := now.Add(3 * time.Hour)

	repo := newClaudeLimitRepoStub()
	repo.enabledBySource[2] = []ClaudeSubscriptionGroupConfig{
		{
			GroupID:            6,
			AutoEnabled:        true,
			FiveHourCapPercent: 90,
			FiveHourShareCount: 5,
		},
		{
			GroupID:            7,
			AutoEnabled:        true,
			FiveHourCapPercent: 80,
			FiveHourShareCount: 4,
			FiveHourOffsetUSD:  2,
		},
	}
	// Group on a different source account must not be affected.
	repo.enabledBySource[9] = []ClaudeSubscriptionGroupConfig{
		{
			GroupID:            99,
			AutoEnabled:        true,
			FiveHourCapPercent: 90,
			FiveHourShareCount: 2,
		},
	}

	usage := &claudeUsageStatsStub{cost: 60}
	invalidator := &claudeAuthInvalidatorStub{}
	svc := NewClaudeSubscriptionLimitService(repo, usage, invalidator)

	// 1) Cross 50% (utilization = 50%, cost = $60 -> full = $120)
	s50 := &ClaudeSubscriptionQuotaWindowSample{
		SourceAccountID:    2,
		WindowType:         ClaudeSubscriptionWindowFiveHour,
		WindowStart:        windowStart,
		WindowEnd:          windowEnd,
		UtilizationPercent: 50,
		SampledAt:          now,
	}
	require.NoError(t, svc.processQuotaSample(context.Background(), s50, 50))
	require.Len(t, repo.recordedEstimates, 1)
	require.InDelta(t, 120, repo.recordedEstimates[0].fullUSD, 1e-8)
	require.Len(t, repo.appliedInputs, 2)
	// Group 6: 120 * 90% / 5 = 21.6
	require.Equal(t, int64(6), repo.appliedInputs[0].GroupID)
	require.InDelta(t, 21.6, repo.appliedInputs[0].NewLimitUSD, 1e-8)
	require.Equal(t, ClaudeSubscriptionEstimateSourceCurrent, repo.appliedInputs[0].AppliedEstimateSource)
	// Group 7: 120 * 80% / 4 + 2 = 26
	require.Equal(t, int64(7), repo.appliedInputs[1].GroupID)
	require.InDelta(t, 26, repo.appliedInputs[1].NewLimitUSD, 1e-8)
	require.Equal(t, []int64{6, 7}, invalidator.groups)

	// 2) Utilization 65% still maps to threshold 50 -> skipped
	s65 := *s50
	s65.UtilizationPercent = 65
	require.NoError(t, svc.processQuotaSample(context.Background(), &s65, 50))
	require.Len(t, repo.appliedInputs, 2)

	// 3) Cross 75% (utilization = 75%, cost = $105 -> full = $140)
	usage.cost = 105
	s75 := *s50
	s75.UtilizationPercent = 75
	s75.SampledAt = now.Add(30 * time.Minute)
	require.NoError(t, svc.processQuotaSample(context.Background(), &s75, 75))
	require.Len(t, repo.recordedEstimates, 2)
	require.InDelta(t, 140, repo.recordedEstimates[1].fullUSD, 1e-8)
	require.Len(t, repo.appliedInputs, 4)
	require.InDelta(t, 25.2, repo.appliedInputs[2].NewLimitUSD, 1e-8) // 140 * 90% / 5
	require.Equal(t, 75, *repo.appliedInputs[2].ThresholdPercent)

	// 4) Cross 85% (utilization = 85%, cost = $127.5 -> full = $150)
	usage.cost = 127.5
	s85 := *s50
	s85.UtilizationPercent = 85
	s85.SampledAt = now.Add(time.Hour)
	require.NoError(t, svc.processQuotaSample(context.Background(), &s85, 85))
	require.Len(t, repo.recordedEstimates, 3)
	require.InDelta(t, 150, repo.recordedEstimates[2].fullUSD, 1e-8)
	require.Len(t, repo.appliedInputs, 6)
	require.InDelta(t, 27, repo.appliedInputs[4].NewLimitUSD, 1e-8) // 150 * 90% / 5
	require.Equal(t, 85, *repo.appliedInputs[4].ThresholdPercent)

	// 5) Further samples (92%) still map to 85 -> skipped
	s92 := s85
	s92.UtilizationPercent = 92
	require.NoError(t, svc.processQuotaSample(context.Background(), &s92, 85))
	require.Len(t, repo.appliedInputs, 6)
}

func TestClaudeProcessQuotaSampleSkipsWhenNoEligibleSingleSourceGroup(t *testing.T) {
	now := time.Now().UTC()
	repo := newClaudeLimitRepoStub()
	// No eligible enabled single-source group for account 2 (e.g. group paused due to multiple bound accounts)
	svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{cost: 70}, nil)
	sample := &ClaudeSubscriptionQuotaWindowSample{
		SourceAccountID:    2,
		WindowType:         ClaudeSubscriptionWindowFiveHour,
		WindowStart:        now.Add(-time.Hour),
		WindowEnd:          now.Add(4 * time.Hour),
		UtilizationPercent: 55,
		SampledAt:          now,
	}
	require.NoError(t, svc.processQuotaSample(context.Background(), sample, 50))
	require.Empty(t, repo.recordedEstimates)
	require.Empty(t, repo.appliedInputs)
	require.Zero(t, repo.advanceCalls)
}

func TestClaudeProcessQuotaSampleSkipsDuplicateInsertFromPeerInstance(t *testing.T) {
	now := time.Now().UTC()
	repo := newClaudeLimitRepoStub()
	notInserted := false
	repo.recordInsertReply = &notInserted
	repo.enabledBySource[2] = []ClaudeSubscriptionGroupConfig{{
		GroupID:            6,
		AutoEnabled:        true,
		FiveHourCapPercent: 90,
		FiveHourShareCount: 5,
	}}
	svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{cost: 60}, nil)
	sample := &ClaudeSubscriptionQuotaWindowSample{
		SourceAccountID:    2,
		WindowType:         ClaudeSubscriptionWindowFiveHour,
		WindowStart:        now.Add(-time.Hour),
		WindowEnd:          now.Add(4 * time.Hour),
		UtilizationPercent: 50,
		SampledAt:          now,
	}
	require.NoError(t, svc.processQuotaSample(context.Background(), sample, 50))
	require.Empty(t, repo.appliedInputs)
	require.Zero(t, repo.advanceCalls)
}

func TestClaudeUpdateGroupConfigSourceValidation(t *testing.T) {
	baseUpdate := func(auto bool) *ClaudeSubscriptionGroupConfigUpdate {
		return &ClaudeSubscriptionGroupConfigUpdate{
			AutoEnabled:            auto,
			InitialFiveHourFullUSD: 150,
			InitialSevenDayFullUSD: 1800,
			FiveHourCapPercent:     90,
			SevenDayCapPercent:     90,
			FiveHourShareCount:     5,
			SevenDayShareCount:     5,
		}
	}

	t.Run("reject enabling when group has no bound accounts", func(t *testing.T) {
		repo := newClaudeLimitRepoStub()
		repo.groups[1] = ClaudeSubscriptionGroupConfig{GroupID: 1, GroupName: "Claude Pro"}
		svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{}, nil)

		err := svc.UpdateGroupConfig(context.Background(), 1, baseUpdate(true))
		require.Error(t, err)
		code, _ := infraerrors.ToHTTP(err)
		require.Equal(t, http.StatusBadRequest, code)
		require.Empty(t, repo.upserted)
	})

	t.Run("reject enabling when group binds multiple accounts even if one is disabled", func(t *testing.T) {
		repo := newClaudeLimitRepoStub()
		repo.groups[1] = ClaudeSubscriptionGroupConfig{GroupID: 1, GroupName: "Claude Pro"}
		repo.boundByGroup[1] = []ClaudeSubscriptionBoundAccountDetail{
			{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 10, Name: "Acc-A", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive}},
			{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 11, Name: "Acc-B", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusDisabled}},
		}
		svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{}, nil)

		err := svc.UpdateGroupConfig(context.Background(), 1, baseUpdate(true))
		require.Error(t, err)
		require.Contains(t, err.Error(), "同时绑定了 2 个账号")
		require.Contains(t, err.Error(), "Acc-A")
		require.Contains(t, err.Error(), "Acc-B")
		require.Empty(t, repo.upserted)
	})

	t.Run("reject enabling when sole bound account is not Anthropic OAuth or Setup Token", func(t *testing.T) {
		repo := newClaudeLimitRepoStub()
		repo.groups[1] = ClaudeSubscriptionGroupConfig{GroupID: 1, GroupName: "Claude Pro"}
		repo.boundByGroup[1] = []ClaudeSubscriptionBoundAccountDetail{
			{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 12, Name: "Key-Only", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive}},
		}
		svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{}, nil)

		err := svc.UpdateGroupConfig(context.Background(), 1, baseUpdate(true))
		require.Error(t, err)
		require.Contains(t, err.Error(), "不是 Anthropic OAuth 或 Setup Token")
		require.Empty(t, repo.upserted)
	})

	t.Run("allow saving defaults and disabling even when multiple accounts are bound", func(t *testing.T) {
		repo := newClaudeLimitRepoStub()
		repo.groups[1] = ClaudeSubscriptionGroupConfig{GroupID: 1, GroupName: "Claude Pro", AutoEnabled: true}
		repo.boundByGroup[1] = []ClaudeSubscriptionBoundAccountDetail{
			{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 10, Name: "Acc-A", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive}},
			{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 11, Name: "Acc-B", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive}},
		}
		svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{}, nil)

		require.NoError(t, svc.UpdateGroupConfig(context.Background(), 1, baseUpdate(false)))
		require.Len(t, repo.upserted, 1)
		require.False(t, repo.upserted[0].AutoEnabled)
		require.InDelta(t, 150, repo.upserted[0].InitialFiveHourFullUSD, 1e-8)
		require.InDelta(t, 1800, repo.upserted[0].InitialSevenDayFullUSD, 1e-8)
		require.Empty(t, repo.appliedInputs)
	})
}

func TestClaudeUpdateGroupConfigInitialDefaultOverriddenPerWindowByDynamicEstimate(t *testing.T) {
	now := time.Now().UTC()
	repo := newClaudeLimitRepoStub()
	repo.groups[1] = ClaudeSubscriptionGroupConfig{
		GroupID:                1,
		GroupName:              "Claude Pro",
		InitialFiveHourFullUSD: 100,
		InitialSevenDayFullUSD: 1200,
	}
	repo.boundByGroup[1] = []ClaudeSubscriptionBoundAccountDetail{
		{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{
			ID:       20,
			Name:     "Source-20",
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
		}},
	}
	svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{}, nil)

	// Phase 1: No >=50% estimate yet -> admin custom initial defaults drive both 5h and 7d.
	require.NoError(t, svc.UpdateGroupConfig(context.Background(), 1, &ClaudeSubscriptionGroupConfigUpdate{
		AutoEnabled:            true,
		InitialFiveHourFullUSD: 160,
		InitialSevenDayFullUSD: 2000,
		FiveHourCapPercent:     90,
		SevenDayCapPercent:     90,
		FiveHourShareCount:     4,
		SevenDayShareCount:     4,
	}))
	require.Len(t, repo.appliedInputs, 2)
	require.Equal(t, ClaudeSubscriptionWindowFiveHour, repo.appliedInputs[0].WindowType)
	require.InDelta(t, 160, repo.appliedInputs[0].AppliedEstimateUSD, 1e-8)
	require.Equal(t, ClaudeSubscriptionEstimateSourceDefault, repo.appliedInputs[0].AppliedEstimateSource)
	require.InDelta(t, 36, repo.appliedInputs[0].NewLimitUSD, 1e-8) // 160 * 90% / 4

	require.Equal(t, ClaudeSubscriptionWindowSevenDay, repo.appliedInputs[1].WindowType)
	require.InDelta(t, 2000, repo.appliedInputs[1].AppliedEstimateUSD, 1e-8)
	require.Equal(t, ClaudeSubscriptionEstimateSourceDefault, repo.appliedInputs[1].AppliedEstimateSource)
	require.InDelta(t, 450, repo.appliedInputs[1].NewLimitUSD, 1e-8) // 2000 * 90% / 4

	// Phase 2: 5h has an effective (>=50%) estimate from a completed cycle; 7d still has none.
	repo.effectiveByKey[claudeQuotaStateKey(20, ClaudeSubscriptionWindowFiveHour)] = &ClaudeSubscriptionQuotaEstimateRef{
		EstimatedFullUSD: 140,
		WindowStart:      now.Add(-6 * time.Hour),
		WindowEnd:        now.Add(-1 * time.Hour),
		ThresholdPercent: 85,
		SampledAt:        now.Add(-90 * time.Minute),
	}
	repo.appliedInputs = nil

	require.NoError(t, svc.UpdateGroupConfig(context.Background(), 1, &ClaudeSubscriptionGroupConfigUpdate{
		AutoEnabled:            true,
		InitialFiveHourFullUSD: 160,
		InitialSevenDayFullUSD: 2000,
		FiveHourCapPercent:     90,
		SevenDayCapPercent:     90,
		FiveHourShareCount:     5,
		SevenDayShareCount:     5,
	}))
	require.Len(t, repo.appliedInputs, 2)
	// 5h is self-driving now (overrides admin default 160 with 140)
	require.InDelta(t, 140, repo.appliedInputs[0].AppliedEstimateUSD, 1e-8)
	require.Equal(t, ClaudeSubscriptionEstimateSourcePrevious, repo.appliedInputs[0].AppliedEstimateSource)
	require.InDelta(t, 25.2, repo.appliedInputs[0].NewLimitUSD, 1e-8) // 140 * 90% / 5
	// 7d still falls back to admin initial default 2000
	require.InDelta(t, 2000, repo.appliedInputs[1].AppliedEstimateUSD, 1e-8)
	require.Equal(t, ClaudeSubscriptionEstimateSourceDefault, repo.appliedInputs[1].AppliedEstimateSource)
	require.InDelta(t, 360, repo.appliedInputs[1].NewLimitUSD, 1e-8) // 2000 * 90% / 5
}

func TestClaudeGetOverviewResolvesPerGroupSourcesAndWarnings(t *testing.T) {
	now := time.Now().UTC()
	future5h := now.Add(2 * time.Hour)
	repo := newClaudeLimitRepoStub()
	repo.groupOrder = []int64{1, 2, 3, 4}
	repo.groups[1] = ClaudeSubscriptionGroupConfig{GroupID: 1, GroupName: "Pro-A", AutoEnabled: true, InitialFiveHourFullUSD: 100, InitialSevenDayFullUSD: 1200}
	repo.groups[2] = ClaudeSubscriptionGroupConfig{GroupID: 2, GroupName: "Pro-B", AutoEnabled: false, InitialFiveHourFullUSD: 180, InitialSevenDayFullUSD: 2400}
	repo.groups[3] = ClaudeSubscriptionGroupConfig{GroupID: 3, GroupName: "Conflict", AutoEnabled: true, InitialFiveHourFullUSD: 100, InitialSevenDayFullUSD: 1200}
	repo.groups[4] = ClaudeSubscriptionGroupConfig{GroupID: 4, GroupName: "Empty", AutoEnabled: false, InitialFiveHourFullUSD: 100, InitialSevenDayFullUSD: 1200}

	repo.boundByGroup[1] = []ClaudeSubscriptionBoundAccountDetail{{
		ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 10, Name: "OAuth-10", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive},
		FiveHourResetsAt:               &future5h,
	}}
	repo.boundByGroup[2] = []ClaudeSubscriptionBoundAccountDetail{{
		ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 11, Name: "Setup-11", Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Status: StatusActive},
	}}
	repo.boundByGroup[3] = []ClaudeSubscriptionBoundAccountDetail{
		{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 10, Name: "OAuth-10", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive}},
		{ClaudeSubscriptionBoundAccount: ClaudeSubscriptionBoundAccount{ID: 11, Name: "Setup-11", Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Status: StatusActive}},
	}
	repo.effectiveByKey[claudeQuotaStateKey(10, ClaudeSubscriptionWindowFiveHour)] = &ClaudeSubscriptionQuotaEstimateRef{
		EstimatedFullUSD: 135,
		WindowStart:      future5h.Add(-5 * time.Hour),
		WindowEnd:        future5h,
		ThresholdPercent: 75,
		SampledAt:        now.Add(-5 * time.Minute),
	}

	svc := NewClaudeSubscriptionLimitService(repo, &claudeUsageStatsStub{}, nil)
	overview, err := svc.GetOverview(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int{50, 75, 85}, overview.Thresholds)
	require.Len(t, overview.Groups, 4)

	// Group 1: OK, 5h self-driving (current_cycle), 7d default
	g1 := overview.Groups[0]
	require.Equal(t, ClaudeSubscriptionSourceStatusOK, g1.SourceStatus)
	require.NotNil(t, g1.SourceAccountID)
	require.Equal(t, int64(10), *g1.SourceAccountID)
	require.Equal(t, ClaudeSubscriptionEstimateSourceCurrent, g1.AppliedEstimates[ClaudeSubscriptionWindowFiveHour].Source)
	require.InDelta(t, 135, g1.AppliedEstimates[ClaudeSubscriptionWindowFiveHour].ValueUSD, 1e-8)
	require.Equal(t, 75, *g1.AppliedEstimates[ClaudeSubscriptionWindowFiveHour].ThresholdPercent)
	require.Equal(t, ClaudeSubscriptionEstimateSourceDefault, g1.AppliedEstimates[ClaudeSubscriptionWindowSevenDay].Source)
	require.InDelta(t, 1200, g1.AppliedEstimates[ClaudeSubscriptionWindowSevenDay].ValueUSD, 1e-8)

	// Group 2: OK, custom admin defaults
	g2 := overview.Groups[1]
	require.Equal(t, ClaudeSubscriptionSourceStatusOK, g2.SourceStatus)
	require.InDelta(t, 180, g2.AppliedEstimates[ClaudeSubscriptionWindowFiveHour].ValueUSD, 1e-8)
	require.InDelta(t, 2400, g2.AppliedEstimates[ClaudeSubscriptionWindowSevenDay].ValueUSD, 1e-8)

	// Group 3: Multiple bound accounts warning
	g3 := overview.Groups[2]
	require.Equal(t, ClaudeSubscriptionSourceStatusMultiple, g3.SourceStatus)
	require.Nil(t, g3.SourceAccountID)
	require.Len(t, g3.BoundAccounts, 2)
	require.NotEmpty(t, g3.SourceWarning)

	// Group 4: Missing bound account warning
	g4 := overview.Groups[3]
	require.Equal(t, ClaudeSubscriptionSourceStatusMissing, g4.SourceStatus)
	require.Nil(t, g4.SourceAccountID)
	require.NotEmpty(t, g4.SourceWarning)
}

func TestClaudePassiveQuotaSamplesUsesHeadersAndWindowResets(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	end5h := now.Add(2 * time.Hour)
	end7d := now.Add(5 * 24 * time.Hour)
	headers := make(http.Header)
	headers.Set("Anthropic-Ratelimit-Unified-5h-Reset", strconv.FormatInt(end5h.Unix(), 10))
	account := &Account{
		ID:       2,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
	}
	samples := claudePassiveQuotaSamples(account, headers, map[string]any{
		"session_window_utilization":   0.52,
		"passive_usage_7d_utilization": 0.76,
		"passive_usage_7d_reset":       end7d.Unix(),
	}, now)

	require.Len(t, samples, 2)
	require.Equal(t, ClaudeSubscriptionWindowFiveHour, samples[0].WindowType)
	require.True(t, end5h.Add(-5*time.Hour).Equal(samples[0].WindowStart))
	require.InDelta(t, 52, samples[0].UtilizationPercent, 1e-9)
	require.Equal(t, ClaudeSubscriptionWindowSevenDay, samples[1].WindowType)
	require.True(t, end7d.Add(-7*24*time.Hour).Equal(samples[1].WindowStart))
	require.InDelta(t, 76, samples[1].UtilizationPercent, 1e-9)
}
