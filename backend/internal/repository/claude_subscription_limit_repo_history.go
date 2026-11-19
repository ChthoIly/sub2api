package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func floatPtrValue(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func anyIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func anyTimePtr(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}

func claudeLimitChanged(oldValue *float64, newValue float64) bool {
	if oldValue == nil {
		return true
	}
	return math.Abs(*oldValue-newValue) > 0.00000001
}

func (r *claudeSubscriptionLimitRepository) ApplyGroupWindowLimit(
	ctx context.Context,
	input *service.ClaudeSubscriptionGroupLimitApplyInput,
) (*service.ClaudeSubscriptionGroupLimitApplyResult, error) {
	if input == nil {
		return nil, errors.New("group limit apply input is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claude subscription group limit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		groupName string
		old5h     sql.NullFloat64
		old7d     sql.NullFloat64
	)
	if err := tx.QueryRowContext(ctx, `
SELECT name, five_hour_limit_usd, weekly_limit_usd
FROM groups
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE
`, input.GroupID).Scan(&groupName, &old5h, &old7d); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("claude subscription group not found")
		}
		return nil, fmt.Errorf("lock claude subscription group: %w", err)
	}

	var oldLimit *float64
	if input.WindowType == service.ClaudeSubscriptionWindowFiveHour {
		if old5h.Valid {
			v := old5h.Float64
			oldLimit = &v
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE groups
SET five_hour_limit_usd = $2, updated_at = NOW()
WHERE id = $1 AND five_hour_limit_usd IS DISTINCT FROM $2
`, input.GroupID, input.NewLimitUSD); err != nil {
			return nil, fmt.Errorf("update claude subscription five-hour limit: %w", err)
		}
	} else {
		if old7d.Valid {
			v := old7d.Float64
			oldLimit = &v
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE groups
SET weekly_limit_usd = $2, updated_at = NOW()
WHERE id = $1 AND weekly_limit_usd IS DISTINCT FROM $2
`, input.GroupID, input.NewLimitUSD); err != nil {
			return nil, fmt.Errorf("update claude subscription seven-day limit: %w", err)
		}
	}

	changed := claudeLimitChanged(oldLimit, input.NewLimitUSD)
	var delta any
	var deltaPercent any
	if oldLimit != nil {
		d := input.NewLimitUSD - *oldLimit
		delta = d
		if *oldLimit > 0 {
			deltaPercent = d / *oldLimit * 100
		}
	}

	insertArgs := []any{
		input.SourceAccountID,
		input.GroupID,
		groupName,
		input.WindowType,
		input.TriggerType,
		anyIntPtr(input.ThresholdPercent),
		anyTimePtr(input.SourceWindowStart),
		anyTimePtr(input.SourceWindowEnd),
		floatPtrValue(input.SourceUtilizationPercent),
		floatPtrValue(input.SourceWindowCostUSD),
		floatPtrValue(input.SourceEstimatedFullUSD),
		input.AppliedEstimateUSD,
		input.AppliedEstimateSource,
		input.CapPercent,
		input.ShareCount,
		input.ManualOffsetUSD,
		floatPtrValue(oldLimit),
		input.NewLimitUSD,
		delta,
		deltaPercent,
		changed,
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO claude_subscription_limit_adjustments (
source_account_id, group_id, group_name_snapshot,
user_id, user_email_snapshot, subscription_id,
window_type, trigger_type, threshold_percent,
source_window_start, source_window_end,
source_utilization_percent, source_window_cost_usd, source_estimated_full_usd,
applied_estimate_usd, applied_estimate_source,
cap_percent, share_count, manual_offset_usd,
old_limit_usd, new_limit_usd, delta_usd, delta_percent, changed
)
SELECT
$1, $2, $3,
us.user_id, COALESCE(u.email, ''), us.id,
$4, $5, $6,
$7, $8,
$9, $10, $11,
$12, $13,
$14, $15, $16,
$17, $18, $19, $20, $21
FROM user_subscriptions us
LEFT JOIN users u ON u.id = us.user_id
WHERE us.group_id = $2
  AND us.deleted_at IS NULL
  AND us.status = 'active'
  AND us.expires_at > NOW()
`, insertArgs...)
	if err != nil {
		return nil, fmt.Errorf("insert claude subscription per-user adjustment history: %w", err)
	}
	affectedSubscriptions, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("count claude subscription affected users: %w", err)
	}

	historyRows := affectedSubscriptions
	if affectedSubscriptions == 0 {
		groupArgs := []any{
			input.SourceAccountID,
			input.GroupID,
			groupName,
			input.WindowType,
			input.TriggerType,
			anyIntPtr(input.ThresholdPercent),
			anyTimePtr(input.SourceWindowStart),
			anyTimePtr(input.SourceWindowEnd),
			floatPtrValue(input.SourceUtilizationPercent),
			floatPtrValue(input.SourceWindowCostUSD),
			floatPtrValue(input.SourceEstimatedFullUSD),
			input.AppliedEstimateUSD,
			input.AppliedEstimateSource,
			input.CapPercent,
			input.ShareCount,
			input.ManualOffsetUSD,
			floatPtrValue(oldLimit),
			input.NewLimitUSD,
			delta,
			deltaPercent,
			changed,
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO claude_subscription_limit_adjustments (
source_account_id, group_id, group_name_snapshot,
user_id, user_email_snapshot, subscription_id,
window_type, trigger_type, threshold_percent,
source_window_start, source_window_end,
source_utilization_percent, source_window_cost_usd, source_estimated_full_usd,
applied_estimate_usd, applied_estimate_source,
cap_percent, share_count, manual_offset_usd,
old_limit_usd, new_limit_usd, delta_usd, delta_percent, changed
) VALUES (
$1, $2, $3,
NULL, '', NULL,
$4, $5, $6,
$7, $8,
$9, $10, $11,
$12, $13,
$14, $15, $16,
$17, $18, $19, $20, $21
)
`, groupArgs...); err != nil {
			return nil, fmt.Errorf("insert claude subscription group adjustment history: %w", err)
		}
		historyRows = 1
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claude subscription group limit update: %w", err)
	}
	return &service.ClaudeSubscriptionGroupLimitApplyResult{
		OldLimitUSD:           oldLimit,
		NewLimitUSD:           input.NewLimitUSD,
		Changed:               changed,
		AffectedSubscriptions: affectedSubscriptions,
		HistoryRows:           historyRows,
	}, nil
}

func (r *claudeSubscriptionLimitRepository) ListQuotaEstimates(
	ctx context.Context,
	filter *service.ClaudeSubscriptionQuotaEstimateHistoryFilter,
) ([]service.ClaudeSubscriptionQuotaEstimateHistory, int64, error) {
	if filter == nil {
		filter = &service.ClaudeSubscriptionQuotaEstimateHistoryFilter{}
	}
	page, pageSize := normalizeClaudeHistoryPagination(filter.Page, filter.PageSize)
	where := []string{"1 = 1"}
	args := make([]any, 0, 6)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.SourceAccountID != nil {
		where = append(where, "e.source_account_id = "+addArg(*filter.SourceAccountID))
	}
	if filter.WindowType != "" {
		where = append(where, "e.window_type = "+addArg(filter.WindowType))
	}
	if filter.StartTime != nil {
		where = append(where, "e.sampled_at >= "+addArg(*filter.StartTime))
	}
	if filter.EndTime != nil {
		where = append(where, "e.sampled_at < "+addArg(*filter.EndTime))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM claude_subscription_quota_estimates e
WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count claude subscription quota estimates: %w", err)
	}

	limitArg := addArg(pageSize)
	offsetArg := addArg((page - 1) * pageSize)
	rows, err := r.db.QueryContext(ctx, `
SELECT
e.id, e.source_account_id, a.name,
COALESCE(NULLIF(a.extra ->> 'email_address', ''), NULLIF(a.credentials ->> 'email_address', ''), ''),
e.window_type, e.window_start, e.window_end,
e.threshold_percent, e.utilization_percent, e.window_cost_usd,
e.estimated_full_usd, e.sampled_at, e.created_at
FROM claude_subscription_quota_estimates e
JOIN accounts a ON a.id = e.source_account_id
WHERE `+whereSQL+`
ORDER BY e.sampled_at DESC, e.id DESC
LIMIT `+limitArg+` OFFSET `+offsetArg, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list claude subscription quota estimates: %w", err)
	}
	defer rows.Close()

	out := make([]service.ClaudeSubscriptionQuotaEstimateHistory, 0)
	for rows.Next() {
		var item service.ClaudeSubscriptionQuotaEstimateHistory
		if err := rows.Scan(
			&item.ID, &item.SourceAccountID, &item.SourceAccountName, &item.SourceAccountEmail,
			&item.WindowType, &item.WindowStart, &item.WindowEnd,
			&item.ThresholdPercent, &item.UtilizationPercent, &item.WindowCostUSD,
			&item.EstimatedFullUSD, &item.SampledAt, &item.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan claude subscription quota estimate: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate claude subscription quota estimates: %w", err)
	}
	return out, total, nil
}

func (r *claudeSubscriptionLimitRepository) ListLimitAdjustments(
	ctx context.Context,
	filter *service.ClaudeSubscriptionLimitAdjustmentHistoryFilter,
) ([]service.ClaudeSubscriptionLimitAdjustmentHistory, int64, error) {
	if filter == nil {
		filter = &service.ClaudeSubscriptionLimitAdjustmentHistoryFilter{}
	}
	page, pageSize := normalizeClaudeHistoryPagination(filter.Page, filter.PageSize)
	where := []string{"1 = 1"}
	args := make([]any, 0, 8)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.GroupID != nil {
		where = append(where, "a.group_id = "+addArg(*filter.GroupID))
	}
	if filter.WindowType != "" {
		where = append(where, "a.window_type = "+addArg(filter.WindowType))
	}
	if filter.TriggerType != "" {
		where = append(where, "a.trigger_type = "+addArg(filter.TriggerType))
	}
	if filter.OnlyChanged {
		where = append(where, "a.changed = TRUE")
	}
	if strings.TrimSpace(filter.UserKeyword) != "" {
		where = append(where, "a.user_email_snapshot ILIKE "+addArg("%"+strings.TrimSpace(filter.UserKeyword)+"%"))
	}
	if filter.StartTime != nil {
		where = append(where, "a.created_at >= "+addArg(*filter.StartTime))
	}
	if filter.EndTime != nil {
		where = append(where, "a.created_at < "+addArg(*filter.EndTime))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM claude_subscription_limit_adjustments a
WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count claude subscription limit adjustments: %w", err)
	}

	limitArg := addArg(pageSize)
	offsetArg := addArg((page - 1) * pageSize)
	rows, err := r.db.QueryContext(ctx, `
SELECT
a.id, a.source_account_id,
COALESCE(sa.name, ''),
a.group_id, a.group_name_snapshot,
a.user_id, a.user_email_snapshot, a.subscription_id,
a.window_type, a.trigger_type, a.threshold_percent,
a.source_window_start, a.source_window_end,
a.source_utilization_percent, a.source_window_cost_usd, a.source_estimated_full_usd,
a.applied_estimate_usd, a.applied_estimate_source,
a.cap_percent, a.share_count, a.manual_offset_usd,
a.old_limit_usd, a.new_limit_usd, a.delta_usd, a.delta_percent,
a.changed, a.created_at
FROM claude_subscription_limit_adjustments a
LEFT JOIN accounts sa ON sa.id = a.source_account_id
WHERE `+whereSQL+`
ORDER BY a.created_at DESC, a.id DESC
LIMIT `+limitArg+` OFFSET `+offsetArg, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list claude subscription limit adjustments: %w", err)
	}
	defer rows.Close()

	out := make([]service.ClaudeSubscriptionLimitAdjustmentHistory, 0)
	for rows.Next() {
		var (
			item        service.ClaudeSubscriptionLimitAdjustmentHistory
			sourceID    sql.NullInt64
			groupID     sql.NullInt64
			userID      sql.NullInt64
			subID       sql.NullInt64
			threshold   sql.NullInt64
			windowStart sql.NullTime
			windowEnd   sql.NullTime
			util        sql.NullFloat64
			cost        sql.NullFloat64
			sourceEst   sql.NullFloat64
			oldLimit    sql.NullFloat64
			delta       sql.NullFloat64
			deltaPct    sql.NullFloat64
		)
		if err := rows.Scan(
			&item.ID, &sourceID, &item.SourceAccountName,
			&groupID, &item.GroupNameSnapshot,
			&userID, &item.UserEmail, &subID,
			&item.WindowType, &item.TriggerType, &threshold,
			&windowStart, &windowEnd,
			&util, &cost, &sourceEst,
			&item.AppliedEstimateUSD, &item.AppliedEstimateSource,
			&item.CapPercent, &item.ShareCount, &item.ManualOffsetUSD,
			&oldLimit, &item.NewLimitUSD, &delta, &deltaPct,
			&item.Changed, &item.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan claude subscription limit adjustment: %w", err)
		}
		if sourceID.Valid {
			v := sourceID.Int64
			item.SourceAccountID = &v
		}
		if groupID.Valid {
			v := groupID.Int64
			item.GroupID = &v
		}
		if userID.Valid {
			v := userID.Int64
			item.UserID = &v
		}
		if subID.Valid {
			v := subID.Int64
			item.SubscriptionID = &v
		}
		if threshold.Valid {
			v := int(threshold.Int64)
			item.ThresholdPercent = &v
		}
		if windowStart.Valid {
			t := windowStart.Time
			item.SourceWindowStart = &t
		}
		if windowEnd.Valid {
			t := windowEnd.Time
			item.SourceWindowEnd = &t
		}
		if util.Valid {
			v := util.Float64
			item.SourceUtilizationPercent = &v
		}
		if cost.Valid {
			v := cost.Float64
			item.SourceWindowCostUSD = &v
		}
		if sourceEst.Valid {
			v := sourceEst.Float64
			item.SourceEstimatedFullUSD = &v
		}
		if oldLimit.Valid {
			v := oldLimit.Float64
			item.OldLimitUSD = &v
		}
		if delta.Valid {
			v := delta.Float64
			item.DeltaUSD = &v
		}
		if deltaPct.Valid {
			v := deltaPct.Float64
			item.DeltaPercent = &v
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate claude subscription limit adjustments: %w", err)
	}
	return out, total, nil
}

func normalizeClaudeHistoryPagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
