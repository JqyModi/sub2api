package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration192ClearsOnlyDesktopAuthorizationKeyExpiry(t *testing.T) {
	content, err := FS.ReadFile("192_clear_desktop_auth_api_key_expiry.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "UPDATE api_keys")
	require.Contains(t, sql, "SET expires_at = NULL")
	require.Contains(t, sql, "name LIKE 'Codex Multi Launcher - %'")
	require.Contains(t, sql, "expires_at IS NOT NULL")
	require.Contains(t, sql, "deleted_at IS NULL")
	require.NotContains(t, sql, "UPDATE user_subscriptions")
}
