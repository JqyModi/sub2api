//go:build unit

package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTrialUsageIPGuardClaim(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	guard := NewTrialUsageIPGuard(redisClient)
	expiresAt := time.Now().Add(48 * time.Hour)
	ctx := context.Background()

	require.NoError(t, guard.Claim(ctx, 101, "198.200.42.57", expiresAt))
	require.NoError(t, guard.Claim(ctx, 101, "198.200.42.57", expiresAt))
	require.ErrorIs(t, guard.Claim(ctx, 102, "198.200.42.57", expiresAt), errTrialNetworkAlreadyUsed)
	require.ErrorIs(t, guard.Claim(ctx, 101, "203.0.113.9", expiresAt), errTrialAccountNetworkChanged)
}

func TestTrialUsageIPGuardConcurrentClaimHasSingleWinner(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	guard := NewTrialUsageIPGuard(redisClient)
	expiresAt := time.Now().Add(48 * time.Hour)
	results := make(chan error, 2)
	for _, userID := range []int64{201, 202} {
		go func(id int64) {
			results <- guard.Claim(context.Background(), id, "198.200.42.57", expiresAt)
		}(userID)
	}

	var allowed, rejected int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			allowed++
		case errors.Is(err, errTrialNetworkAlreadyUsed):
			rejected++
		default:
			t.Fatalf("unexpected claim result: %v", err)
		}
	}
	require.Equal(t, 1, allowed)
	require.Equal(t, 1, rejected)
}

func TestTrialUsageIPGuardFailsClosedWhenRedisUnavailable(t *testing.T) {
	guard := NewTrialUsageIPGuard(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}))
	err := guard.Claim(context.Background(), 301, "198.200.42.57", time.Now().Add(time.Hour))
	require.Error(t, err)
	require.NotErrorIs(t, err, errTrialNetworkAlreadyUsed)
}

func TestTrialUsageIPGuardOnlyRestrictsCurrentSignupTrial(t *testing.T) {
	guard := &TrialUsageIPGuard{}
	require.True(t, guard.IsRestrictedGroup(&service.Group{Name: "starter-beta-v2"}))
	require.True(t, guard.IsRestrictedGroup(&service.Group{Name: " STARTER-BETA-V2 "}))
	require.True(t, guard.IsRestrictedGroup(&service.Group{Name: "starter-beta-v3"}))
	require.False(t, guard.IsRestrictedGroup(&service.Group{Name: "starter-beta"}))
	require.False(t, guard.IsRestrictedGroup(&service.Group{Name: "pro"}))
	require.False(t, guard.IsRestrictedGroup(nil))
}
