# v0.2.0 订阅增长活动记录

## 当前已上线

2026-08-16 已在生产环境完成 PostgreSQL 事务配置，未修改 Codex Multi Launcher 桌面 App。

| 项目 | 配置 |
| --- | --- |
| 邮箱注册试用 | 验证邮箱后自动获得 Starter Beta |
| 试用有效期 | 2 天 |
| 注册赠送余额 | `$10` |
| 试用日限额 | OpenAI 日 `$5` |
| 试用 RPM | 5 |
| 轻量版 | `$7.99` / `$300` / 日 `$30` / 周 `$100` / RPM 10 |
| 标准版 | `$14.99` / `$570` / 日 `$60` / 周 `$220` / RPM 15 |
| 高频版 | `$24.99` / `$950` / 日 `$100` / 周 `$350` / RPM 20 |
| 重度版 | `$44.99` / `$1,700` / 日 `$180` / 周 `$650` / RPM 25 |
| 销量上限 | 轻量 20、标准 20、高频 8、重度 3 |
| 邀请奖励 | 好友首笔付费订阅成功后，邀请人获得 30 天 `$20` 独立订阅额度 |

套餐额度按稳定上游约 `CNY 0.15 / 标准 USD` 估算，并预留支付、汇率和基础设施成本。当前有效订阅会直接读取所属分组的新限额，无需重新授权。

注册防滥用规则：邮箱注册来源发放 `$10` 余额、`starter-beta` 2 天订阅和 OpenAI 日限 `$5`；同一公网 IP 在 24 小时内最多获得 1 次注册赠送。三段及以上域名（例如 `magic668.eu.org`）以及后台配置的一次性邮箱黑名单会被拒绝注册，普通 `gmail.com`、`qq.com`、`163.com` 等两段域名不受影响。

## 邀请奖励实现

现有普通余额返利保持不变。为保证奖励可直接用于桌面授权，本活动额外创建不售卖的
`codex-invite-bonus` 订阅分组，复制 `codex-pro` 的上游路由，限额为日 `$10`、周/月
`$20`、5 RPM、有效期 30 天。

被邀请用户的首笔付费订阅完成后，服务端会：

1. 读取其已绑定的邀请人。
2. 为邀请人创建或续期奖励订阅。
3. 使用 `payment_audit_logs(order_id, action)` 唯一索引防止重复 Webhook 重复发放。
4. 退款完成时撤销该笔订单对应的奖励订阅。

奖励订阅与售卖套餐独立。现有桌面 App 已支持多个有效订阅的授权选择，因此不需要
更新 App；邀请人可在新建 Profile 或“重新授权”时选择该奖励订阅。

生产开关为 `affiliate_enabled=true`，用于在注册时绑定邀请关系并显示邀请入口；旧的
余额返利比例固定为 `affiliate_rebate_rate=0`。因此本活动不会叠加未宣传的余额返利，
对邀请人的权益只有上述独立订阅奖励。

奖励分组初始化脚本：

`scripts/activity-affiliate-reward.sql`

统计和排查以以下审计动作为准：

- `AFFILIATE_SUBSCRIPTION_REWARD_APPLIED`
- `AFFILIATE_SUBSCRIPTION_REWARD_REVOKED`

## 部署验证

2026-08-16 已在 OCI 使用容器镜像完成部署。验证项：

1. Go 后端和前端生产构建通过。
2. `https://sub2api.minai.eu.org/health` 返回 `{"status":"ok"}`。
3. `codex-invite-bonus` 为 active OpenAI 订阅分组，保有 5 条与 Pro 一致的上游路由。
4. `payment_audit_logs` 存在 `(order_id, action)` 唯一索引，保障重复支付通知的奖励幂等。
5. 邀请页面与中英文操作教程已说明活动规则；桌面 App 无需更新。
6. 支付概览漏斗已增加“试用已发放”和“邀请奖励已发放”两项，分别从 `starter-beta`
   订阅记录及奖励审计记录计算。

关键自动测试已在 OCI 的 Go 1.26.5 容器通过：

- `TestGetGrowthFunnelStatsIncludesTrialAndReferralRewardStages`
- `TestAffiliateSubscriptionRewardIsIdempotentAndRefundRevokesIt`
- `TestAuthService_Register_UsesEmailAuthSourceDefaultsWhenGrantEnabled`

## 回滚

部署前数据库备份保存在 OCI 实例：

`/opt/sub2api/backups/activity/pre-growth-20260815T174333Z.sql.gz`

校验值：

`7fb4fd1e91895cb4a0bc378795e7fe3ac5c7897269a11ba863b8db68f6eff5ef`

邀请奖励部署前的额外备份：

`/opt/sub2api/backups/activity/pre-referral-reward-20260815T182809Z.sql.gz`

校验值：

`389f9d91b3be8d9801f161509ddbca886b31fa1cf6e3165d9873498d7fb4294f`

启用邀请关系绑定前的额外备份：

`/opt/sub2api/backups/activity/pre-affiliate-enable-20260815T185600Z.sql.gz`

校验值：

`cbdc84f46327505e59dfe97a50159876d58ce9f33d63debc82bae2eb51ba43da`

回滚前应停止写入并由管理员确认。不要直接删除现有用户订阅记录。
