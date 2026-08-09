package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestEnabledVisibleMethodsForProvider(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{payment.TypeWxpay}, enabledVisibleMethodsForProvider(payment.TypeStripe, payment.TypeWxpay))
	require.Empty(t, enabledVisibleMethodsForProvider(payment.TypeStripe, "card,wxpay"))
}
