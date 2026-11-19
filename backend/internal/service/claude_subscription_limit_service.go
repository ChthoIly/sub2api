package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	claudeQuotaWindowFiveHours   = 5 * time.Hour
	claudeQuotaWindowSevenDays   = 7 * 24 * time.Hour
	claudeSourceCacheTTL         = 15 * time.Second
	claudeLimitRoundScale        = 1e8
	claudeLimitMinimumPositive   = 1e-8
	claudeMaxInitialFiveHourUSD  = 1_000_000.0
	claudeMaxInitialSevenDayUSD  = 10_000_000.0
	claudeWindowResetMatchWindow = 2 * time.Minute
)

// ClaudeSubscriptionPassiveUsageObserver consumes successfully sampled Anthropic
// response headers. It is intentionally synchronous only for parsing; the
// quota/DB work is scheduled away from the gateway response path.
type ClaudeSubscriptionPassiveUsageObserver interface {
	ObservePassiveUsage(ctx context.Context, account *Account, headers http.Header, extraUpdates map[string]any)
}

type claudeQuotaRuntimeState struct {
	windowStart   time.Time
	windowEnd     time.Time
	lastThreshold int
}

// ClaudeSubscriptionLimitService calculates and applies dynamic Claude group
// limits from each group's uniquely bound upstream source account's passive
// 5h / 7d usage-window samples.
type ClaudeSubscriptionLimitService struct {
	repo        ClaudeSubscriptionLimitRepository
	usageRepo   UsageLogRepository
	invalidator APIKeyAuthCacheInvalidator

	processMu    sync.Mutex
	runtimeMu    sync.RWMutex
	runtimeState map[string]claudeQuotaRuntimeState

	sourceMu        sync.Mutex
	activeSourceIDs map[int64]bool
	sourceLoaded    bool
	sourceExpires   time.Time
}

func NewClaudeSubscriptionLimitService(
	repo ClaudeSubscriptionLimitRepository,
	usageRepo UsageLogRepository,
	invalidator APIKeyAuthCacheInvalidator,
) *ClaudeSubscriptionLimitService {
	return &ClaudeSubscriptionLimitService{
		repo:            repo,
		usageRepo:       usageRepo,
		invalidator:     invalidator,
		runtimeState:    make(map[string]claudeQuotaRuntimeState),
		activeSourceIDs: make(map[int64]bool),
	}
}

func claudeQuotaStateKey(sourceAccountID int64, windowType string) string {
	return fmt.Sprintf("%d:%s", sourceAccountID, windowType)
}

func (s *ClaudeSubscriptionLimitService) sourceRuntimeState(sample *ClaudeSubscriptionQuotaWindowSample) (claudeQuotaRuntimeState, bool) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	state, ok := s.runtimeState[claudeQuotaStateKey(sample.SourceAccountID, sample.WindowType)]
	if !ok || !state.windowStart.Equal(sample.WindowStart) || !state.windowEnd.Equal(sample.WindowEnd) {
		return claudeQuotaRuntimeState{}, false
	}
	return state, true
}

func (s *ClaudeSubscriptionLimitService) setSourceRuntimeState(sample *ClaudeSubscriptionQuotaWindowSample, threshold int) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.runtimeState[claudeQuotaStateKey(sample.SourceAccountID, sample.WindowType)] = claudeQuotaRuntimeState{
		windowStart:   sample.WindowStart,
		windowEnd:     sample.WindowEnd,
		lastThreshold: threshold,
	}
}

func (s *ClaudeSubscriptionLimitService) invalidateActiveSourceCache() {
	s.sourceMu.Lock()
	s.sourceLoaded = false
	s.sourceExpires = time.Time{}
	s.sourceMu.Unlock()
}

func (s *ClaudeSubscriptionLimitService) isActiveSourceAccount(ctx context.Context, accountID int64) (bool, error) {
	if accountID <= 0 {
		return false, nil
	}
	now := time.Now()
	s.sourceMu.Lock()
	if s.sourceLoaded && now.Before(s.sourceExpires) {
		active := s.activeSourceIDs[accountID]
		s.sourceMu.Unlock()
		return active, nil
	}
	s.sourceMu.Unlock()

	ids, err := s.repo.ListActiveSourceAccountIDs(ctx)
	if err != nil {
		s.sourceMu.Lock()
		active := s.activeSourceIDs[accountID]
		loaded := s.sourceLoaded
		s.sourceMu.Unlock()
		if loaded {
			return active, nil
		}
		return false, err
	}
	next := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id > 0 {
			next[id] = true
		}
	}
	s.sourceMu.Lock()
	s.activeSourceIDs = next
	s.sourceLoaded = true
	s.sourceExpires = time.Now().Add(claudeSourceCacheTTL)
	active := next[accountID]
	s.sourceMu.Unlock()
	return active, nil
}

// ObservePassiveUsage is called from RateLimitService after it has persisted the
// latest passive utilization snapshot. For each active source account window
// (5h / 7d independently), it recalculates and immediately refreshes limits when
// utilization reaches 50%, 75%, and 85%.
func (s *ClaudeSubscriptionLimitService) ObservePassiveUsage(
	ctx context.Context,
	account *Account,
	headers http.Header,
	extraUpdates map[string]any,
) {
	if s == nil || account == nil || !account.IsAnthropicOAuthOrSetupToken() {
		return
	}
	active, err := s.isActiveSourceAccount(ctx, account.ID)
	if err != nil {
		slog.Warn("claude_subscription_limit_source_lookup_failed", "account_id", account.ID, "error", err)
		return
	}
	if !active {
		return
	}

	now := time.Now()
	samples := claudePassiveQuotaSamples(account, headers, extraUpdates, now)
	for _, sample := range samples {
		threshold := claudeThresholdForUtilization(sample.UtilizationPercent)
		if threshold == 0 {
			continue
		}
		if state, ok := s.sourceRuntimeState(&sample); ok && threshold <= state.lastThreshold {
			continue
		}
		sampleCopy := sample
		thresholdCopy := threshold
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					slog.Error("claude_subscription_limit_observe_panic",
						"account_id", sampleCopy.SourceAccountID,
						"window", sampleCopy.WindowType,
						"panic", recovered,
					)
				}
			}()
			processCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			defer cancel()
			if err := s.processQuotaSample(processCtx, &sampleCopy, thresholdCopy); err != nil {
				slog.Warn("claude_subscription_limit_update_failed",
					"account_id", sampleCopy.SourceAccountID,
					"window", sampleCopy.WindowType,
					"threshold", thresholdCopy,
					"error", err,
				)
			}
		}()
	}
}

func claudePassiveQuotaSamples(account *Account, headers http.Header, extraUpdates map[string]any, now time.Time) []ClaudeSubscriptionQuotaWindowSample {
	if account == nil {
		return nil
	}
	samples := make([]ClaudeSubscriptionQuotaWindowSample, 0, 2)
	extra := account.Extra

	util5hRaw, util5hPresent := claudeSnapshotValue(extraUpdates, extra, "session_window_utilization")
	if util5hPresent {
		if util5h, ok := claudeFloatValue(util5hRaw); ok && util5h > 0 {
			reset5h := claudeFiveHourResetAt(account, headers, now)
			if reset5h != nil && reset5h.After(now) {
				samples = append(samples, ClaudeSubscriptionQuotaWindowSample{
					SourceAccountID:    account.ID,
					WindowType:         ClaudeSubscriptionWindowFiveHour,
					WindowStart:        reset5h.Add(-claudeQuotaWindowFiveHours),
					WindowEnd:          *reset5h,
					UtilizationPercent: util5h * 100,
					SampledAt:          now,
				})
			}
		}
	}

	util7dRaw, util7dPresent := claudeSnapshotValue(extraUpdates, extra, "passive_usage_7d_utilization")
	if util7dPresent {
		if util7d, ok := claudeFloatValue(util7dRaw); ok && util7d > 0 {
			reset7d := claudeSevenDayResetAt(account, headers, extraUpdates)
			if reset7d != nil && reset7d.After(now) {
				samples = append(samples, ClaudeSubscriptionQuotaWindowSample{
					SourceAccountID:    account.ID,
					WindowType:         ClaudeSubscriptionWindowSevenDay,
					WindowStart:        reset7d.Add(-claudeQuotaWindowSevenDays),
					WindowEnd:          *reset7d,
					UtilizationPercent: util7d * 100,
					SampledAt:          now,
				})
			}
		}
	}
	return samples
}

func claudeSnapshotValue(updates map[string]any, fallback map[string]any, key string) (any, bool) {
	if updates != nil {
		if value, ok := updates[key]; ok {
			return value, true
		}
	}
	if fallback != nil {
		value, ok := fallback[key]
		return value, ok
	}
	return nil, false
}

func claudeFloatValue(v any) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		f, err := value.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(value, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func claudeUnixValueFromSnapshot(updates map[string]any, fallback map[string]any, key string) (time.Time, bool) {
	raw, ok := claudeSnapshotValue(updates, fallback, key)
	if !ok {
		return time.Time{}, false
	}
	value, ok := claudeFloatValue(raw)
	if !ok || value <= 0 {
		return time.Time{}, false
	}
	if value > 1e11 {
		value /= 1000
	}
	return time.Unix(int64(value), 0), true
}

func claudeHeaderUnixTime(headers http.Header, key string) (time.Time, bool) {
	if headers == nil {
		return time.Time{}, false
	}
	raw := headers.Get(key)
	if raw == "" {
		return time.Time{}, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return time.Time{}, false
	}
	if value > 1e11 {
		value /= 1000
	}
	return time.Unix(int64(value), 0), true
}

func claudeFiveHourResetAt(account *Account, headers http.Header, now time.Time) *time.Time {
	if reset, ok := claudeHeaderUnixTime(headers, "anthropic-ratelimit-unified-5h-reset"); ok {
		return &reset
	}
	if account != nil && account.SessionWindowEnd != nil && account.SessionWindowEnd.After(now) {
		reset := *account.SessionWindowEnd
		return &reset
	}
	return nil
}

func claudeSevenDayResetAt(account *Account, headers http.Header, extraUpdates map[string]any) *time.Time {
	if reset, ok := claudeHeaderUnixTime(headers, "anthropic-ratelimit-unified-7d-reset"); ok {
		return &reset
	}
	var extra map[string]any
	if account != nil {
		extra = account.Extra
	}
	if reset, ok := claudeUnixValueFromSnapshot(extraUpdates, extra, "passive_usage_7d_reset"); ok {
		return &reset
	}
	return nil
}

func claudeThresholdForUtilization(utilizationPercent float64) int {
	threshold := 0
	for _, candidate := range ClaudeSubscriptionUtilizationThresholds {
		if utilizationPercent+1e-9 >= float64(candidate) {
			threshold = candidate
		} else {
			break
		}
	}
	return threshold
}

func claudeEstimateFullQuota(windowCostUSD, utilizationPercent float64) float64 {
	if windowCostUSD <= 0 || utilizationPercent <= 0 {
		return 0
	}
	return roundClaudeMoney(windowCostUSD / (utilizationPercent / 100))
}

func claudeCalculateGroupLimit(estimateFullUSD, capPercent float64, shareCount int, offsetUSD float64) float64 {
	if shareCount <= 0 {
		return 0
	}
	value := estimateFullUSD*capPercent/100/float64(shareCount) + offsetUSD
	if value < claudeLimitMinimumPositive {
		value = claudeLimitMinimumPositive
	}
	return roundClaudeMoney(value)
}

func roundClaudeMoney(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*claudeLimitRoundScale) / claudeLimitRoundScale
}

func claudeDefaultFullEstimate(windowType string) float64 {
	if windowType == ClaudeSubscriptionWindowFiveHour {
		return ClaudeSubscriptionDefaultFiveHourFullUSD
	}
	return ClaudeSubscriptionDefaultSevenDayFullUSD
}

func claudeGroupInitialDefault(cfg *ClaudeSubscriptionGroupConfig, windowType string) float64 {
	if cfg != nil {
		if windowType == ClaudeSubscriptionWindowFiveHour && cfg.InitialFiveHourFullUSD > 0 {
			return cfg.InitialFiveHourFullUSD
		}
		if windowType == ClaudeSubscriptionWindowSevenDay && cfg.InitialSevenDayFullUSD > 0 {
			return cfg.InitialSevenDayFullUSD
		}
	}
	return claudeDefaultFullEstimate(windowType)
}

func isSupportedClaudeSourceAccount(acc ClaudeSubscriptionBoundAccount) bool {
	return acc.Platform == PlatformAnthropic && (acc.Type == AccountTypeOAuth || acc.Type == AccountTypeSetupToken)
}

func resolveGroupSourceAccount(bound []ClaudeSubscriptionBoundAccountDetail) (
	status string,
	warning string,
	source *ClaudeSubscriptionSourceAccount,
	summaries []ClaudeSubscriptionBoundAccount,
) {
	summaries = make([]ClaudeSubscriptionBoundAccount, 0, len(bound))
	for _, item := range bound {
		summaries = append(summaries, item.ClaudeSubscriptionBoundAccount)
	}

	switch len(bound) {
	case 0:
		return ClaudeSubscriptionSourceStatusMissing,
			"该分组尚未绑定上游账号，请先绑定唯一的 Anthropic OAuth / Setup Token 账号",
			nil,
			summaries
	case 1:
		only := bound[0]
		if !isSupportedClaudeSourceAccount(only.ClaudeSubscriptionBoundAccount) {
			return ClaudeSubscriptionSourceStatusUnsupported,
				fmt.Sprintf("该分组绑定的账号「%s」（%s / %s）不是 Anthropic OAuth 或 Setup Token，无法采集 5h/7d 被动用量",
					only.Name, only.Platform, only.Type),
				nil,
				summaries
		}
		return ClaudeSubscriptionSourceStatusOK, "", &ClaudeSubscriptionSourceAccount{
			ClaudeSubscriptionSourceAccountOption: ClaudeSubscriptionSourceAccountOption{
				ID:     only.ID,
				Name:   only.Name,
				Email:  only.Email,
				Type:   only.Type,
				Status: only.Status,
			},
			FiveHourUtilizationPercent: only.FiveHourUtilizationPercent,
			SevenDayUtilizationPercent: only.SevenDayUtilizationPercent,
			FiveHourResetsAt:           only.FiveHourResetsAt,
			SevenDayResetsAt:           only.SevenDayResetsAt,
		}, summaries
	default:
		names := make([]string, 0, min(len(bound), 4))
		for i, item := range bound {
			if i >= 4 {
				break
			}
			names = append(names, item.Name)
		}
		listed := strings.Join(names, "、")
		if len(bound) > len(names) {
			listed = fmt.Sprintf("%s 等 %d 个账号", listed, len(bound))
		}
		return ClaudeSubscriptionSourceStatusMultiple,
			fmt.Sprintf("该分组同时绑定了 %d 个账号（%s），无法确定唯一上游源账号，拒绝开启或已暂停动态限额", len(bound), listed),
			nil,
			summaries
	}
}

func claudeSourceAccountWindowResetAt(sourceAccount *ClaudeSubscriptionSourceAccount, windowType string) *time.Time {
	if sourceAccount == nil {
		return nil
	}
	if windowType == ClaudeSubscriptionWindowFiveHour {
		return sourceAccount.FiveHourResetsAt
	}
	return sourceAccount.SevenDayResetsAt
}

func claudeAbsDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func buildAppliedEstimate(
	est *ClaudeSubscriptionQuotaEstimateRef,
	sourceAccount *ClaudeSubscriptionSourceAccount,
	windowType string,
	initialDefaultUSD float64,
	now time.Time,
) ClaudeSubscriptionAppliedEstimate {
	if est != nil && est.EstimatedFullUSD > 0 {
		source := ClaudeSubscriptionEstimateSourcePrevious
		if est.WindowEnd.After(now) {
			activeReset := claudeSourceAccountWindowResetAt(sourceAccount, windowType)
			if activeReset == nil || claudeAbsDuration(est.WindowEnd.Sub(*activeReset)) <= claudeWindowResetMatchWindow {
				source = ClaudeSubscriptionEstimateSourceCurrent
			}
		}
		threshold := est.ThresholdPercent
		windowStart := est.WindowStart
		windowEnd := est.WindowEnd
		sampledAt := est.SampledAt
		return ClaudeSubscriptionAppliedEstimate{
			ValueUSD:         est.EstimatedFullUSD,
			Source:           source,
			ThresholdPercent: &threshold,
			WindowStart:      &windowStart,
			WindowEnd:        &windowEnd,
			SampledAt:        &sampledAt,
		}
	}
	return ClaudeSubscriptionAppliedEstimate{
		ValueUSD: initialDefaultUSD,
		Source:   ClaudeSubscriptionEstimateSourceDefault,
	}
}

func (s *ClaudeSubscriptionLimitService) processQuotaSample(ctx context.Context, sample *ClaudeSubscriptionQuotaWindowSample, threshold int) error {
	if threshold < ClaudeSubscriptionMinEffectiveThreshold {
		return nil
	}

	s.processMu.Lock()
	defer s.processMu.Unlock()

	if state, ok := s.sourceRuntimeState(sample); ok && threshold <= state.lastThreshold {
		return nil
	}

	// Re-check live single-source eligibility so groups that gained extra accounts
	// are paused immediately and never updated on stale cache data.
	configs, err := s.repo.ListEnabledGroupConfigsBySourceAccount(ctx, sample.SourceAccountID)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}

	cycle, err := s.repo.EnsureQuotaCycle(ctx, sample.SourceAccountID, sample.WindowType, sample.WindowStart, sample.WindowEnd)
	if err != nil {
		return err
	}
	if threshold <= cycle.LastThresholdPercent {
		s.setSourceRuntimeState(sample, cycle.LastThresholdPercent)
		return nil
	}

	stats, err := s.usageRepo.GetAccountWindowStats(ctx, sample.SourceAccountID, sample.WindowStart)
	if err != nil {
		return fmt.Errorf("get source window stats: %w", err)
	}
	if stats == nil || stats.Cost <= 0 {
		return errors.New("source window cost is not available")
	}
	sample.WindowCostUSD = stats.Cost
	estimatedFullUSD := claudeEstimateFullQuota(sample.WindowCostUSD, sample.UtilizationPercent)
	if estimatedFullUSD <= 0 {
		return errors.New("estimated source full quota is not positive")
	}

	inserted, err := s.repo.RecordQuotaEstimate(ctx, sample, threshold, estimatedFullUSD)
	if err != nil {
		return err
	}
	if !inserted {
		// Another worker/instance already recorded this (account, window, start, threshold).
		s.setSourceRuntimeState(sample, threshold)
		return nil
	}

	thresholdCopy := threshold
	for i := range configs {
		cfg := configs[i]
		capPercent, shareCount, offsetUSD := claudeWindowConfig(&cfg, sample.WindowType)
		newLimit := claudeCalculateGroupLimit(estimatedFullUSD, capPercent, shareCount, offsetUSD)
		result, err := s.repo.ApplyGroupWindowLimit(ctx, &ClaudeSubscriptionGroupLimitApplyInput{
			SourceAccountID:          sample.SourceAccountID,
			GroupID:                  cfg.GroupID,
			WindowType:               sample.WindowType,
			TriggerType:              ClaudeSubscriptionTriggerThreshold,
			ThresholdPercent:         &thresholdCopy,
			SourceWindowStart:        &sample.WindowStart,
			SourceWindowEnd:          &sample.WindowEnd,
			SourceUtilizationPercent: &sample.UtilizationPercent,
			SourceWindowCostUSD:      &sample.WindowCostUSD,
			SourceEstimatedFullUSD:   &estimatedFullUSD,
			AppliedEstimateUSD:       estimatedFullUSD,
			AppliedEstimateSource:    ClaudeSubscriptionEstimateSourceCurrent,
			CapPercent:               capPercent,
			ShareCount:               shareCount,
			ManualOffsetUSD:          offsetUSD,
			NewLimitUSD:              newLimit,
		})
		if err != nil {
			return fmt.Errorf("apply group %d %s limit: %w", cfg.GroupID, sample.WindowType, err)
		}
		if result.Changed && s.invalidator != nil {
			s.invalidator.InvalidateAuthCacheByGroupID(context.WithoutCancel(ctx), cfg.GroupID)
		}
	}

	if err := s.repo.AdvanceQuotaCycle(ctx, sample, threshold, estimatedFullUSD); err != nil {
		return err
	}
	s.setSourceRuntimeState(sample, threshold)
	return nil
}

func claudeWindowConfig(cfg *ClaudeSubscriptionGroupConfig, windowType string) (float64, int, float64) {
	if windowType == ClaudeSubscriptionWindowFiveHour {
		return cfg.FiveHourCapPercent, cfg.FiveHourShareCount, cfg.FiveHourOffsetUSD
	}
	return cfg.SevenDayCapPercent, cfg.SevenDayShareCount, cfg.SevenDayOffsetUSD
}

func (s *ClaudeSubscriptionLimitService) GetOverview(ctx context.Context) (*ClaudeSubscriptionLimitOverview, error) {
	if err := s.repo.EnsureGroupConfigs(ctx); err != nil {
		return nil, err
	}
	sourceAccounts, err := s.repo.ListSourceAccountOptions(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroupConfigs(ctx)
	if err != nil {
		return nil, err
	}

	groupIDs := make([]int64, 0, len(groups))
	for i := range groups {
		groupIDs = append(groupIDs, groups[i].GroupID)
	}
	boundByGroup, err := s.repo.ListBoundAccountsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	estimateCache := make(map[string]*ClaudeSubscriptionQuotaEstimateRef)
	cycleCache := make(map[string]*ClaudeSubscriptionQuotaCycle)

	getEffectiveEstimate := func(sourceID int64, windowType string) *ClaudeSubscriptionQuotaEstimateRef {
		key := claudeQuotaStateKey(sourceID, windowType)
		if cached, ok := estimateCache[key]; ok {
			return cached
		}
		est, err := s.repo.GetLatestEffectiveEstimate(ctx, sourceID, windowType, ClaudeSubscriptionMinEffectiveThreshold)
		if err != nil {
			slog.Warn("claude_subscription_limit_effective_estimate_failed",
				"source_account_id", sourceID,
				"window", windowType,
				"error", err,
			)
			est = nil
		}
		estimateCache[key] = est
		return est
	}

	getCurrentCycle := func(sourceID int64, windowType string) *ClaudeSubscriptionQuotaCycle {
		key := claudeQuotaStateKey(sourceID, windowType)
		if cached, ok := cycleCache[key]; ok {
			return cached
		}
		cycle, err := s.repo.GetCurrentCycle(ctx, sourceID, windowType)
		if err != nil {
			slog.Warn("claude_subscription_limit_current_cycle_failed",
				"source_account_id", sourceID,
				"window", windowType,
				"error", err,
			)
			cycle = nil
		}
		cycleCache[key] = cycle
		return cycle
	}

	for i := range groups {
		g := &groups[i]
		status, warning, sourceAcc, summaries := resolveGroupSourceAccount(boundByGroup[g.GroupID])
		g.SourceStatus = status
		g.SourceWarning = warning
		g.BoundAccounts = summaries
		g.AppliedEstimates = make(map[string]ClaudeSubscriptionAppliedEstimate, 2)
		g.CurrentCycles = make(map[string]*ClaudeSubscriptionQuotaCycle, 2)

		if sourceAcc != nil {
			id := sourceAcc.ID
			g.SourceAccountID = &id
			g.SourceAccount = sourceAcc
		}

		for _, windowType := range []string{ClaudeSubscriptionWindowFiveHour, ClaudeSubscriptionWindowSevenDay} {
			var est *ClaudeSubscriptionQuotaEstimateRef
			if sourceAcc != nil {
				est = getEffectiveEstimate(sourceAcc.ID, windowType)
				if cycle := getCurrentCycle(sourceAcc.ID, windowType); cycle != nil {
					g.CurrentCycles[windowType] = cycle
				}
			}
			g.AppliedEstimates[windowType] = buildAppliedEstimate(
				est,
				sourceAcc,
				windowType,
				claudeGroupInitialDefault(g, windowType),
				now,
			)
		}
	}

	thresholds := make([]int, len(ClaudeSubscriptionUtilizationThresholds))
	copy(thresholds, ClaudeSubscriptionUtilizationThresholds)

	return &ClaudeSubscriptionLimitOverview{
		Thresholds:     thresholds,
		SourceAccounts: sourceAccounts,
		Groups:         groups,
	}, nil
}

func (s *ClaudeSubscriptionLimitService) UpdateGroupConfig(ctx context.Context, groupID int64, update *ClaudeSubscriptionGroupConfigUpdate) error {
	if groupID <= 0 || update == nil {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_CONFIG", "分组配置参数无效")
	}
	if err := validateClaudeGroupConfigUpdate(update); err != nil {
		return err
	}
	cfg, err := s.repo.GetGroupConfig(ctx, groupID)
	if err != nil {
		return err
	}

	boundByGroup, err := s.repo.ListBoundAccountsByGroupIDs(ctx, []int64{groupID})
	if err != nil {
		return err
	}
	sourceStatus, sourceWarning, sourceAccount, _ := resolveGroupSourceAccount(boundByGroup[groupID])
	if update.AutoEnabled && sourceStatus != ClaudeSubscriptionSourceStatusOK {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_SOURCE_INVALID", sourceWarning)
	}

	cfg.AutoEnabled = update.AutoEnabled
	cfg.InitialFiveHourFullUSD = roundClaudeMoney(update.InitialFiveHourFullUSD)
	cfg.InitialSevenDayFullUSD = roundClaudeMoney(update.InitialSevenDayFullUSD)
	cfg.FiveHourCapPercent = update.FiveHourCapPercent
	cfg.SevenDayCapPercent = update.SevenDayCapPercent
	cfg.FiveHourShareCount = update.FiveHourShareCount
	cfg.SevenDayShareCount = update.SevenDayShareCount
	cfg.FiveHourOffsetUSD = update.FiveHourOffsetUSD
	cfg.SevenDayOffsetUSD = update.SevenDayOffsetUSD

	if err := s.repo.UpsertGroupConfig(ctx, cfg); err != nil {
		return err
	}
	s.invalidateActiveSourceCache()
	if !cfg.AutoEnabled || sourceAccount == nil {
		return nil
	}
	return s.applyCurrentEstimatesForGroup(ctx, cfg, sourceAccount, ClaudeSubscriptionTriggerConfigUpdate)
}

func validateClaudeGroupConfigUpdate(update *ClaudeSubscriptionGroupConfigUpdate) error {
	if math.IsNaN(update.InitialFiveHourFullUSD) || math.IsInf(update.InitialFiveHourFullUSD, 0) ||
		update.InitialFiveHourFullUSD <= 0 || update.InitialFiveHourFullUSD > claudeMaxInitialFiveHourUSD {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_INITIAL_5H", "5h 初始满额默认值必须大于 0 且不超过 1000000")
	}
	if math.IsNaN(update.InitialSevenDayFullUSD) || math.IsInf(update.InitialSevenDayFullUSD, 0) ||
		update.InitialSevenDayFullUSD <= 0 || update.InitialSevenDayFullUSD > claudeMaxInitialSevenDayUSD {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_INITIAL_7D", "7d 初始满额默认值必须大于 0 且不超过 10000000")
	}
	if math.IsNaN(update.FiveHourCapPercent) || math.IsInf(update.FiveHourCapPercent, 0) ||
		update.FiveHourCapPercent < 0 || update.FiveHourCapPercent > 100 {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_CAP_5H", "5h 百分比上限必须在 0 到 100 之间")
	}
	if math.IsNaN(update.SevenDayCapPercent) || math.IsInf(update.SevenDayCapPercent, 0) ||
		update.SevenDayCapPercent < 0 || update.SevenDayCapPercent > 100 {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_CAP_7D", "7d 百分比上限必须在 0 到 100 之间")
	}
	if update.FiveHourShareCount <= 0 || update.FiveHourShareCount > 1000 {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_SHARES_5H", "5h 平分份数必须在 1 到 1000 之间")
	}
	if update.SevenDayShareCount <= 0 || update.SevenDayShareCount > 1000 {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_SHARES_7D", "7d 平分份数必须在 1 到 1000 之间")
	}
	if math.IsNaN(update.FiveHourOffsetUSD) || math.IsInf(update.FiveHourOffsetUSD, 0) ||
		math.IsNaN(update.SevenDayOffsetUSD) || math.IsInf(update.SevenDayOffsetUSD, 0) {
		return infraerrors.BadRequest("CLAUDE_SUBSCRIPTION_LIMIT_INVALID_OFFSET", "手动偏移必须是有限数值")
	}
	return nil
}

func (s *ClaudeSubscriptionLimitService) applyCurrentEstimatesForGroup(
	ctx context.Context,
	cfg *ClaudeSubscriptionGroupConfig,
	sourceAccount *ClaudeSubscriptionSourceAccount,
	triggerType string,
) error {
	now := time.Now()
	for _, windowType := range []string{ClaudeSubscriptionWindowFiveHour, ClaudeSubscriptionWindowSevenDay} {
		est, err := s.repo.GetLatestEffectiveEstimate(ctx, sourceAccount.ID, windowType, ClaudeSubscriptionMinEffectiveThreshold)
		if err != nil {
			slog.Warn("claude_subscription_limit_effective_estimate_failed",
				"source_account_id", sourceAccount.ID,
				"window", windowType,
				"error", err,
			)
			est = nil
		}
		applied := buildAppliedEstimate(est, sourceAccount, windowType, claudeGroupInitialDefault(cfg, windowType), now)
		capPercent, shareCount, offsetUSD := claudeWindowConfig(cfg, windowType)
		newLimit := claudeCalculateGroupLimit(applied.ValueUSD, capPercent, shareCount, offsetUSD)
		result, err := s.repo.ApplyGroupWindowLimit(ctx, &ClaudeSubscriptionGroupLimitApplyInput{
			SourceAccountID:       sourceAccount.ID,
			GroupID:               cfg.GroupID,
			WindowType:            windowType,
			TriggerType:           triggerType,
			SourceWindowStart:     applied.WindowStart,
			SourceWindowEnd:       applied.WindowEnd,
			AppliedEstimateUSD:    applied.ValueUSD,
			AppliedEstimateSource: applied.Source,
			CapPercent:            capPercent,
			ShareCount:            shareCount,
			ManualOffsetUSD:       offsetUSD,
			NewLimitUSD:           newLimit,
		})
		if err != nil {
			return err
		}
		if result.Changed && s.invalidator != nil {
			s.invalidator.InvalidateAuthCacheByGroupID(context.WithoutCancel(ctx), cfg.GroupID)
		}
	}
	return nil
}

func (s *ClaudeSubscriptionLimitService) ListQuotaEstimates(
	ctx context.Context,
	filter *ClaudeSubscriptionQuotaEstimateHistoryFilter,
) ([]ClaudeSubscriptionQuotaEstimateHistory, int64, error) {
	return s.repo.ListQuotaEstimates(ctx, filter)
}

func (s *ClaudeSubscriptionLimitService) ListLimitAdjustments(
	ctx context.Context,
	filter *ClaudeSubscriptionLimitAdjustmentHistoryFilter,
) ([]ClaudeSubscriptionLimitAdjustmentHistory, int64, error) {
	return s.repo.ListLimitAdjustments(ctx, filter)
}
