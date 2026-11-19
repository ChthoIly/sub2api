package service

import (
	"context"
	"time"
)

const (
	ClaudeSubscriptionWindowFiveHour = "5h"
	ClaudeSubscriptionWindowSevenDay = "7d"
)

const (
	ClaudeSubscriptionDefaultFiveHourFullUSD = 100.0
	ClaudeSubscriptionDefaultSevenDayFullUSD = 1200.0
)

const (
	ClaudeSubscriptionEstimateSourceCurrent  = "current_cycle"
	ClaudeSubscriptionEstimateSourcePrevious = "previous_cycle"
	ClaudeSubscriptionEstimateSourceDefault  = "default"
)

const (
	ClaudeSubscriptionSourceStatusOK          = "ok"
	ClaudeSubscriptionSourceStatusMissing     = "missing"
	ClaudeSubscriptionSourceStatusMultiple    = "multiple"
	ClaudeSubscriptionSourceStatusUnsupported = "unsupported"
)

const (
	ClaudeSubscriptionTriggerThreshold    = "threshold"
	ClaudeSubscriptionTriggerConfigUpdate = "config_update"
	ClaudeSubscriptionTriggerSourceUpdate = "source_update"
)

const ClaudeSubscriptionMinEffectiveThreshold = 50

// ClaudeSubscriptionUtilizationThresholds defines the source-account utilization
// checkpoints (in percent) that recalculate and refresh group limits within a
// 5h / 7d window cycle: first at 50%, then refreshed at 75% and 85%.
var ClaudeSubscriptionUtilizationThresholds = []int{50, 75, 85}

// ClaudeSubscriptionSourceAccountOption is a selectable/filterable passive-usage source.
type ClaudeSubscriptionSourceAccountOption struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// ClaudeSubscriptionSourceAccount contains the current passive snapshot for a group's source account.
type ClaudeSubscriptionSourceAccount struct {
	ClaudeSubscriptionSourceAccountOption
	FiveHourUtilizationPercent *float64   `json:"five_hour_utilization_percent"`
	SevenDayUtilizationPercent *float64   `json:"seven_day_utilization_percent"`
	FiveHourResetsAt           *time.Time `json:"five_hour_resets_at"`
	SevenDayResetsAt           *time.Time `json:"seven_day_resets_at"`
}

// ClaudeSubscriptionBoundAccount summarizes an account bound to a subscription group.
type ClaudeSubscriptionBoundAccount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

// ClaudeSubscriptionBoundAccountDetail includes passive usage fields so overview
// loading resolves every group's bound account(s) without N+1 queries.
type ClaudeSubscriptionBoundAccountDetail struct {
	ClaudeSubscriptionBoundAccount
	FiveHourUtilizationPercent *float64
	SevenDayUtilizationPercent *float64
	FiveHourResetsAt           *time.Time
	SevenDayResetsAt           *time.Time
}

// ClaudeSubscriptionQuotaCycle tracks one source-account usage window.
type ClaudeSubscriptionQuotaCycle struct {
	ID                     int64      `json:"id"`
	SourceAccountID        int64      `json:"source_account_id"`
	WindowType             string     `json:"window_type"`
	WindowStart            time.Time  `json:"window_start"`
	WindowEnd              time.Time  `json:"window_end"`
	Status                 string     `json:"status"`
	LastThresholdPercent   int        `json:"last_threshold_percent"`
	LastUtilizationPercent *float64   `json:"last_utilization_percent"`
	LastWindowCostUSD      *float64   `json:"last_window_cost_usd"`
	LatestEstimatedFullUSD *float64   `json:"latest_estimated_full_usd"`
	LastSampledAt          *time.Time `json:"last_sampled_at"`
	CompletedAt            *time.Time `json:"completed_at"`
}

// ClaudeSubscriptionQuotaEstimateRef references an effective recorded estimate.
type ClaudeSubscriptionQuotaEstimateRef struct {
	EstimatedFullUSD float64   `json:"estimated_full_usd"`
	WindowStart      time.Time `json:"window_start"`
	WindowEnd        time.Time `json:"window_end"`
	ThresholdPercent int       `json:"threshold_percent"`
	SampledAt        time.Time `json:"sampled_at"`
}

// ClaudeSubscriptionAppliedEstimate describes which full-quota estimate is currently applied.
type ClaudeSubscriptionAppliedEstimate struct {
	ValueUSD         float64    `json:"value_usd"`
	Source           string     `json:"source"`
	ThresholdPercent *int       `json:"threshold_percent,omitempty"`
	WindowStart      *time.Time `json:"window_start,omitempty"`
	WindowEnd        *time.Time `json:"window_end,omitempty"`
	SampledAt        *time.Time `json:"sampled_at,omitempty"`
}

// ClaudeSubscriptionGroupConfig is the persisted per-group dynamic-limit config
// enriched with its resolved upstream source account and effective estimates.
type ClaudeSubscriptionGroupConfig struct {
	GroupID                 int64                                        `json:"group_id"`
	GroupName               string                                       `json:"group_name"`
	GroupPlatform           string                                       `json:"group_platform"`
	GroupStatus             string                                       `json:"group_status"`
	SubscriptionCount       int64                                        `json:"subscription_count"`
	CurrentFiveHourLimitUSD *float64                                     `json:"current_five_hour_limit_usd"`
	CurrentSevenDayLimitUSD *float64                                     `json:"current_seven_day_limit_usd"`
	AutoEnabled             bool                                         `json:"auto_enabled"`
	SourceStatus            string                                       `json:"source_status"`
	SourceWarning           string                                       `json:"source_warning,omitempty"`
	SourceAccountID         *int64                                       `json:"source_account_id"`
	SourceAccount           *ClaudeSubscriptionSourceAccount             `json:"source_account,omitempty"`
	BoundAccounts           []ClaudeSubscriptionBoundAccount             `json:"bound_accounts"`
	AppliedEstimates        map[string]ClaudeSubscriptionAppliedEstimate `json:"applied_estimates"`
	CurrentCycles           map[string]*ClaudeSubscriptionQuotaCycle     `json:"current_cycles,omitempty"`
	InitialFiveHourFullUSD  float64                                      `json:"initial_five_hour_full_usd"`
	InitialSevenDayFullUSD  float64                                      `json:"initial_seven_day_full_usd"`
	FiveHourCapPercent      float64                                      `json:"five_hour_cap_percent"`
	SevenDayCapPercent      float64                                      `json:"seven_day_cap_percent"`
	FiveHourShareCount      int                                          `json:"five_hour_share_count"`
	SevenDayShareCount      int                                          `json:"seven_day_share_count"`
	FiveHourOffsetUSD       float64                                      `json:"five_hour_offset_usd"`
	SevenDayOffsetUSD       float64                                      `json:"seven_day_offset_usd"`
}

// ClaudeSubscriptionLimitOverview is returned by the standalone configuration page.
type ClaudeSubscriptionLimitOverview struct {
	Thresholds     []int                                   `json:"thresholds"`
	SourceAccounts []ClaudeSubscriptionSourceAccountOption `json:"source_accounts"`
	Groups         []ClaudeSubscriptionGroupConfig         `json:"groups"`
}

// ClaudeSubscriptionGroupConfigUpdate is the editable part of a group config.
type ClaudeSubscriptionGroupConfigUpdate struct {
	AutoEnabled            bool    `json:"auto_enabled"`
	InitialFiveHourFullUSD float64 `json:"initial_five_hour_full_usd"`
	InitialSevenDayFullUSD float64 `json:"initial_seven_day_full_usd"`
	FiveHourCapPercent     float64 `json:"five_hour_cap_percent"`
	SevenDayCapPercent     float64 `json:"seven_day_cap_percent"`
	FiveHourShareCount     int     `json:"five_hour_share_count"`
	SevenDayShareCount     int     `json:"seven_day_share_count"`
	FiveHourOffsetUSD      float64 `json:"five_hour_offset_usd"`
	SevenDayOffsetUSD      float64 `json:"seven_day_offset_usd"`
}

// ClaudeSubscriptionQuotaWindowSample is one passive sample for one source window.
type ClaudeSubscriptionQuotaWindowSample struct {
	SourceAccountID    int64
	WindowType         string
	WindowStart        time.Time
	WindowEnd          time.Time
	UtilizationPercent float64
	WindowCostUSD      float64
	SampledAt          time.Time
}

// ClaudeSubscriptionGroupLimitApplyInput updates one group window and writes per-user history.
type ClaudeSubscriptionGroupLimitApplyInput struct {
	SourceAccountID          int64
	GroupID                  int64
	WindowType               string
	TriggerType              string
	ThresholdPercent         *int
	SourceWindowStart        *time.Time
	SourceWindowEnd          *time.Time
	SourceUtilizationPercent *float64
	SourceWindowCostUSD      *float64
	SourceEstimatedFullUSD   *float64
	AppliedEstimateUSD       float64
	AppliedEstimateSource    string
	CapPercent               float64
	ShareCount               int
	ManualOffsetUSD          float64
	NewLimitUSD              float64
}

// ClaudeSubscriptionGroupLimitApplyResult reports the applied group/user update.
type ClaudeSubscriptionGroupLimitApplyResult struct {
	OldLimitUSD           *float64 `json:"old_limit_usd"`
	NewLimitUSD           float64  `json:"new_limit_usd"`
	Changed               bool     `json:"changed"`
	AffectedSubscriptions int64    `json:"affected_subscriptions"`
	HistoryRows           int64    `json:"history_rows"`
}

// ClaudeSubscriptionQuotaEstimateHistoryFilter filters estimate history.
type ClaudeSubscriptionQuotaEstimateHistoryFilter struct {
	SourceAccountID *int64
	WindowType      string
	StartTime       *time.Time
	EndTime         *time.Time
	Page            int
	PageSize        int
}

// ClaudeSubscriptionQuotaEstimateHistory is one recorded source full-quota estimate.
type ClaudeSubscriptionQuotaEstimateHistory struct {
	ID                 int64     `json:"id"`
	SourceAccountID    int64     `json:"source_account_id"`
	SourceAccountName  string    `json:"source_account_name"`
	SourceAccountEmail string    `json:"source_account_email"`
	WindowType         string    `json:"window_type"`
	WindowStart        time.Time `json:"window_start"`
	WindowEnd          time.Time `json:"window_end"`
	ThresholdPercent   int       `json:"threshold_percent"`
	UtilizationPercent float64   `json:"utilization_percent"`
	WindowCostUSD      float64   `json:"window_cost_usd"`
	EstimatedFullUSD   float64   `json:"estimated_full_usd"`
	SampledAt          time.Time `json:"sampled_at"`
	CreatedAt          time.Time `json:"created_at"`
}

// ClaudeSubscriptionLimitAdjustmentHistoryFilter filters per-user adjustment history.
type ClaudeSubscriptionLimitAdjustmentHistoryFilter struct {
	GroupID     *int64
	WindowType  string
	TriggerType string
	UserKeyword string
	OnlyChanged bool
	StartTime   *time.Time
	EndTime     *time.Time
	Page        int
	PageSize    int
}

// ClaudeSubscriptionLimitAdjustmentHistory is one per-user limit adjustment record.
type ClaudeSubscriptionLimitAdjustmentHistory struct {
	ID                       int64      `json:"id"`
	SourceAccountID          *int64     `json:"source_account_id"`
	SourceAccountName        string     `json:"source_account_name"`
	GroupID                  *int64     `json:"group_id"`
	GroupNameSnapshot        string     `json:"group_name_snapshot"`
	UserID                   *int64     `json:"user_id"`
	UserEmail                string     `json:"user_email"`
	SubscriptionID           *int64     `json:"subscription_id"`
	WindowType               string     `json:"window_type"`
	TriggerType              string     `json:"trigger_type"`
	ThresholdPercent         *int       `json:"threshold_percent"`
	SourceWindowStart        *time.Time `json:"source_window_start"`
	SourceWindowEnd          *time.Time `json:"source_window_end"`
	SourceUtilizationPercent *float64   `json:"source_utilization_percent"`
	SourceWindowCostUSD      *float64   `json:"source_window_cost_usd"`
	SourceEstimatedFullUSD   *float64   `json:"source_estimated_full_usd"`
	AppliedEstimateUSD       float64    `json:"applied_estimate_usd"`
	AppliedEstimateSource    string     `json:"applied_estimate_source"`
	CapPercent               float64    `json:"cap_percent"`
	ShareCount               int        `json:"share_count"`
	ManualOffsetUSD          float64    `json:"manual_offset_usd"`
	OldLimitUSD              *float64   `json:"old_limit_usd"`
	NewLimitUSD              float64    `json:"new_limit_usd"`
	DeltaUSD                 *float64   `json:"delta_usd"`
	DeltaPercent             *float64   `json:"delta_percent"`
	Changed                  bool       `json:"changed"`
	CreatedAt                time.Time  `json:"created_at"`
}

// ClaudeSubscriptionLimitRepository persists config, cycles, estimates, and history.
type ClaudeSubscriptionLimitRepository interface {
	ListSourceAccountOptions(ctx context.Context) ([]ClaudeSubscriptionSourceAccountOption, error)
	EnsureGroupConfigs(ctx context.Context) error
	ListGroupConfigs(ctx context.Context) ([]ClaudeSubscriptionGroupConfig, error)
	GetGroupConfig(ctx context.Context, groupID int64) (*ClaudeSubscriptionGroupConfig, error)
	UpsertGroupConfig(ctx context.Context, cfg *ClaudeSubscriptionGroupConfig) error
	ListBoundAccountsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]ClaudeSubscriptionBoundAccountDetail, error)
	ListActiveSourceAccountIDs(ctx context.Context) ([]int64, error)
	ListEnabledGroupConfigsBySourceAccount(ctx context.Context, sourceAccountID int64) ([]ClaudeSubscriptionGroupConfig, error)
	EnsureQuotaCycle(ctx context.Context, sourceAccountID int64, windowType string, windowStart, windowEnd time.Time) (*ClaudeSubscriptionQuotaCycle, error)
	RecordQuotaEstimate(ctx context.Context, sample *ClaudeSubscriptionQuotaWindowSample, thresholdPercent int, estimatedFullUSD float64) (bool, error)
	AdvanceQuotaCycle(ctx context.Context, sample *ClaudeSubscriptionQuotaWindowSample, thresholdPercent int, estimatedFullUSD float64) error
	GetLatestEffectiveEstimate(ctx context.Context, sourceAccountID int64, windowType string, minThreshold int) (*ClaudeSubscriptionQuotaEstimateRef, error)
	GetCurrentCycle(ctx context.Context, sourceAccountID int64, windowType string) (*ClaudeSubscriptionQuotaCycle, error)
	ApplyGroupWindowLimit(ctx context.Context, input *ClaudeSubscriptionGroupLimitApplyInput) (*ClaudeSubscriptionGroupLimitApplyResult, error)
	ListQuotaEstimates(ctx context.Context, filter *ClaudeSubscriptionQuotaEstimateHistoryFilter) ([]ClaudeSubscriptionQuotaEstimateHistory, int64, error)
	ListLimitAdjustments(ctx context.Context, filter *ClaudeSubscriptionLimitAdjustmentHistoryFilter) ([]ClaudeSubscriptionLimitAdjustmentHistory, int64, error)
}
