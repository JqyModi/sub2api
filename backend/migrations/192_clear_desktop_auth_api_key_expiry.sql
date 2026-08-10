-- Desktop-authorized API keys are governed by the user's active subscription
-- for the key's group. Legacy versions copied the subscription expiry onto the
-- key, which prevented same-group renewals from reviving an existing profile.
UPDATE api_keys
SET expires_at = NULL,
    updated_at = NOW()
WHERE name LIKE 'Codex Multi Launcher - %'
  AND expires_at IS NOT NULL
  AND deleted_at IS NULL;
