//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func createCapacityTestUser(t *testing.T, client *dbent.Client, email string) *dbent.User {
	t.Helper()
	user, err := client.User.Create().SetEmail(email).SetPasswordHash("hash").SetUsername(email).Save(context.Background())
	require.NoError(t, err)
	return user
}

func createCapacityTestPlan(t *testing.T, client *dbent.Client, maxSales, perUserLimit int) *dbent.SubscriptionPlan {
	t.Helper()
	plan, err := client.SubscriptionPlan.Create().SetGroupID(1).SetName("capacity-plan").SetDescription("capacity test").SetPrice(1).
		SetValidityDays(1).SetValidityUnit("days").SetMaxSales(maxSales).SetPerUserLimit(perUserLimit).Save(context.Background())
	require.NoError(t, err)
	return plan
}

func createCapacityTestOrder(t *testing.T, client *dbent.Client, user *dbent.User, plan *dbent.SubscriptionPlan, status string) {
	t.Helper()
	_, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(1).SetPayAmount(1).SetRechargeCode("").SetPaymentType(payment.TypeStripeCard).SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).SetPlanID(plan.ID).SetStatus(status).SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").SetSrcHost("test.local").Save(context.Background())
	require.NoError(t, err)
}

func checkCapacity(t *testing.T, client *dbent.Client, user *dbent.User, plan *dbent.SubscriptionPlan) error {
	t.Helper()
	svc := &PaymentService{entClient: client}
	tx, err := client.Tx(context.Background())
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	return svc.checkPlanCapacity(context.Background(), tx, plan, user.ID)
}

func TestCreateOrderInTx_EnforcesTotalPlanCapacity(t *testing.T) {
	client := newPaymentConfigServiceTestClient(t)
	first := createCapacityTestUser(t, client, "capacity-first@example.com")
	second := createCapacityTestUser(t, client, "capacity-second@example.com")
	plan := createCapacityTestPlan(t, client, 1, 0)
	createCapacityTestOrder(t, client, first, plan, OrderStatusCompleted)
	require.ErrorContains(t, checkCapacity(t, client, second, plan), "PLAN_SOLD_OUT")
}

func TestCreateOrderInTx_EnforcesPerUserPurchaseLimit(t *testing.T) {
	client := newPaymentConfigServiceTestClient(t)
	user := createCapacityTestUser(t, client, "capacity-user@example.com")
	plan := createCapacityTestPlan(t, client, 0, 1)
	createCapacityTestOrder(t, client, user, plan, OrderStatusPaid)
	require.ErrorContains(t, checkCapacity(t, client, user, plan), "PLAN_PURCHASE_LIMIT_REACHED")
}

func TestCreateOrderInTx_ReleasesCapacityForTerminalOrders(t *testing.T) {
	for _, status := range []string{OrderStatusFailed, OrderStatusCancelled, OrderStatusExpired, OrderStatusRefunded} {
		t.Run(status, func(t *testing.T) {
			client := newPaymentConfigServiceTestClient(t)
			first := createCapacityTestUser(t, client, "terminal-first-"+status+"@example.com")
			second := createCapacityTestUser(t, client, "terminal-second-"+status+"@example.com")
			plan := createCapacityTestPlan(t, client, 1, 0)
			createCapacityTestOrder(t, client, first, plan, status)
			require.NoError(t, checkCapacity(t, client, second, plan))
		})
	}
}
