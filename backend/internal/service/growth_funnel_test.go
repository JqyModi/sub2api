//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecordGrowthEventStoresAnonymousCampaignMetadata(t *testing.T) {
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}

	require.NoError(t, svc.RecordGrowthEvent(context.Background(), GrowthEventInput{
		EventType:   GrowthEventAnnouncementViewed,
		CampaignID:  "subscription-launch-v019",
		Platform:    "win32",
		AppVersion:  "0.1.9",
		SessionHash: strings.Repeat("a", 64),
	}))

	event, err := client.GrowthEvent.Query().Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, GrowthEventAnnouncementViewed, event.EventType)
	require.Equal(t, "subscription-launch-v019", event.CampaignID)
	require.Equal(t, "win32", event.Platform)
	require.Equal(t, "0.1.9", event.AppVersion)
	require.Nil(t, event.UserID)
}

func TestGetGrowthFunnelStatsCombinesAnonymousEventsWithBusinessStages(t *testing.T) {
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	ctx := context.Background()

	for _, eventType := range []string{
		GrowthEventAnnouncementViewed,
		GrowthEventAnnouncementViewed,
		GrowthEventAnnouncementClicked,
		GrowthEventDesktopAuthStarted,
	} {
		require.NoError(t, svc.RecordGrowthEvent(ctx, GrowthEventInput{EventType: eventType}))
	}

	stages, err := svc.getGrowthFunnelStats(ctx, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	counts := make(map[string]int, len(stages))
	for _, stage := range stages {
		counts[stage.Key] = stage.Count
	}
	require.Equal(t, 2, counts["announcement_viewed"])
	require.Equal(t, 1, counts["announcement_clicked"])
	require.Equal(t, 1, counts["desktop_auth_started"])
	require.Equal(t, 0, counts["payment_completed"])
	require.Equal(t, 0, counts["first_api_request_succeeded"])
}

func TestGetGrowthFunnelStatsIncludesTrialAndReferralRewardStages(t *testing.T) {
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	ctx := context.Background()

	trialGroup, err := client.Group.Create().
		SetName("starter-beta").
		SetPlatform(PlatformOpenAI).
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	trialUser, err := client.User.Create().
		SetEmail("trial-funnel@example.com").
		SetUsername("trial-funnel").
		SetPasswordHash("hash").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UserSubscription.Create().
		SetUserID(trialUser.ID).
		SetGroupID(trialGroup.ID).
		SetStartsAt(time.Now()).
		SetExpiresAt(time.Now().AddDate(0, 0, 3)).
		SetAssignedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.Create().
		SetOrderID("test-referral-order").
		SetAction(affiliateSubscriptionRewardAction).
		SetDetail(`{"subID":1}`).
		SetOperator("system").
		Save(ctx)
	require.NoError(t, err)

	stages, err := svc.getGrowthFunnelStats(ctx, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	counts := make(map[string]int, len(stages))
	for _, stage := range stages {
		counts[stage.Key] = stage.Count
	}
	require.Equal(t, 1, counts["trial_activated"])
	require.Equal(t, 1, counts["referral_reward_granted"])
}

func TestIsPublicGrowthEventTypeRejectsServerOwnedStages(t *testing.T) {
	require.True(t, IsPublicGrowthEventType(GrowthEventAnnouncementClicked))
	require.False(t, IsPublicGrowthEventType(GrowthEventTokenRedeemed))
	require.False(t, IsPublicGrowthEventType("payment_completed"))
}
