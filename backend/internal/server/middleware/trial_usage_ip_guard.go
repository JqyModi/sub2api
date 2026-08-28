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

const trialUsageMaxNetworks = 3

var (
	errTrialNetworkAlreadyUsed  = errors.New("trial network already used by another account")
	errTrialAccountNetworkLimit = errors.New("trial account network change limit reached")
)

var claimTrialUsageNetworkScript = redis.NewScript(`
local ip_owner = redis.call("GET", KEYS[1])
local legacy_user_ip = redis.call("GET", KEYS[2])

if ip_owner and ip_owner ~= ARGV[1] then
  return 2
end

if legacy_user_ip then
  redis.call("SADD", KEYS[3], legacy_user_ip)
end

local already_claimed = redis.call("SISMEMBER", KEYS[3], ARGV[2])
local claimed_count = redis.call("SCARD", KEYS[3])
if already_claimed == 0 and claimed_count >= tonumber(ARGV[4]) then
  return 3
end

redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[3])
redis.call("SADD", KEYS[3], ARGV[2])
redis.call("PEXPIRE", KEYS[3], ARGV[3])

if not legacy_user_ip then
  redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
else
  redis.call("PEXPIRE", KEYS[2], ARGV[3])
end
return 1
`)

// TrialUsageIPGuard limits a free-trial account to a small set of consumption
// networks and prevents one network from consuming grants for multiple users.
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
			fmt.Sprintf("trial_usage_user_ips:v3:%d", userID),
		},
		userID,
		ipHash,
		ttl.Milliseconds(),
		trialUsageMaxNetworks,
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
		return errTrialAccountNetworkLimit
	default:
		return fmt.Errorf("claim trial usage network: unexpected result %d", result)
	}
}

func hashTrialUsageIP(clientIP string) string {
	sum := sha256.Sum256([]byte(clientIP))
	return hex.EncodeToString(sum[:])
}
