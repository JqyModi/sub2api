# Codex 多开助手桌面授权本地验证

本手册验证 Sub2API fork 的桌面授权接口和 Codex Multi Launcher 的订阅服务入口。测试环境使用本地 Docker Compose 和仅限本地的模拟易支付服务，不需要真实商户凭据，但仍会创建真实订单、校验支付签名、处理 Webhook 并执行真实订阅履约。

## 前提

- Docker Engine 和 Docker Compose 可用。
- 已克隆本 fork 的 `jqy/desktop-auth` 分支。
- 已克隆 Codex Multi Launcher 的 `jqy/sub2api-desktop-auth` 分支。
- 使用本地测试数据，不使用生产用户、支付凭据或上游账号。

## 启动 Sub2API

在 Sub2API fork 的 `deploy` 目录执行。必须叠加 `docker-compose.codex-auth.yml` 构建当前 fork；直接使用默认 Compose 会拉取上游镜像，桌面授权接口不存在。

```bash
cd deploy
cp .env.example .env
```

在 `.env` 中至少设置独立的测试密码和固定密钥：

```dotenv
POSTGRES_PASSWORD=<test-only-password>
ADMIN_PASSWORD=<test-only-admin-password>
JWT_SECRET=<random-64-hex>
TOTP_ENCRYPTION_KEY=<random-64-hex>
BIND_HOST=127.0.0.1
SERVER_PORT=8080
```

启动服务。第三个 overlay 会额外启动 `fake-easypay`，只能用于本地开发验收，生产环境禁止加载：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.codex-auth.yml \
  -f docker-compose.desktop-auth-test.yml \
  up -d --build
```

确认 `http://127.0.0.1:8080/health` 与 `http://127.0.0.1:8090/health` 均返回 200。

## 初始化注册、套餐和支付

执行可重复运行的初始化脚本：

```bash
node testing/bootstrap-desktop-auth-test.mjs
```

脚本会通过管理员 API 开启注册和支付，创建或更新：

- 订阅分组 `Desktop Local Test`；
- 可售套餐 `Desktop Local Monthly`；
- 支付实例 `Desktop Local Fake EasyPay`；
- 支付宝可见入口和本地购买地址。

本地模拟商户固定值仅存在于测试 overlay 与初始化脚本中。它不能作为正式商户密钥，也不能部署到公网。

## 自动验收真实购买闭环

```bash
node testing/verify-desktop-auth-purchase.mjs
```

每次执行都会创建一个全新的普通用户，并验证：

1. 无订阅用户确认授权时得到 `payment_required`。
2. 用户购买套餐后生成真实 subscription order。
3. 错误 EasyPay 签名回调得到 HTTP 400。
4. 模拟支付页提交正确签名 Webhook。
5. 订单变为 `COMPLETED`，用户得到有效订阅。
6. 桌面授权创建 `Codex Multi Launcher - <device>` Key。
7. PKCE 兑换得到 `/v1` 地址和设备 Key，且同一会话不能兑换第二次。

## 验证网页和 API

用普通测试用户登录后，访问：

```text
http://127.0.0.1:8080/desktop/authorize?session=<desktop-session-id>
```

预期：

1. 未登录时跳转登录，登录后保留 `session` 参数。
2. 没有有效订阅时页面提示购买，并持续等待服务端状态。
3. 点击“查看套餐”，选择 `Desktop Local Monthly`，使用支付宝创建订单。
4. 浏览器打开 `http://127.0.0.1:8090/pay?...`，点击“模拟支付成功”。
5. 支付回调完成后授权页面自动继续。
6. `POST /api/v1/desktop-auth/token` 只能使用正确的 PKCE verifier 成功兑换一次。
7. 用户 API Key 列表中出现命名为 `Codex Multi Launcher - <device>` 的 Key，绑定到该订阅分组。
8. 停用或过期订阅后，Sub2API API Key 中间件应拒绝这个订阅型分组的请求。

## 验证桌面端

在 Codex Multi Launcher 仓库中，以本地服务地址运行 Electron：

```bash
CODEX_PROFILE_MANAGER_SUBSCRIPTION_SERVICE_URL=http://127.0.0.1:8080 npm run dev
```

在创建 Profile 向导中选择“订阅服务”，然后完成：

1. 点击“前往授权”，浏览器打开 Sub2API 授权页。
2. 以测试用户登录并完成授权。
3. 回到桌面端，状态自动变为“授权已完成”。
4. 生成 Profile，不填写 Base URL、模型或 API Key。
5. 打开新 Profile，验证 `GET /v1/models` 和 `/v1/responses` 可用。
6. 重启 Codex Multi Launcher 后再次打开 Profile，确认加密保存的设备 Key 仍可用。

## 清理

测试结束后停止本地服务：

```bash
cd deploy
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.codex-auth.yml \
  -f docker-compose.desktop-auth-test.yml \
  down
```

仅在确认不再需要测试数据时删除 `deploy/data`、`deploy/postgres_data` 和 `deploy/redis_data`。
