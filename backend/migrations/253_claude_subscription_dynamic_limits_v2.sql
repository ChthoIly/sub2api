-- Refine Claude dynamic subscription limits:
-- 1. Allow per-group admin-configured initial full-quota defaults (5h / 7d) until
--    self-driving estimates (>= 50% threshold) take over.
-- 2. Allow thresholds up to 100% (specifically 50 / 75 / 85) while keeping legacy
--    rows (20..80) valid.
-- 3. Add a partial index for fast lookup of the latest effective (>= 50%) estimate.

ALTER TABLE claude_subscription_limit_configs
    ADD COLUMN IF NOT EXISTS initial_five_hour_full_usd NUMERIC(20,8) NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS initial_seven_day_full_usd NUMERIC(20,8) NOT NULL DEFAULT 1200;

ALTER TABLE claude_subscription_limit_configs
    DROP CONSTRAINT IF EXISTS chk_claude_subscription_limit_configs_initial_5h;
ALTER TABLE claude_subscription_limit_configs
    ADD CONSTRAINT chk_claude_subscription_limit_configs_initial_5h
    CHECK (initial_five_hour_full_usd > 0);

ALTER TABLE claude_subscription_limit_configs
    DROP CONSTRAINT IF EXISTS chk_claude_subscription_limit_configs_initial_7d;
ALTER TABLE claude_subscription_limit_configs
    ADD CONSTRAINT chk_claude_subscription_limit_configs_initial_7d
    CHECK (initial_seven_day_full_usd > 0);

ALTER TABLE claude_subscription_quota_estimates
    DROP CONSTRAINT IF EXISTS chk_claude_subscription_quota_estimates_threshold;
ALTER TABLE claude_subscription_quota_estimates
    ADD CONSTRAINT chk_claude_subscription_quota_estimates_threshold
    CHECK (threshold_percent BETWEEN 1 AND 100);

ALTER TABLE claude_subscription_limit_adjustments
    DROP CONSTRAINT IF EXISTS chk_claude_subscription_limit_adjustments_threshold;
ALTER TABLE claude_subscription_limit_adjustments
    ADD CONSTRAINT chk_claude_subscription_limit_adjustments_threshold
    CHECK (threshold_percent IS NULL OR threshold_percent BETWEEN 1 AND 100);

CREATE INDEX IF NOT EXISTS idx_claude_subscription_quota_estimates_effective
    ON claude_subscription_quota_estimates (source_account_id, window_type, sampled_at DESC, id DESC)
    WHERE threshold_percent >= 50 AND estimated_full_usd > 0;
