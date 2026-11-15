package securityaudit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildEventWhereUsesEffectiveReviewDecisionAndRisk(t *testing.T) {
	where, args := buildEventWhere(EventFilter{
		Decision:     "pass",
		RiskLevel:    "low",
		ReviewStatus: "confirmed_false_positive",
	}, 1)

	require.Contains(t, where, "CASE WHEN e.review_status='confirmed_false_positive' THEN 'pass' ELSE e.decision END")
	require.Contains(t, where, "CASE WHEN e.review_status='confirmed_false_positive' THEN 'low' ELSE e.risk_level END")
	require.Contains(t, where, "e.review_status=$3")
	require.Equal(t, []any{"pass", "low", "confirmed_false_positive"}, args)
}
