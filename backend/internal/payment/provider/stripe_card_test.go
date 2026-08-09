package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterStripeCardMethodTypes(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"card", "link"}, filterStripeCardMethodTypes([]string{"card", "wechat_pay", "link"}))
	require.Equal(t, []string{"card"}, filterStripeCardMethodTypes([]string{"wechat_pay"}))
}
