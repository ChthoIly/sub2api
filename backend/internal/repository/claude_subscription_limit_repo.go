package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type claudeSubscriptionLimitRepository struct {
	db *sql.DB
}

// NewClaudeSubscriptionLimitRepository creates the repository backing the
// standalone Claude dynamic subscription-limit page.
func NewClaudeSubscriptionLimitRepository(db *sql.DB) service.ClaudeSubscriptionLimitRepository {
	return &claudeSubscriptionLimitRepository{db: db}
}

func (r *claudeSubscriptionLimitRepository) ListSourceAccountOptions(ctx context.Context) ([]service.ClaudeSubscriptionSourceAccountOption, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
id,
name,
COALESCE(NULLIF(extra ->> 'email_address', ''), NULLIF(credentials ->> 'email_address', ''), '') AS email,
type,
status
FROM accounts
WHERE deleted_at IS NULL
  AND platform = 'anthropic'
  AND type IN ('oauth', 'setup-token')
ORDER BY CASE WHEN status = 'active' THEN 0 ELSE 1 END, name, id
`)
	if err != nil {
		return nil, fmt.Errorf("list claude subscription source accounts: %w", err)
	}
	defer rows.Close()

	out := make([]service.ClaudeSubscriptionSourceAccountOption, 0)
	for rows.Next() {
		var item service.ClaudeSubscriptionSourceAccountOption
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Type, &item.Status); err != nil {
			return nil, fmt.Errorf("scan claude subscription source account: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claude subscription source accounts: %w", err)
	}
	return out, nil
}

func (r *claudeSubscriptionLimitRepository) EnsureGroupConfigs(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO claude_subscription_limit_configs (group_id)
SELECT g.id
FROM groups g
WHERE g.deleted_at IS NULL
  AND g.subscription_type = 'subscription'
ON CONFLICT (group_id) DO NOTHING
`)
	if err != nil {
		return fmt.Errorf("ensure claude subscription limit configs: %w", err)
	}
	return nil
}

func scanClaudeSubscriptionGroupConfig(scan func(dest ...any) error) (*service.ClaudeSubscriptionGroupConfig, error) {
	var (
		cfg       service.ClaudeSubscriptionGroupConfig
		current5h sql.NullFloat64
		current7d sql.NullFloat64
	)
	if err := scan(
		&cfg.GroupID,
		&cfg.GroupName,
		&cfg.GroupPlatform,
		&cfg.GroupStatus,
		&cfg.SubscriptionCount,
		&current5h,
		&current7d,
		&cfg.AutoEnabled,
		&cfg.InitialFiveHourFullUSD,
		&cfg.InitialSevenDayFullUSD,
		&cfg.FiveHourCapPercent,
		&cfg.SevenDayCapPercent,
		&cfg.FiveHourShareCount,
		&cfg.SevenDayShareCount,
		&cfg.FiveHourOffsetUSD,
		&cfg.SevenDayOffsetUSD,
	); err != nil {
		return nil, err
	}
	if current5h.Valid {
		v := current5h.Float64
		cfg.CurrentFiveHourLimitUSD = &v
	}
	if current7d.Valid {
		v := current7d.Float64
		cfg.CurrentSevenDayLimitUSD = &v
	}
	return &cfg, nil
}

const claudeSubscriptionGroupConfigSelect = `
SELECT
g.id,
g.name,
g.platform,
g.status,
COALESCE(sc.subscription_count, 0) AS subscription_count,
g.five_hour_limit_usd,
g.weekly_limit_usd,
c.auto_enabled,
c.initial_five_hour_full_usd,
c.initial_seven_day_full_usd,
c.five_hour_cap_percent,
c.seven_day_cap_percent,
c.five_hour_share_count,
c.seven_day_share_count,
c.five_hour_offset_usd,
c.seven_day_offset_usd
FROM groups g
JOIN claude_subscription_limit_configs c ON c.group_id = g.id
LEFT JOIN LATERAL (
SELECT COUNT(*) AS subscription_count
FROM user_subscriptions us
WHERE us.group_id = g.id
  AND us.deleted_at IS NULL
  AND us.status = 'active'
  AND us.expires_at > NOW()
) sc ON TRUE
WHERE g.deleted_at IS NULL
  AND g.subscription_type = 'subscription'
`

func (r *claudeSubscriptionLimitRepository) ListGroupConfigs(ctx context.Context) ([]service.ClaudeSubscriptionGroupConfig, error) {
	rows, err := r.db.QueryContext(ctx, claudeSubscriptionGroupConfigSelect+` ORDER BY g.id`)
	if err != nil {
		return nil, fmt.Errorf("list claude subscription group configs: %w", err)
	}
	defer rows.Close()

	out := make([]service.ClaudeSubscriptionGroupConfig, 0)
	for rows.Next() {
		cfg, err := scanClaudeSubscriptionGroupConfig(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan claude subscription group config: %w", err)
		}
		out = append(out, *cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claude subscription group configs: %w", err)
	}
	return out, nil
}

func (r *claudeSubscriptionLimitRepository) GetGroupConfig(ctx context.Context, groupID int64) (*service.ClaudeSubscriptionGroupConfig, error) {
	row := r.db.QueryRowContext(ctx, claudeSubscriptionGroupConfigSelect+` AND g.id = $1`, groupID)
	cfg, err := scanClaudeSubscriptionGroupConfig(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.NotFound("CLAUDE_SUBSCRIPTION_LIMIT_GROUP_NOT_FOUND", "订阅分组配置不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("get claude subscription group config: %w", err)
	}
	return cfg, nil
}

func (r *claudeSubscriptionLimitRepository) UpsertGroupConfig(ctx context.Context, cfg *service.ClaudeSubscriptionGroupConfig) error {
	if cfg == nil {
		return errors.New("group config is nil")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO claude_subscription_limit_configs (
group_id,
auto_enabled,
initial_five_hour_full_usd,
initial_seven_day_full_usd,
five_hour_cap_percent,
seven_day_cap_percent,
five_hour_share_count,
seven_day_share_count,
five_hour_offset_usd,
seven_day_offset_usd
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (group_id) DO UPDATE SET
auto_enabled = EXCLUDED.auto_enabled,
initial_five_hour_full_usd = EXCLUDED.initial_five_hour_full_usd,
initial_seven_day_full_usd = EXCLUDED.initial_seven_day_full_usd,
five_hour_cap_percent = EXCLUDED.five_hour_cap_percent,
seven_day_cap_percent = EXCLUDED.seven_day_cap_percent,
five_hour_share_count = EXCLUDED.five_hour_share_count,
seven_day_share_count = EXCLUDED.seven_day_share_count,
five_hour_offset_usd = EXCLUDED.five_hour_offset_usd,
seven_day_offset_usd = EXCLUDED.seven_day_offset_usd,
updated_at = NOW()
`, cfg.GroupID, cfg.AutoEnabled, cfg.InitialFiveHourFullUSD, cfg.InitialSevenDayFullUSD,
		cfg.FiveHourCapPercent, cfg.SevenDayCapPercent, cfg.FiveHourShareCount, cfg.SevenDayShareCount,
		cfg.FiveHourOffsetUSD, cfg.SevenDayOffsetUSD)
	if err != nil {
		return fmt.Errorf("upsert claude subscription group config: %w", err)
	}
	return nil
}

func (r *claudeSubscriptionLimitRepository) ListBoundAccountsByGroupIDs(
	ctx context.Context,
	groupIDs []int64,
) (map[int64][]service.ClaudeSubscriptionBoundAccountDetail, error) {
	out := make(map[int64][]service.ClaudeSubscriptionBoundAccountDetail, len(groupIDs))
	if len(groupIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT
ag.group_id,
a.id,
a.name,
COALESCE(NULLIF(a.extra ->> 'email_address', ''), NULLIF(a.credentials ->> 'email_address', ''), '') AS email,
a.platform,
a.type,
a.status,
CASE WHEN jsonb_typeof(a.extra -> 'session_window_utilization') = 'number'
     THEN (a.extra ->> 'session_window_utilization')::double precision END AS util_5h,
CASE WHEN jsonb_typeof(a.extra -> 'passive_usage_7d_utilization') = 'number'
     THEN (a.extra ->> 'passive_usage_7d_utilization')::double precision END AS util_7d,
a.session_window_end,
CASE WHEN jsonb_typeof(a.extra -> 'passive_usage_7d_reset') = 'number'
     THEN (a.extra ->> 'passive_usage_7d_reset')::double precision END AS reset_7d_sec
FROM account_groups ag
JOIN accounts a ON a.id = ag.account_id
WHERE a.deleted_at IS NULL
  AND ag.group_id = ANY($1)
ORDER BY ag.group_id, a.id
`, pq.Array(groupIDs))
	if err != nil {
		return nil, fmt.Errorf("list claude subscription bound accounts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			groupID    int64
			item       service.ClaudeSubscriptionBoundAccountDetail
			util5h     sql.NullFloat64
			util7d     sql.NullFloat64
			reset5h    sql.NullTime
			reset7dSec sql.NullFloat64
		)
		if err := rows.Scan(
			&groupID,
			&item.ID,
			&item.Name,
			&item.Email,
			&item.Platform,
			&item.Type,
			&item.Status,
			&util5h,
			&util7d,
			&reset5h,
			&reset7dSec,
		); err != nil {
			return nil, fmt.Errorf("scan claude subscription bound account: %w", err)
		}
		if util5h.Valid {
			v := util5h.Float64 * 100
			item.FiveHourUtilizationPercent = &v
		}
		if util7d.Valid {
			v := util7d.Float64 * 100
			item.SevenDayUtilizationPercent = &v
		}
		if reset5h.Valid {
			t := reset5h.Time
			item.FiveHourResetsAt = &t
		}
		if reset7dSec.Valid {
			sec := reset7dSec.Float64
			if sec > 1e11 {
				sec /= 1000
			}
			t := time.Unix(int64(sec), 0)
			item.SevenDayResetsAt = &t
		}
		out[groupID] = append(out[groupID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claude subscription bound accounts: %w", err)
	}
	return out, nil
}

func (r *claudeSubscriptionLimitRepository) ListActiveSourceAccountIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT DISTINCT b.account_id
FROM claude_subscription_limit_configs c
JOIN groups g ON g.id = c.group_id
JOIN (
    SELECT ag.group_id, MIN(ag.account_id) AS account_id
    FROM account_groups ag
    JOIN accounts a ON a.id = ag.account_id
    WHERE a.deleted_at IS NULL
    GROUP BY ag.group_id
    HAVING COUNT(*) = 1
) b ON b.group_id = g.id
JOIN accounts sa ON sa.id = b.account_id
WHERE c.auto_enabled = TRUE
  AND g.deleted_at IS NULL
  AND g.subscription_type = 'subscription'
  AND sa.deleted_at IS NULL
  AND sa.platform = 'anthropic'
  AND sa.type IN ('oauth', 'setup-token')
ORDER BY b.account_id
`)
	if err != nil {
		return nil, fmt.Errorf("list active claude subscription source account ids: %w", err)
	}
	defer rows.Close()

	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan active claude subscription source account id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active claude subscription source account ids: %w", err)
	}
	return out, nil
}

func (r *claudeSubscriptionLimitRepository) ListEnabledGroupConfigsBySourceAccount(
	ctx context.Context,
	sourceAccountID int64,
) ([]service.ClaudeSubscriptionGroupConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
g.id,
g.name,
g.platform,
g.status,
COALESCE(sc.subscription_count, 0) AS subscription_count,
g.five_hour_limit_usd,
g.weekly_limit_usd,
c.auto_enabled,
c.initial_five_hour_full_usd,
c.initial_seven_day_full_usd,
c.five_hour_cap_percent,
c.seven_day_cap_percent,
c.five_hour_share_count,
c.seven_day_share_count,
c.five_hour_offset_usd,
c.seven_day_offset_usd
FROM groups g
JOIN claude_subscription_limit_configs c ON c.group_id = g.id
JOIN (
    SELECT ag.group_id, MIN(ag.account_id) AS account_id
    FROM account_groups ag
    JOIN accounts a ON a.id = ag.account_id
    WHERE a.deleted_at IS NULL
    GROUP BY ag.group_id
    HAVING COUNT(*) = 1
) b ON b.group_id = g.id
JOIN accounts sa ON sa.id = b.account_id
LEFT JOIN LATERAL (
SELECT COUNT(*) AS subscription_count
FROM user_subscriptions us
WHERE us.group_id = g.id
  AND us.deleted_at IS NULL
  AND us.status = 'active'
  AND us.expires_at > NOW()
) sc ON TRUE
WHERE g.deleted_at IS NULL
  AND g.subscription_type = 'subscription'
  AND c.auto_enabled = TRUE
  AND b.account_id = $1
  AND sa.deleted_at IS NULL
  AND sa.platform = 'anthropic'
  AND sa.type IN ('oauth', 'setup-token')
ORDER BY g.id
`, sourceAccountID)
	if err != nil {
		return nil, fmt.Errorf("list enabled claude subscription group configs by source account: %w", err)
	}
	defer rows.Close()

	out := make([]service.ClaudeSubscriptionGroupConfig, 0)
	for rows.Next() {
		cfg, err := scanClaudeSubscriptionGroupConfig(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan enabled claude subscription group config: %w", err)
		}
		out = append(out, *cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled claude subscription group configs: %w", err)
	}
	return out, nil
}

func scanClaudeSubscriptionQuotaCycle(scan func(dest ...any) error) (*service.ClaudeSubscriptionQuotaCycle, error) {
	var (
		cycle      service.ClaudeSubscriptionQuotaCycle
		lastUtil   sql.NullFloat64
		lastCost   sql.NullFloat64
		latestEst  sql.NullFloat64
		lastSample sql.NullTime
		completed  sql.NullTime
	)
	if err := scan(
		&cycle.ID,
		&cycle.SourceAccountID,
		&cycle.WindowType,
		&cycle.WindowStart,
		&cycle.WindowEnd,
		&cycle.Status,
		&cycle.LastThresholdPercent,
		&lastUtil,
		&lastCost,
		&latestEst,
		&lastSample,
		&completed,
	); err != nil {
		return nil, err
	}
	if lastUtil.Valid {
		v := lastUtil.Float64
		cycle.LastUtilizationPercent = &v
	}
	if lastCost.Valid {
		v := lastCost.Float64
		cycle.LastWindowCostUSD = &v
	}
	if latestEst.Valid {
		v := latestEst.Float64
		cycle.LatestEstimatedFullUSD = &v
	}
	if lastSample.Valid {
		t := lastSample.Time
		cycle.LastSampledAt = &t
	}
	if completed.Valid {
		t := completed.Time
		cycle.CompletedAt = &t
	}
	return &cycle, nil
}

func (r *claudeSubscriptionLimitRepository) EnsureQuotaCycle(
	ctx context.Context,
	sourceAccountID int64,
	windowType string,
	windowStart, windowEnd time.Time,
) (*service.ClaudeSubscriptionQuotaCycle, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claude subscription quota cycle transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	lockKey := fmt.Sprintf("claude-subscription-limit:%d:%s", sourceAccountID, windowType)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, lockKey); err != nil {
		return nil, fmt.Errorf("lock claude subscription quota cycle: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE claude_subscription_quota_cycles
SET status = 'completed',
    completed_at = COALESCE(completed_at, NOW()),
    updated_at = NOW()
WHERE source_account_id = $1
  AND window_type = $2
  AND status = 'active'
  AND window_start <> $3
`, sourceAccountID, windowType, windowStart); err != nil {
		return nil, fmt.Errorf("complete previous claude subscription quota cycles: %w", err)
	}

	row := tx.QueryRowContext(ctx, `
INSERT INTO claude_subscription_quota_cycles (
source_account_id, window_type, window_start, window_end, status
) VALUES ($1, $2, $3, $4, 'active')
ON CONFLICT (source_account_id, window_type, window_start) DO UPDATE
SET window_end = EXCLUDED.window_end,
    status = 'active',
    completed_at = NULL,
    updated_at = NOW()
RETURNING
id, source_account_id, window_type, window_start, window_end, status,
last_threshold_percent, last_utilization_percent, last_window_cost_usd,
latest_estimated_full_usd, last_sampled_at, completed_at
`, sourceAccountID, windowType, windowStart, windowEnd)
	cycle, err := scanClaudeSubscriptionQuotaCycle(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("upsert claude subscription quota cycle: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claude subscription quota cycle: %w", err)
	}
	return cycle, nil
}

func (r *claudeSubscriptionLimitRepository) RecordQuotaEstimate(
	ctx context.Context,
	sample *service.ClaudeSubscriptionQuotaWindowSample,
	thresholdPercent int,
	estimatedFullUSD float64,
) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO claude_subscription_quota_estimates (
source_account_id, window_type, window_start, window_end,
threshold_percent, utilization_percent, window_cost_usd,
estimated_full_usd, sampled_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source_account_id, window_type, window_start, threshold_percent) DO NOTHING
`, sample.SourceAccountID, sample.WindowType, sample.WindowStart, sample.WindowEnd,
		thresholdPercent, sample.UtilizationPercent, sample.WindowCostUSD, estimatedFullUSD, sample.SampledAt)
	if err != nil {
		return false, fmt.Errorf("record claude subscription quota estimate: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("record claude subscription quota estimate affected rows: %w", err)
	}
	return affected > 0, nil
}

func (r *claudeSubscriptionLimitRepository) AdvanceQuotaCycle(
	ctx context.Context,
	sample *service.ClaudeSubscriptionQuotaWindowSample,
	thresholdPercent int,
	estimatedFullUSD float64,
) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE claude_subscription_quota_cycles
SET last_threshold_percent = $5,
    last_utilization_percent = $6,
    last_window_cost_usd = $7,
    latest_estimated_full_usd = $8,
    last_sampled_at = $9,
    window_end = $4,
    updated_at = NOW()
WHERE source_account_id = $1
  AND window_type = $2
  AND window_start = $3
  AND last_threshold_percent < $5
`, sample.SourceAccountID, sample.WindowType, sample.WindowStart, sample.WindowEnd,
		thresholdPercent, sample.UtilizationPercent, sample.WindowCostUSD, estimatedFullUSD, sample.SampledAt)
	if err != nil {
		return fmt.Errorf("advance claude subscription quota cycle: %w", err)
	}
	return nil
}

func (r *claudeSubscriptionLimitRepository) GetLatestEffectiveEstimate(
	ctx context.Context,
	sourceAccountID int64,
	windowType string,
	minThreshold int,
) (*service.ClaudeSubscriptionQuotaEstimateRef, error) {
	var (
		estimate  float64
		start     time.Time
		end       time.Time
		threshold int
		sampledAt time.Time
	)
	err := r.db.QueryRowContext(ctx, `
SELECT estimated_full_usd, window_start, window_end, threshold_percent, sampled_at
FROM claude_subscription_quota_estimates
WHERE source_account_id = $1
  AND window_type = $2
  AND threshold_percent >= $3
  AND estimated_full_usd > 0
ORDER BY sampled_at DESC, id DESC
LIMIT 1
`, sourceAccountID, windowType, minThreshold).Scan(&estimate, &start, &end, &threshold, &sampledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest effective claude subscription estimate: %w", err)
	}
	return &service.ClaudeSubscriptionQuotaEstimateRef{
		EstimatedFullUSD: estimate,
		WindowStart:      start,
		WindowEnd:        end,
		ThresholdPercent: threshold,
		SampledAt:        sampledAt,
	}, nil
}

func (r *claudeSubscriptionLimitRepository) GetCurrentCycle(
	ctx context.Context,
	sourceAccountID int64,
	windowType string,
) (*service.ClaudeSubscriptionQuotaCycle, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
id, source_account_id, window_type, window_start, window_end, status,
last_threshold_percent, last_utilization_percent, last_window_cost_usd,
latest_estimated_full_usd, last_sampled_at, completed_at
FROM claude_subscription_quota_cycles
WHERE source_account_id = $1
  AND window_type = $2
  AND status = 'active'
  AND window_end > NOW()
ORDER BY window_start DESC, id DESC
LIMIT 1
`, sourceAccountID, windowType)
	cycle, err := scanClaudeSubscriptionQuotaCycle(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get current claude subscription quota cycle: %w", err)
	}
	return cycle, nil
}
