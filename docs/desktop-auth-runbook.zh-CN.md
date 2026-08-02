# Codex 多开助手订阅服务运行手册

> 适用分支：`jqy/desktop-auth`
>
> 对应桌面端分支：`jqy/sub2api-desktop-auth`
>
> 状态：本地 Docker、注册、真实订单、模拟签名支付回调、浏览器授权、桌面端授权和 Profile 创建闭环已验证；生产商户、上游账号和公网运维尚未上线。

本手册记录 Codex Multi Launcher（以下简称“桌面端”）与本 fork 的 Sub2API 之间的订阅授权实现、部署方式、验收方法和边界。它是当前实现的运行记录，不包含管理员密码、支付凭据、上游账号密钥或任何测试用户凭据。

## 1. 职责边界

| 组件 | 负责内容 | 不负责内容 |
| --- | --- | --- |
| Codex Multi Launcher | 创建授权会话、打开浏览器、轮询授权结果、创建本地 Profile、加密保存用户设备 Key | 用户注册、套餐、支付、上游账号、额度和并发管理 |
| Sub2API fork | 用户登录、套餐/订单/订阅、桌面授权页、设备 Key 创建、API 网关、额度和并发 | 管理本地 Codex Profile、保存桌面端管理员凭据 |
| PostgreSQL | 用户、套餐、订阅、订单、Key、配置等持久数据 | 短期桌面授权会话 |
| Redis | 30 分钟桌面授权会话和一次性兑换状态 | 持久订阅和用户数据 |

桌面端不会拿到管理员 Token、上游账号 Token 或支付密钥。浏览器授权页也不显示设备 Key；Key 只在正确的 PKCE verifier 兑换成功后返回给发起该会话的桌面主进程。

## 2. 已实现授权链路

```text
桌面端选择“订阅服务”
  -> POST /api/v1/desktop-auth/sessions（携带 PKCE code_challenge）
  -> 服务端将 30 分钟会话写入 Redis，返回 /desktop/authorize?session=...
  -> 系统浏览器登录 Sub2API，授权页自动检查当前订阅
  -> 新用户注册后直接进入“订阅”购买页；已有无订阅用户登录后也自动进入购买页
  -> 支付成功后自动回到原授权会话
  -> 服务端检查有效订阅，创建一个绑定订阅分组的用户设备 Key
  -> 桌面端轮询 POST /api/v1/desktop-auth/token（携带 code_verifier）
  -> 服务端校验 PKCE，单次返回 base_url、设备 Key、模型、到期时间
  -> 桌面端用既有加密存储保存 Key，自动创建并打开 Profile
```

实现要点：

- 固定客户端标识为 `codex-multi-launcher`。
- 会话有效期 30 分钟，默认轮询间隔 2 秒，为注册和第三方支付预留合理操作时间。
- `POST /token` 在成功后删除 Redis 会话；相同会话再次兑换返回 `410 Gone`。
- 没有有效订阅时授权状态变为 `payment_required`，页面在同一标签页直接进入 `purchase?tab=subscription`。
- 购买页携带有效桌面授权 `redirect` 时，Stripe/易支付等跳转支付会继续使用当前标签页，不调用 `window.open`；普通独立充值仍保留桌面弹窗方式。
- 授权路径只允许站内 `/desktop/authorize?session=...`。登录、注册、购买、Stripe/Airwallex/易支付结果页都会保留该路径，支付履约后自动返回并继续授权；外部或畸形回跳地址会被拒绝。
- 当前会选择用户的第一条有效订阅并为其分组创建一个 Key；尚没有“选择套餐/设备列表/设备撤销”专项界面。
- 授权 URL 和返回的 `base_url` 必须与桌面端配置的订阅服务同源。公网服务应使用 HTTPS；仅 `localhost`、`127.0.0.1`、`::1` 本地开发地址允许 HTTP。

相关实现：

- 服务端路由：[desktop_auth.go](../backend/internal/server/routes/desktop_auth.go)
- HTTP 处理：[desktop_auth_handler.go](../backend/internal/handler/desktop_auth_handler.go)
- PKCE、Redis 会话和设备 Key 编排：[desktop_auth_service.go](../backend/internal/service/desktop_auth_service.go)
- 授权网页：[DesktopAuthorizationView.vue](../frontend/src/views/user/DesktopAuthorizationView.vue)
- 桌面端集成说明：`JqyModi/codex-multi-launcher` 的 `docs/sub2api-desktop-auth-plan.zh-CN.md`

## 3. 容器与持久数据

本地/单机正式 Compose 使用三项服务：

| 容器 | 默认镜像 | 作用 | 本地持久目录 |
| --- | --- | --- | --- |
| `sub2api` | 当前 fork 构建镜像 | Web、管理端、桌面授权接口、OpenAI 兼容网关 | `deploy/data` |
| `sub2api-postgres` | `postgres:18-alpine` | 账户、订阅、订单、Key、配置 | `deploy/postgres_data` |
| `sub2api-redis` | `redis:8-alpine` | 授权会话、缓存和短期状态 | `deploy/redis_data` |

`docker-compose.local.yml` 单独启动时使用上游 `weishaw/sub2api:latest`，不包含桌面授权接口。必须叠加 `docker-compose.codex-auth.yml`，从本 fork 构建 `sub2api` 镜像。

完整购买验收可再叠加 `docker-compose.desktop-auth-test.yml`，增加第四个 `fake-easypay` 容器。它只监听宿主机 `127.0.0.1:8090`，用于模拟商户支付页和签名 Webhook；生产环境不得加载该 overlay。

## 4. 首次部署

### 4.1 前提

- Linux 服务器或本地开发环境已安装 Docker Engine 与 Docker Compose。
- 首次从源码构建建议至少 4 GB 内存；生产建议将验证过的镜像推送至私有镜像仓库，运行节点只拉取镜像。
- 公网部署需准备域名和 HTTPS 反向代理；不要将 `8080` 直接公开暴露。

### 4.2 创建配置

```bash
git clone --branch jqy/desktop-auth https://github.com/JqyModi/sub2api.git
cd sub2api/deploy
cp .env.example .env
chmod 600 .env
mkdir -p data postgres_data redis_data
```

在 `.env` 中至少设置以下值。密码和随机密钥应存入密码管理器，不要提交到 Git：

```dotenv
BIND_HOST=127.0.0.1
SERVER_PORT=8080
SERVER_MODE=release
TZ=Asia/Shanghai

POSTGRES_USER=sub2api
POSTGRES_PASSWORD=<strong-random-password>
POSTGRES_DB=sub2api

ADMIN_EMAIL=<administrator-email>
ADMIN_PASSWORD=<strong-admin-password>
JWT_SECRET=<openssl-rand-hex-32>
TOTP_ENCRYPTION_KEY=<openssl-rand-hex-32>
```

可用以下命令生成两个固定密钥：

```bash
openssl rand -hex 32
```

`JWT_SECRET` 与 `TOTP_ENCRYPTION_KEY` 不能在重启或升级时随意更换，否则现有登录会话或已配置的 TOTP 会失效。

### 4.3 构建并启动

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.codex-auth.yml \
  up -d --build
```

检查三项服务和健康接口：

```bash
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml ps
curl --fail http://127.0.0.1:8080/health
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml logs -f sub2api
```

预期 `sub2api`、`sub2api-postgres`、`sub2api-redis` 均为 `healthy`，健康接口返回：

```json
{"status":"ok"}
```

首次自动初始化会连接 PostgreSQL 和 Redis、执行数据库迁移、创建管理员账号并写入运行配置。不要在数据库尚未健康时反复重启 `sub2api`。

## 5. HTTPS 与反向代理

正式服务地址必须是 `https://service.example.com`。反向代理应只将流量转发至同机 `127.0.0.1:8080`，并正确传递：

```text
Host: service.example.com
X-Forwarded-Proto: https
```

授权 URL 和最终 API Base URL 都由请求来源生成。没有 `X-Forwarded-Proto` 时，服务端会根据实际连接判断 HTTP/HTTPS：这使本地 `http://127.0.0.1:8080` 可用，同时不会把公网 HTTP 当作 HTTPS。代理遗漏或伪造协议头会导致桌面端以“授权地址不可信”拒绝继续。

桌面端以环境变量指定服务地址：

```bash
CODEX_PROFILE_MANAGER_SUBSCRIPTION_SERVICE_URL=https://service.example.com npm run dev
```

本地开发使用：

```bash
CODEX_PROFILE_MANAGER_SUBSCRIPTION_SERVICE_URL=http://127.0.0.1:8080 npm run dev
```

生产部署还应阅读 [EDGE_SECURITY.md](../deploy/EDGE_SECURITY.md)，配置可信代理、TLS、访问控制、限流和备份策略。

## 6. 管理端准备与用户流程

在 `https://service.example.com` 登录管理员账号后：

1. 完成 Sub2API 管理端要求的合规确认。
2. 创建订阅型分组，配置可用上游、分组额度和并发策略。
3. 创建套餐并接入支付渠道；本地开发可按专项手册使用模拟易支付完整验证订单与履约。
4. 确认用户在“我的订阅”能看到有效订阅和对应分组。
5. 用户在桌面端创建 Profile 时选择“订阅服务”，点击“前往授权”，在浏览器登录并确认。

服务端为该用户创建的 Key 名称以 `Codex Multi Launcher - ` 开头，归属用户并绑定授权时选中的订阅分组。管理员可通过既有 Sub2API 用户/Key/订阅功能查看、停用或删除它。

## 7. 验收与自动化检查

### 本地手测

1. 按 [本地验证手册](./desktop-auth-local-verification.zh-CN.md) 启动四容器并执行初始化脚本。
2. 在桌面端创建向导选择“订阅服务”，点击“前往授权”。
3. 注册全新普通用户，确认注册后直接进入带 `tab=subscription` 的购买页，不经过 Dashboard。
4. 选择套餐，确认原浏览器标签从购买页进入本地模拟支付页，且没有新建支付窗口或标签。
5. 完成支付，确认同一标签自动返回原 `/desktop/authorize?session=...`，页面显示接入成功。
6. 回到桌面端，确认轮询状态变为已授权并自动聚焦 App，然后创建 Profile。
7. 打开 Profile，确认 `/v1/models` 和 `/v1/responses` 请求可用。
8. 重启桌面端后再次打开该 Profile，确认加密保存的 Key 仍可用。

自动验证命令：

```bash
cd deploy
node testing/bootstrap-desktop-auth-test.mjs
node testing/verify-desktop-auth-purchase.mjs
```

### CI 和专项测试

- `.github/workflows/desktop-auth-ci.yml`：服务端桌面授权逻辑测试。
- `.github/workflows/desktop-auth-compose-smoke.yml`：全新 Compose 环境下管理员合规确认、建分组、建用户、分配订阅、PKCE 授权、单次兑换的完整冒烟测试。
- `backend/internal/handler/desktop_auth_handler_test.go`：直接 HTTP、TLS、反向代理协议和 HTTP 授权流程测试。
- 桌面端 `scripts/verify-subscription-auth.mjs`、`scripts/verify-subscription-profile-create.mjs`：主进程授权校验与 Profile 创建验证。

## 8. 日常运维、升级与回滚

升级前必须备份 `data`、`postgres_data` 和 `redis_data`。其中 PostgreSQL 最重要，Redis 丢失只会让尚未完成的 30 分钟授权会话失效。

```bash
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml down
# 备份三个目录后，切换到已验证提交
git fetch origin
git switch jqy/desktop-auth
git pull --ff-only
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml up -d --build
```

出现服务端回归时，切回上一个已验证提交后重新构建镜像。不要通过删除用户本地 Profile 来回滚服务端；这不会撤销已签发的设备 Key。停止新授权入口时，应保留既有 API Key、订阅查询、退款和用户支持能力。

排障优先级：

| 现象 | 首先检查 |
| --- | --- |
| 授权地址不可信 | 桌面端服务地址、授权 URL 是否同源、`X-Forwarded-Proto`、本地 HTTP/公网 HTTPS 规则 |
| 认证页显示无有效订阅 | 用户登录身份、有效订阅、订阅分组、支付 Webhook 履约 |
| 授权后桌面端一直等待 | `sub2api` 日志、Redis 健康、支付结果是否保留 `redirect`、会话是否超过 30 分钟 |
| Profile 创建失败 | 桌面端主进程日志、服务端 `/v1/models` 和 `/v1/responses`、分组上游路由 |
| 重启后异常 | `.env` 中 JWT/TOTP 密钥是否被替换、PostgreSQL 数据目录和磁盘空间 |

## 9. 当前边界和上线前必做项

本地授权闭环已通过，但以下不是“已上线能力”：

- 本地已验证 EasyPay MD5 请求签名、错误回调拒绝、正确 Webhook 验签和订阅履约；真实商户通道、退款和风控策略仍需按运营方案配置和验证。
- 上游账号池、Codex 长上下文、SSE、工具调用、配额和并发应基于实际供应商做压测。
- 设备模型、设备数量限制、可视化设备撤销和 Key 自动轮换尚未单独实现；当前可通过既有 Key 管理能力人工处理。
- 订阅变更、过期或退款后已签发 Key 的即时限制策略需要结合实际网关/分组规则进行生产验收。
- 公网域名、TLS、备份恢复演练、监控告警、服务条款、隐私说明及上游转售合规必须在开放付费前完成。

相关的早期部署说明见 [desktop-auth-deployment.zh-CN.md](./desktop-auth-deployment.zh-CN.md)，本地验证步骤见 [desktop-auth-local-verification.zh-CN.md](./desktop-auth-local-verification.zh-CN.md)。
