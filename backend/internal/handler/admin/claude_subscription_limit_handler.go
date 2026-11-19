package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ClaudeSubscriptionLimitHandler exposes the standalone dynamic Claude
// subscription-limit configuration and history page APIs.
type ClaudeSubscriptionLimitHandler struct {
	service *service.ClaudeSubscriptionLimitService
}

func NewClaudeSubscriptionLimitHandler(svc *service.ClaudeSubscriptionLimitService) *ClaudeSubscriptionLimitHandler {
	return &ClaudeSubscriptionLimitHandler{service: svc}
}

type updateClaudeSubscriptionGroupConfigRequest struct {
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

// GetOverview GET /api/v1/admin/claude-subscription-limits
func (h *ClaudeSubscriptionLimitHandler) GetOverview(c *gin.Context) {
	overview, err := h.service.GetOverview(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

// UpdateGroupConfig PUT /api/v1/admin/claude-subscription-limits/groups/:id
func (h *ClaudeSubscriptionLimitHandler) UpdateGroupConfig(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "invalid group id")
		return
	}
	var req updateClaudeSubscriptionGroupConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	err = h.service.UpdateGroupConfig(c.Request.Context(), groupID, &service.ClaudeSubscriptionGroupConfigUpdate{
		AutoEnabled:            req.AutoEnabled,
		InitialFiveHourFullUSD: req.InitialFiveHourFullUSD,
		InitialSevenDayFullUSD: req.InitialSevenDayFullUSD,
		FiveHourCapPercent:     req.FiveHourCapPercent,
		SevenDayCapPercent:     req.SevenDayCapPercent,
		FiveHourShareCount:     req.FiveHourShareCount,
		SevenDayShareCount:     req.SevenDayShareCount,
		FiveHourOffsetUSD:      req.FiveHourOffsetUSD,
		SevenDayOffsetUSD:      req.SevenDayOffsetUSD,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"group_id": groupID})
}

// ListQuotaEstimates GET /api/v1/admin/claude-subscription-limits/history/estimates
func (h *ClaudeSubscriptionLimitHandler) ListQuotaEstimates(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := &service.ClaudeSubscriptionQuotaEstimateHistoryFilter{
		WindowType: strings.TrimSpace(c.Query("window_type")),
		Page:       page,
		PageSize:   pageSize,
	}
	if raw := strings.TrimSpace(c.Query("source_account_id")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			filter.SourceAccountID = &id
		}
	}
	filter.StartTime = parseClaudeHistoryTime(c.Query("start_time"), false)
	filter.EndTime = parseClaudeHistoryTime(c.Query("end_time"), true)

	items, total, err := h.service.ListQuotaEstimates(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// ListLimitAdjustments GET /api/v1/admin/claude-subscription-limits/history/adjustments
func (h *ClaudeSubscriptionLimitHandler) ListLimitAdjustments(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := &service.ClaudeSubscriptionLimitAdjustmentHistoryFilter{
		WindowType:  strings.TrimSpace(c.Query("window_type")),
		TriggerType: strings.TrimSpace(c.Query("trigger_type")),
		UserKeyword: strings.TrimSpace(c.Query("user_keyword")),
		OnlyChanged: strings.EqualFold(strings.TrimSpace(c.Query("only_changed")), "true"),
		Page:        page,
		PageSize:    pageSize,
	}
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			filter.GroupID = &id
		}
	}
	filter.StartTime = parseClaudeHistoryTime(c.Query("start_time"), false)
	filter.EndTime = parseClaudeHistoryTime(c.Query("end_time"), true)

	items, total, err := h.service.ListLimitAdjustments(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func parseClaudeHistoryTime(raw string, endOfDay bool) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	if parsed, err := time.Parse("2006-01-02", raw); err == nil {
		if endOfDay {
			parsed = parsed.Add(24 * time.Hour)
		}
		return &parsed
	}
	return nil
}
