package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/growthevent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
)

const (
	GrowthEventAnnouncementViewed  = "announcement_viewed"
	GrowthEventAnnouncementClicked = "announcement_clicked"
	GrowthEventDesktopAuthStarted  = "desktop_auth_started"
	GrowthEventTokenRedeemed       = "token_redeemed"
	growthEventRetention           = 90 * 24 * time.Hour
)

var publicGrowthEventTypes = map[string]bool{
	GrowthEventAnnouncementViewed:  true,
	GrowthEventAnnouncementClicked: true,
}

type GrowthEventInput struct {
	EventType       string
	CampaignID      string
	Platform        string
	AppVersion      string
	SessionHash     string
	UserID          *int64
	PlanID          *int64
	PaymentProvider string
	Result          string
	ErrorCode       string
}

func IsPublicGrowthEventType(eventType string) bool {
	return publicGrowthEventTypes[strings.TrimSpace(eventType)]
}

func (s *PaymentService) RecordGrowthEvent(ctx context.Context, input GrowthEventInput) error {
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return nil
	}
	result := strings.TrimSpace(input.Result)
	if result == "" {
		result = "success"
	}
	builder := s.entClient.GrowthEvent.Create().
		SetEventType(eventType).
		SetCampaignID(trimGrowthValue(input.CampaignID, 100)).
		SetPlatform(trimGrowthValue(input.Platform, 32)).
		SetAppVersion(trimGrowthValue(input.AppVersion, 32)).
		SetSessionHash(trimGrowthValue(input.SessionHash, 64)).
		SetPaymentProvider(trimGrowthValue(input.PaymentProvider, 32)).
		SetResult(trimGrowthValue(result, 32)).
		SetErrorCode(trimGrowthValue(input.ErrorCode, 100))
	if input.UserID != nil {
		builder.SetUserID(*input.UserID)
	}
	if input.PlanID != nil {
		builder.SetPlanID(*input.PlanID)
	}
	_, err := builder.Save(ctx)
	return err
}

func trimGrowthValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func (s *PaymentService) getGrowthFunnelStats(ctx context.Context, since time.Time) ([]GrowthFunnelStage, error) {
	if _, err := s.entClient.GrowthEvent.Delete().Where(growthevent.CreatedAtLT(time.Now().Add(-growthEventRetention))).Exec(ctx); err != nil {
		return nil, err
	}
	eventStages := []struct {
		key   string
		event string
	}{
		{"announcement_viewed", GrowthEventAnnouncementViewed},
		{"announcement_clicked", GrowthEventAnnouncementClicked},
		{"desktop_auth_started", GrowthEventDesktopAuthStarted},
	}
	stages := make([]GrowthFunnelStage, 0, 9)
	for _, stage := range eventStages {
		count, err := s.entClient.GrowthEvent.Query().Where(
			growthevent.EventTypeEQ(stage.event),
			growthevent.CreatedAtGTE(since),
		).Count(ctx)
		if err != nil {
			return nil, err
		}
		stages = append(stages, GrowthFunnelStage{Key: stage.key, Count: count})
	}

	registered, err := s.entClient.User.Query().Where(user.CreatedAtGTE(since)).Count(ctx)
	if err != nil {
		return nil, err
	}
	stages = append(stages, GrowthFunnelStage{Key: "registration_completed", Count: registered})

	trialGroupIDs, err := s.entClient.Group.Query().
		Where(group.NameEQ("starter-beta")).
		Select(group.FieldID).
		Ints(ctx)
	if err != nil {
		return nil, err
	}
	trialActivated := 0
	if len(trialGroupIDs) > 0 {
		trialGroupID64s := make([]int64, 0, len(trialGroupIDs))
		for _, id := range trialGroupIDs {
			trialGroupID64s = append(trialGroupID64s, int64(id))
		}
		var trialUsers []struct {
			UserID int64 `json:"user_id"`
		}
		err = s.entClient.UserSubscription.Query().Where(
			usersubscription.GroupIDIn(trialGroupID64s...),
			usersubscription.AssignedAtGTE(since),
		).Unique(true).Select(usersubscription.FieldUserID).Scan(ctx, &trialUsers)
		if err != nil {
			return nil, err
		}
		trialActivated = len(trialUsers)
	}
	stages = append(stages, GrowthFunnelStage{Key: "trial_activated", Count: trialActivated})

	var checkoutUsers []struct {
		UserID int64 `json:"user_id"`
	}
	err = s.entClient.PaymentOrder.Query().Where(
		paymentorder.OrderTypeEQ("subscription"),
		paymentorder.CreatedAtGTE(since),
	).Unique(true).Select(paymentorder.FieldUserID).Scan(ctx, &checkoutUsers)
	if err != nil {
		return nil, err
	}
	stages = append(stages, GrowthFunnelStage{Key: "checkout_started", Count: len(checkoutUsers)})

	var paidUsers []struct {
		UserID int64 `json:"user_id"`
	}
	err = s.entClient.PaymentOrder.Query().Where(
		paymentorder.OrderTypeEQ("subscription"),
		paymentorder.StatusIn(OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefunding, OrderStatusRefundPending, OrderStatusPartiallyRefunded, OrderStatusRefundFailed),
		paymentorder.PaidAtGTE(since),
	).Unique(true).Select(paymentorder.FieldUserID).Scan(ctx, &paidUsers)
	if err != nil {
		return nil, err
	}
	stages = append(stages, GrowthFunnelStage{Key: "payment_completed", Count: len(paidUsers)})

	rewarded, err := s.entClient.PaymentAuditLog.Query().Where(
		paymentauditlog.ActionEQ(affiliateSubscriptionRewardAction),
		paymentauditlog.CreatedAtGTE(since),
	).Count(ctx)
	if err != nil {
		return nil, err
	}
	stages = append(stages, GrowthFunnelStage{Key: "referral_reward_granted", Count: rewarded})

	var activeUsers []struct {
		UserID int64 `json:"user_id"`
	}
	err = s.entClient.UsageLog.Query().Where(
		usagelog.SubscriptionIDNotNil(),
		usagelog.CreatedAtGTE(since),
	).Unique(true).Select(usagelog.FieldUserID).Scan(ctx, &activeUsers)
	if err != nil {
		return nil, err
	}
	stages = append(stages, GrowthFunnelStage{Key: "first_api_request_succeeded", Count: len(activeUsers)})
	return stages, nil
}
