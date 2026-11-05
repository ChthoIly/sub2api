-- Subscription-mode 5-hour rolling quota.
-- groups.five_hour_limit_usd: nullable default limit for all subscriptions in the group.
-- user_subscriptions.five_hour_usage_usd / five_hour_window_start: per-user subscription usage.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS five_hour_limit_usd DECIMAL(20,8);

ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS five_hour_window_start TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS five_hour_usage_usd DECIMAL(20,10) NOT NULL DEFAULT 0;
