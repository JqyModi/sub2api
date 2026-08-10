# Sub2API 滚动备份与恢复

`sub2api-backup` 适用于 `docker-compose.local.yml` 的本地目录部署。每次备份生成一个仅 root 可读的时间戳目录，包含：

- `postgres.sql.gz`：PostgreSQL 逻辑备份。
- `redis-data.tar.gz`：Redis 持久化目录。
- `config.tar.gz`：`.env`、Compose 与 Caddy 配置。
- `manifest.txt`：归档 SHA-256 校验值。

## 已安装的计划任务

- 每日 `02:20` 执行，保留最新 7 份。
- 每周日 `02:40` 执行，保留最新 4 份。

查看日志与备份：

```bash
sudo tail -n 100 /var/log/sub2api-backup.log
sudo find /opt/sub2api/backups/rolling -maxdepth 2 -type f -printf '%p %s\n'
```

手动创建一份：

```bash
sudo /usr/local/sbin/sub2api-backup daily
```

## OCI Object Storage 异地副本

`sub2api-offsite-upload.sh` 将最新每日/每周目录打包后，通过 OCI Instance Principal 上传到私有 Bucket。服务器无需保存 OCI 用户 API Key；对象默认使用 OCI 服务端加密。

安装完成后的计划任务：每日 `03:10` 上传每日副本，每周日 `03:30` 上传每周副本。建议 Bucket 生命周期分别保留每日 35 天、每周 180 天。

手动上传与查看日志：

```bash
sudo /usr/local/sbin/sub2api-offsite-upload daily
sudo tail -n 100 /var/log/sub2api-offsite-backup.log
```

## 恢复演练

在维护窗口执行；恢复会覆盖当前数据库与 Redis 数据。先停止应用，再保留当前目录作为额外回退点。

```bash
cd /opt/sub2api/deploy
BACKUP=/opt/sub2api/backups/rolling/daily-YYYYMMDDTHHMMSSZ

sudo docker compose -f docker-compose.local.yml stop sub2api
sudo tar -xzf "$BACKUP/config.tar.gz" -C .
sudo mv redis_data "redis_data.before-restore.$(date +%s)"
sudo tar -xzf "$BACKUP/redis-data.tar.gz" -C .

sudo docker compose -f docker-compose.local.yml up -d postgres redis
sleep 5
sudo zcat "$BACKUP/postgres.sql.gz" | sudo docker exec -i sub2api-postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"
sudo docker compose -f docker-compose.local.yml up -d
```

恢复前可通过 `sha256sum -c manifest.txt` 校验归档。每周应将一份备份复制到另一处存储，避免整台实例不可用时同盘备份一并丢失。

上线前及每月执行一次无损恢复演练。脚本会创建临时 PostgreSQL/Redis 容器，恢复并核对核心表，完成后自动清理，不修改生产容器：

```bash
sudo /usr/local/sbin/sub2api-restore-verify
```
