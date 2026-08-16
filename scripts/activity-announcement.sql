BEGIN;

UPDATE announcements
SET content = $content$
## 新用户送 $20，邀请好友再送 $20

Codex Multi Launcher 订阅服务上新福利已经开始：

- **新用户试用**：完成邮箱验证，自动领取 3 天试用，总额度 $20，每日最高 $10。
- **套餐同价加量**：轻量版 $300、标准版 $570、高频版 $950、重度版 $1,700 标准用量额度。
- **邀请好友**：在“邀请奖励”页面复制邀请链接；好友通过链接注册并完成首笔付费订阅后，邀请人自动获得 30 天 $20 奖励订阅。
- **一键接入**：在 Codex Multi Launcher 创建 Profile 时选择“订阅服务”，完成注册、购买和授权后即可使用，无需填写 API Key。

[中文图文教程](https://jqymodi.github.io/codex-multi-launcher/subscription.html) · [English guide](https://jqymodi.github.io/codex-multi-launcher/en/subscription.html)

活动期：2026 年 8 月 16 日至 9 月 15 日。具体价格、额度、库存及支付方式以购买页实时显示为准。遇到问题请联系 `jqy.tieniu@gmail.com`。

---

## Launch benefits

Verify your email to receive a **3-day $20 trial**. When a friend registers through your referral link and completes their first paid subscription, you receive a separate **30-day $20 reward subscription**. Paid plans also include increased standard usage quota during the launch activity.

Create a Profile in Codex Multi Launcher, choose **Subscription Service**, and finish registration, payment, and authorization. No API key setup is required.
$content$,
    status = 'active',
    notify_mode = 'popup',
    targeting = '{}'::jsonb,
    starts_at = NOW(),
    ends_at = '2026-09-16T00:00:00+08:00'::timestamptz,
    updated_by = 1,
    updated_at = NOW()
WHERE title = 'Codex 订阅上新福利｜Launch Benefits';

INSERT INTO announcements (
    title, content, status, notify_mode, targeting,
    starts_at, ends_at, created_by, updated_by, created_at, updated_at
)
SELECT
    'Codex 订阅上新福利｜Launch Benefits',
    $content$
## 新用户送 $20，邀请好友再送 $20

Codex Multi Launcher 订阅服务上新福利已经开始：

- **新用户试用**：完成邮箱验证，自动领取 3 天试用，总额度 $20，每日最高 $10。
- **套餐同价加量**：轻量版 $300、标准版 $570、高频版 $950、重度版 $1,700 标准用量额度。
- **邀请好友**：在“邀请奖励”页面复制邀请链接；好友通过链接注册并完成首笔付费订阅后，邀请人自动获得 30 天 $20 奖励订阅。
- **一键接入**：在 Codex Multi Launcher 创建 Profile 时选择“订阅服务”，完成注册、购买和授权后即可使用，无需填写 API Key。

[中文图文教程](https://jqymodi.github.io/codex-multi-launcher/subscription.html) · [English guide](https://jqymodi.github.io/codex-multi-launcher/en/subscription.html)

活动期：2026 年 8 月 16 日至 9 月 15 日。具体价格、额度、库存及支付方式以购买页实时显示为准。遇到问题请联系 `jqy.tieniu@gmail.com`。

---

## Launch benefits

Verify your email to receive a **3-day $20 trial**. When a friend registers through your referral link and completes their first paid subscription, you receive a separate **30-day $20 reward subscription**. Paid plans also include increased standard usage quota during the launch activity.

Create a Profile in Codex Multi Launcher, choose **Subscription Service**, and finish registration, payment, and authorization. No API key setup is required.
$content$,
    'active', 'popup', '{}'::jsonb,
    NOW(), '2026-09-16T00:00:00+08:00'::timestamptz,
    1, 1, NOW(), NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM announcements
    WHERE title = 'Codex 订阅上新福利｜Launch Benefits'
);

COMMIT;
