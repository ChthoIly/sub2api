//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSubscriptionFiveHourLimitReturns429(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := 1.0
	group := &service.Group{
		ID:               78,
		Name:             "claude-five-hour",
		Status:           service.StatusActive,
		Platform:         service.PlatformAnthropic,
		Hydrated:         true,
		SubscriptionType: service.SubscriptionTypeSubscription,
		FiveHourLimitUSD: &limit,
	}
	user := &service.User{
		ID:          1007,
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		Balance:     10,
		Concurrency: 3,
	}
	apiKey := &service.APIKey{
		ID:     502,
		UserID: user.ID,
		Key:    "claude-five-hour-key",
		Status: service.StatusActive,
		User:   user,
		Group:  group,
	}
	apiKey.GroupID = &group.ID

	apiKeyRepo := &stubApiKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	}

	now := time.Now()
	sub := &service.UserSubscription{
		ID:                  602,
		UserID:              user.ID,
		GroupID:             group.ID,
		Status:              service.SubscriptionStatusActive,
		ExpiresAt:           now.Add(24 * time.Hour),
		FiveHourWindowStart: &now,
		FiveHourUsageUSD:    1.01,
	}
	subscriptionRepo := &stubUserSubscriptionRepo{
		getByID: func(context.Context, int64) (*service.UserSubscription, error) {
			clone := *sub
			return &clone, nil
		},
		getActive: func(context.Context, int64, int64) (*service.UserSubscription, error) {
			clone := *sub
			return &clone, nil
		},
	}

	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.SubscriptionMaintenance.WorkerCount = 1
	cfg.SubscriptionMaintenance.QueueSize = 1
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, cfg)
	subscriptionService := service.NewSubscriptionService(nil, subscriptionRepo, nil, nil, cfg)
	router := newAuthTestRouter(apiKeyService, subscriptionService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-api-key", apiKey.Key)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Contains(t, w.Body.String(), "USAGE_LIMIT_EXCEEDED")
	require.Contains(t, w.Body.String(), "five-hour usage limit exceeded")
}
