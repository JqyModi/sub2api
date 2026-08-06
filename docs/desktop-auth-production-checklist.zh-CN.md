# Codex 多开助手订阅服务生产上线检查清单

本清单用于将已通过本地验证的 Sub2API fork 部署为可真实收款的订阅服务。甲骨文实例、域名尚未就绪时，可先完成标记为“离线”的项目；标记为“服务器”的项目需等实例开通后执行。

## 1. 发布门槛

- [ ] `jqy/desktop-auth` 使用明确提交 SHA 构建，不直接跟随 `latest`。
- [ ] 桌面端只向测试用户开放订阅入口，生产验收完成后再写入正式 Release 默认地址。
- [ ] Stripe Live 模式只保留一个当前生产 Webhook Endpoint；删除或禁用临时隧道 Endpoint。
- [ ] 管理员账号启用 TOTP，日常账号与管理员账号分离。
- [ ] 已准备服务条款、隐私说明、退款规则、支持邮箱和上游服务合规说明。

## 2. 固定密钥与数据（离线）

在密码管理器中分别生成并保存以下值，不提交 Git：

```dotenv
POSTGRES_PASSWORD=<random>
ADMIN_PASSWORD=<random>
JWT_SECRET=<openssl-rand-hex-32>
TOTP_ENCRYPTION_KEY=<openssl-rand-hex-32>
PAYMENT_RESUME_SIGNING_KEY=<openssl-rand-hex-32>
```

- [ ] Stripe Live `sk_live`、`pk_live` 与 Webhook Secret 分开保存，测试/正式环境不混用。
- [ ] 上游账号凭据只进入 Sub2API 管理端加密存储，不进入桌面端、镜像或 Compose 文件。
- [ ] `.env` 权限为 `600`，数据库和备份目录不在 Git 工作树中。

## 3. 甲骨文实例与网络（服务器）

- [ ] 使用受支持的 64 位 Linux 镜像；确认 fork 镜像与实例架构一致。
- [ ] 仅开放 `22/80/443`；PostgreSQL、Redis、`8080` 不对公网开放。
- [ ] `BIND_HOST=127.0.0.1`，由 Caddy/Nginx 将 HTTPS 转发到 `127.0.0.1:8080`。
- [ ] 代理传递真实 `Host` 与 `X-Forwarded-Proto: https`，桌面授权 URL 和 API Base URL 均为生产域名。
- [ ] 配置可信代理、SSH 密钥登录、防火墙、系统自动安全更新和最小权限运行用户。
- [ ] 磁盘告警阈值设为 80%，日志启用滚动，至少预留一次完整数据库备份空间。

## 4. Stripe Live 配置（服务器）

管理端 Stripe 实例至少填写 `secretKey`、`publishableKey`、`webhookSecret`、`currency=HKD`，并启用实际需要的支付方式。

Stripe Dashboard 创建 Endpoint：

```text
https://<生产域名>/api/v1/payment/webhook/stripe
```

订阅事件：

```text
payment_intent.succeeded
payment_intent.payment_failed
```

- [ ] 用 Stripe Dashboard “发送测试事件”确认 Endpoint 返回 `2xx`。
- [ ] 创建最低金额 Live 订单，确认扣款、订单 `COMPLETED`、订阅生效、桌面授权自动继续。
- [ ] 暂停 Webhook 投递后完成一笔测试支付，确认结果页主动查单可恢复订单。
- [ ] 关闭结果页并重复测试，确认后台巡检在约 60 秒内恢复订单。
- [ ] 发起一笔退款，确认 Stripe 状态、本地订单和订阅扣减一致；订阅失效后原设备 Key 请求返回 `403`。

## 5. 授权与网关验收（服务器）

- [ ] 新用户：注册 -> 订阅页 -> 支付 -> 原授权页 -> App 授权完成，全程不落到 Dashboard。
- [ ] 老用户无订阅：登录 -> 订阅页 -> 支付 -> App 授权完成。
- [ ] 已订阅用户：登录后直接确认授权，不重复购买。
- [ ] PKCE verifier 错误时不能兑换 Key；正确 verifier 只能兑换一次。
- [ ] 设备 Key 绑定订阅分组，且 `expires_at` 与订阅到期时间一致。
- [ ] `/v1/models`、`/v1/responses`、SSE、工具调用和长上下文均使用真实上游通过。
- [ ] macOS 与 Windows 各创建、重启、再次打开一个订阅 Profile。

## 6. 监控、告警与人工处置（服务器）

至少采集容器存活、`/health`、CPU、内存、磁盘、PostgreSQL、Redis 和应用错误日志。首发可使用甲骨文监控加外部 HTTP 探活，不必先引入复杂监控栈。

建议告警：

| 条件 | 阈值 | 处置 |
| --- | --- | --- |
| `/health` 不可用 | 连续 2 分钟 | 检查容器、数据库、Redis、反向代理 |
| Stripe Webhook 非 `2xx` | 任意连续 3 次 | 核对 Secret、域名、实例配置和应用日志 |
| `failed to reconcile pending payment orders` | 连续 3 个周期 | 检查 Stripe API 网络与 `sk_live` |
| 待支付订单超过 10 分钟 | 任意订单 | 在 Stripe 按 PaymentIntent 查证，禁止直接手改订单 |
| 磁盘使用率 | 80%/90% | 清理日志或扩容，90% 时停止发布 |
| 数据库备份失败 | 任意一次 | 当日修复，修复前暂停高风险变更 |

## 7. 备份、恢复和回滚（服务器）

- [ ] 每日 `pg_dump`，保留至少 7 天；备份加密后保存到实例外部。
- [ ] 每周做一次恢复演练，验证用户、订单、订阅、Key 和支付实例均可读取。
- [ ] 记录当前镜像摘要和迁移版本；升级前备份 PostgreSQL 与 `deploy/data`。
- [ ] 回滚只切回已验证镜像/提交，不删除用户 Profile，不更换三个固定签名/加密密钥。
- [ ] 准备紧急开关：管理端关闭注册、停售套餐、禁用支付实例；保留登录、订单查询、退款与支持通道。

## 8. 正式开放判定

只有以下条件同时成立才能公开付费入口：

1. 两笔 Stripe Live 小额支付均自动完成，其中一笔专门验证 Webhook 丢失后的后台补单。
2. 一笔 Live 退款完整完成，退款后旧设备 Key 无法继续调用。
3. 数据库备份已在另一位置成功恢复。
4. macOS、Windows 与两个全新普通用户完成端到端验收。
5. 监控告警和紧急停售开关由实际操作验证，而不是只写在文档中。
