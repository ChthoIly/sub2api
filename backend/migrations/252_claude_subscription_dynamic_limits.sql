-- Dynamic Claude subscription limits driven by passive Anthropic usage-window samples.
-- A config row exists for every subscription-type group. The dynamic engine is opt-in
-- per group so merely deploying the migration cannot rewrite unrelated group limits.
CREATE TABLE IF NOT EXISTS claude_subscription_limit_settings (
    id SMALLINT PRIMARY KEY,
    source_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_claude_subscription_limit_settings_singleton CHECK (id = 1)
);

INSERT INTO claude_subscription_limit_settings (id, source_account_id)
VALUES (
    1,
    (
        SELECT id
        FROM accounts
        WHERE deleted_at IS NULL
          AND platform = 'anthropic'
          AND type IN ('oauth', 'setup-token')
          AND lower(COALESCE(extra ->> 'email_address', credentials ->> 'email_address', '')) = 'tonoakari@2d-gate.cc'
        ORDER BY id
        LIMIT 1
    )
)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS claude_subscription_limit_configs (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL UNIQUE REFERENCES groups(id) ON DELETE CASCADE,
    auto_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    five_hour_cap_percent NUMERIC(8,4) NOT NULL DEFAULT 90,
    seven_day_cap_percent NUMERIC(8,4) NOT NULL DEFAULT 90,
    five_hour_share_count INTEGER NOT NULL DEFAULT 1,
    seven_day_share_count INTEGER NOT NULL DEFAULT 1,
    five_hour_offset_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    seven_day_offset_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_claude_subscription_limit_configs_five_hour_cap CHECK (five_hour_cap_percent >= 0 AND five_hour_cap_percent <= 100),
    CONSTRAINT chk_claude_subscription_limit_configs_seven_day_cap CHECK (seven_day_cap_percent >= 0 AND seven_day_cap_percent <= 100),
    CONSTRAINT chk_claude_subscription_limit_configs_five_hour_shares CHECK (five_hour_share_count > 0),
    CONSTRAINT chk_claude_subscription_limit_configs_seven_day_shares CHECK (seven_day_share_count > 0)
);

-- Seed one disabled row for every subscription group that exists at upgrade time.
-- The three groups requested by the operator are preconfigured and enabled by name so
-- existing installations start working without an extra manual step.
INSERT INTO claude_subscription_limit_configs (
    group_id,
    auto_enabled,
    five_hour_cap_percent,
    seven_day_cap_percent,
    five_hour_share_count,
    seven_day_share_count
)
SELECT
    g.id,
    lower(g.name) IN ('claude pro', 'claude pro 2人车', 'claude max 4人车'),
    90,
    90,
    CASE lower(g.name)
        WHEN 'claude pro' THEN 5
        WHEN 'claude pro 2人车' THEN 10
        WHEN 'claude max 4人车' THEN 4
        ELSE 1
    END,
    CASE lower(g.name)
        WHEN 'claude pro' THEN 5
        WHEN 'claude pro 2人车' THEN 10
        WHEN 'claude max 4人车' THEN 4
        ELSE 1
    END
FROM groups g
WHERE g.deleted_at IS NULL
  AND g.subscription_type = 'subscription'
ON CONFLICT (group_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS claude_subscription_quota_cycles (
    id BIGSERIAL PRIMARY KEY,
    source_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_type VARCHAR(8) NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    last_threshold_percent INTEGER NOT NULL DEFAULT 0,
    last_utilization_percent NUMERIC(10,4),
    last_window_cost_usd NUMERIC(20,8),
    latest_estimated_full_usd NUMERIC(20,8),
    last_sampled_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_claude_subscription_quota_cycles_window_type CHECK (window_type IN ('5h', '7d')),
    CONSTRAINT chk_claude_subscription_quota_cycles_status CHECK (status IN ('active', 'completed')),
    CONSTRAINT uq_claude_subscription_quota_cycles_window UNIQUE (source_account_id, window_type, window_start)
);

CREATE INDEX IF NOT EXISTS idx_claude_subscription_quota_cycles_prev
    ON claude_subscription_quota_cycles (source_account_id, window_type, window_end DESC)
    WHERE status = 'completed';

CREATE TABLE IF NOT EXISTS claude_subscription_quota_estimates (
    id BIGSERIAL PRIMARY KEY,
    source_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_type VARCHAR(8) NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    threshold_percent INTEGER NOT NULL,
    utilization_percent NUMERIC(10,4) NOT NULL,
    window_cost_usd NUMERIC(20,8) NOT NULL,
    estimated_full_usd NUMERIC(20,8) NOT NULL,
    sampled_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_claude_subscription_quota_estimates_window_type CHECK (window_type IN ('5h', '7d')),
    CONSTRAINT chk_claude_subscription_quota_estimates_threshold CHECK (threshold_percent BETWEEN 20 AND 80),
    CONSTRAINT uq_claude_subscription_quota_estimates_threshold UNIQUE (source_account_id, window_type, window_start, threshold_percent)
);

CREATE INDEX IF NOT EXISTS idx_claude_subscription_quota_estimates_lookup
    ON claude_subscription_quota_estimates (source_account_id, window_type, created_at DESC);

CREATE TABLE IF NOT EXISTS claude_subscription_limit_adjustments (
    id BIGSERIAL PRIMARY KEY,
    source_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    group_name_snapshot VARCHAR(100) NOT NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    user_email_snapshot VARCHAR(255) NOT NULL DEFAULT '',
    subscription_id BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    window_type VARCHAR(8) NOT NULL,
    trigger_type VARCHAR(32) NOT NULL,
    threshold_percent INTEGER,
    source_window_start TIMESTAMPTZ,
    source_window_end TIMESTAMPTZ,
    source_utilization_percent NUMERIC(10,4),
    source_window_cost_usd NUMERIC(20,8),
    source_estimated_full_usd NUMERIC(20,8),
    applied_estimate_usd NUMERIC(20,8) NOT NULL,
    applied_estimate_source VARCHAR(16) NOT NULL,
    cap_percent NUMERIC(8,4) NOT NULL,
    share_count INTEGER NOT NULL,
    manual_offset_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    old_limit_usd NUMERIC(20,8),
    new_limit_usd NUMERIC(20,8) NOT NULL,
    delta_usd NUMERIC(20,8),
    delta_percent NUMERIC(14,4),
    changed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_claude_subscription_limit_adjustments_window_type CHECK (window_type IN ('5h', '7d')),
    CONSTRAINT chk_claude_subscription_limit_adjustments_trigger_type CHECK (trigger_type IN ('threshold', 'config_update', 'source_update')),
    CONSTRAINT chk_claude_subscription_limit_adjustments_threshold CHECK (threshold_percent IS NULL OR threshold_percent BETWEEN 20 AND 80)
);

CREATE INDEX IF NOT EXISTS idx_claude_subscription_limit_adjustments_history
    ON claude_subscription_limit_adjustments (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_claude_subscription_limit_adjustments_group
    ON claude_subscription_limit_adjustments (group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_claude_subscription_limit_adjustments_user
    ON claude_subscription_limit_adjustments (user_id, created_at DESC);
