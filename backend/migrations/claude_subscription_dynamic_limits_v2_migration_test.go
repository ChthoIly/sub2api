package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeSubscriptionDynamicLimitsV2Migration(t *testing.T) {
	content, err := FS.ReadFile("253_claude_subscription_dynamic_limits_v2.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS initial_five_hour_full_usd NUMERIC(20,8) NOT NULL DEFAULT 100")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS initial_seven_day_full_usd NUMERIC(20,8) NOT NULL DEFAULT 1200")
	require.Contains(t, sql, "CHECK (threshold_percent BETWEEN 1 AND 100)")
	require.Contains(t, sql, "idx_claude_subscription_quota_estimates_effective")
}
