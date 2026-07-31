# Codex 多开助手桌面授权本地验证

本手册验证 Sub2API fork 的桌面授权接口和 Codex Multi Launcher 的订阅服务入口。测试环境使用本地 Docker Compose，不需要配置真实支付渠道；通过管理员直接分配测试订阅来模拟支付 Webhook 履约后的最终状态。

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

启动服务：

```bash
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml up -d --build
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml logs -f sub2api
```

确认 `http://127.0.0.1:8080/health` 返回 200。

## 准备订阅用户

1. 用自动创建的管理员账号登录 `http://127.0.0.1:8080`。
2. 创建一个订阅型分组，并创建可用的测试套餐。
3. 注册一个普通测试用户。
4. 在管理端为该用户分配这个分组的有效测试订阅。
5. 确认用户能在“我的订阅”看到有效期和对应分组。

这里的管理员分配等价于真实支付 Webhook 完成订单履约之后的状态。正式环境必须仍由 Sub2API 现有订单、Webhook 验签和履约流程写入订阅，不能用浏览器返回参数直接授权。

## 验证网页和 API

用普通测试用户登录后，访问：

```text
http://127.0.0.1:8080/desktop/authorize?session=<desktop-session-id>
```

预期：

1. 未登录时跳转登录，登录后保留 `session` 参数。
2. 没有有效订阅时页面提示购买，并持续等待服务端状态。
3. 管理端分配测试订阅后，页面自动完成授权。
4. `POST /api/v1/desktop-auth/token` 只能使用正确的 PKCE verifier 成功兑换一次。
5. 用户 API Key 列表中出现命名为 `Codex Multi Launcher - <device>` 的 Key，绑定到该订阅分组。
6. 停用或过期订阅后，Sub2API API Key 中间件应拒绝这个订阅型分组的请求。

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
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml down
```

仅在确认不再需要测试数据时删除 `deploy/data`、`deploy/postgres_data` 和 `deploy/redis_data`。
