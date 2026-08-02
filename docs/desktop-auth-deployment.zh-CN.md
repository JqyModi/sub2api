# Codex 多开助手授权服务部署

本文件用于快速部署包含桌面授权接口的 Sub2API fork。完整的架构、容器、配置、验收、运维边界和排障说明见 [desktop-auth-runbook.zh-CN.md](./desktop-auth-runbook.zh-CN.md)。不要只运行
`docker-compose.local.yml`：其默认镜像是上游 `weishaw/sub2api:latest`，不含
`/api/v1/desktop-auth`。

## 适用范围

- 代码分支：`jqy/desktop-auth`。
- 服务端：Sub2API、PostgreSQL、Redis 同机部署。
- 首次从源码构建 fork 镜像建议至少 4 GB 内存；构建完成后可将镜像推送到私有镜像仓库，避免在低配运行节点重复构建。
- 首次封闭测试可绑定到 `127.0.0.1`，通过 HTTPS 反向代理向测试设备开放。
- 不在 `.env`、Git 仓库或桌面端保存支付密钥、上游账号密钥和管理员 Token。

## 启动

```bash
git clone --branch jqy/desktop-auth https://github.com/JqyModi/sub2api.git
cd sub2api/deploy
cp .env.example .env
chmod 600 .env
```

在 `.env` 中至少替换以下值，并保存在密码管理器中：

```dotenv
POSTGRES_PASSWORD=<random-password>
ADMIN_PASSWORD=<admin-password>
JWT_SECRET=<openssl-rand-hex-32>
TOTP_ENCRYPTION_KEY=<openssl-rand-hex-32>
SERVER_MODE=release
BIND_HOST=127.0.0.1
```

从 `deploy` 目录启动 fork 镜像和依赖服务：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.codex-auth.yml \
  up -d --build
```

检查服务：

```bash
curl --fail http://127.0.0.1:8080/health
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml logs -f sub2api
```

## HTTPS 与桌面端

正式环境必须由反向代理终结 HTTPS，并仅转发到本机的 `127.0.0.1:8080`。桌面端只接受 HTTPS 服务地址，本地开发时才允许 `http://127.0.0.1`。

确认反向代理健康后，以实际域名运行桌面端：

```bash
CODEX_PROFILE_MANAGER_SUBSCRIPTION_SERVICE_URL=https://api.example.com npm run dev
```

## 首次验收

按 [本地验证手册](./desktop-auth-local-verification.zh-CN.md) 完成以下检查：

1. 创建桌面授权会话并在浏览器登录。
2. 无订阅时显示购买状态；订阅履约后页面自动继续授权。
3. 仅用正确 PKCE verifier 兑换一次设备 Key。
4. 用“订阅服务”创建 Profile，验证 `/v1/models` 和 `/v1/responses`。
5. 重启桌面 App 后再次打开该 Profile。

完成前不要把订阅服务入口写入正式桌面 Release 默认配置。

## 更新与回滚

更新前先备份 `data`、`postgres_data` 和 `redis_data`，然后在仓库中切换到经 CI 验证的提交并重新构建：

```bash
git fetch origin
git switch jqy/desktop-auth
git pull --ff-only
docker compose -f docker-compose.local.yml -f docker-compose.codex-auth.yml up -d --build
```

发生服务端回归时，切回上一个已验证提交并重复构建；不要通过桌面端删除用户 Profile 或本地加密凭据来回滚。
