-- Codex Multi Launcher launch activity: referral subscription reward.
-- Safe to run repeatedly. It creates no public plan.
BEGIN;

DO $$
DECLARE
    source_group_id BIGINT;
    reward_group_id BIGINT;
BEGIN
    SELECT id
    INTO source_group_id
    FROM groups
    WHERE name = 'codex-pro'
      AND deleted_at IS NULL
    FOR UPDATE;

    IF source_group_id IS NULL THEN
        RAISE EXCEPTION 'source group codex-pro was not found';
    END IF;

    SELECT id
    INTO reward_group_id
    FROM groups
    WHERE name = 'codex-invite-bonus'
      AND deleted_at IS NULL
    FOR UPDATE;

    IF reward_group_id IS NULL THEN
        INSERT INTO groups (
            name, description, platform, subscription_type, status,
            rate_multiplier, daily_limit_usd, weekly_limit_usd,
            monthly_limit_usd, default_validity_days, rpm_limit, sort_order
        ) VALUES (
            'codex-invite-bonus',
            'Invite bonus / 邀请好友首笔订阅奖励：30 days, up to $20 standard usage.',
            'openai', 'subscription', 'active',
            1.0, 10.0, 20.0, 20.0, 30, 5, 5
        )
        RETURNING id INTO reward_group_id;
    ELSE
        UPDATE groups
        SET description = 'Invite bonus / 邀请好友首笔订阅奖励：30 days, up to $20 standard usage.',
            platform = 'openai',
            subscription_type = 'subscription',
            status = 'active',
            rate_multiplier = 1.0,
            daily_limit_usd = 10.0,
            weekly_limit_usd = 20.0,
            monthly_limit_usd = 20.0,
            default_validity_days = 30,
            rpm_limit = 5,
            sort_order = 5,
            updated_at = NOW()
        WHERE id = reward_group_id;
    END IF;

    INSERT INTO account_groups (account_id, group_id, priority)
    SELECT account_id, reward_group_id, priority
    FROM account_groups
    WHERE group_id = source_group_id
    ON CONFLICT (account_id, group_id)
    DO UPDATE SET priority = EXCLUDED.priority;
END $$;

COMMIT;
