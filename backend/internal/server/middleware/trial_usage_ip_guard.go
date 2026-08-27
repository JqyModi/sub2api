package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

var trialUsageRestrictedGroups = map[string]struct{}{
	"starter-beta-v2": {},
	"starter-beta-v3": {},
}

var (
	errTrialNetworkAlreadyUsed    = errors.New("trial network already used by another account")
	errTrialAccountNetworkChanged = errors.New("trial account attempted to use another network")
)

var claimTrialUsageNetworkScript = redis.NewScript(`
local ip_owner = redis.call("GET", KEYS[1])
local user_ip = redis.call("GET", KEYS[2])

if ip_owner and ip_owner ~= ARGV[1] then
  return 2
end
if user_ip and user_ip ~= ARGV[2] then
  return 3
end

redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[3])
redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
return 1
`)

// TrialUsageIPGuard binds a free-trial account to its first consumption IP and
// prevents the same IP from consuming grants issued to multiple accounts.
type TrialUsageIPGuard struct {
	redis *redis.Client
}

func NewTrialUsageIPGuard(redisClient *redis.Client) *TrialUsageIPGuard {
	return &TrialUsageIPGuard{redis: redisClient}
}

func (g *TrialUsageIPGuard) IsRestrictedGroup(group *service.Group) bool {
	if group == nil {
		return false
	}
	_, restricted := trialUsageRestrictedGroups[strings.ToLower(strings.TrimSpace(group.Name))]
	return restricted
}

func (g *TrialUsageIPGuard) Claim(ctx context.Context, userID int64, clientIP string, expiresAt time.Time) error {
	if g == nil || g.redis == nil {
		return errors.New("trial usage network guard unavailable")
	}
	clientIP = strings.TrimSpace(clientIP)
	if userID <= 0 || clientIP == "" || !expiresAt.After(time.Now()) {
		return errors.New("invalid trial usage network claim")
	}
	parsedIP := net.ParseIP(clientIP)
	if parsedIP == nil {
		return errors.New("invalid trial usage client IP")
	}
	clientIP = parsedIP.String()

	ipHash := hashTrialUsageIP(clientIP)
	ttl := time.Until(expiresAt)
	if ttl < time.Minute {
		ttl = time.Minute
	}

	result, err := claimTrialUsageNetworkScript.Run(
		ctx,
		g.redis,
		[]string{
			"trial_usage_ip:v2:" + ipHash,
			fmt.Sprintf("trial_usage_user:v2:%d", userID),
		},
		userID,
		ipHash,
		ttl.Milliseconds(),
	).Int()
	if err != nil {
		return fmt.Errorf("claim trial usage network: %w", err)
	}

	switch result {
	case 1:
		return nil
	case 2:
		return errTrialNetworkAlreadyUsed
	case 3:
		return errTrialAccountNetworkChanged
	default:
		return fmt.Errorf("claim trial usage network: unexpected result %d", result)
	}
}

func hashTrialUsageIP(clientIP string) string {
	sum := sha256.Sum256([]byte(clientIP))
	return hex.EncodeToString(sum[:])
}
