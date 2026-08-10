#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="${1:-}"
if [[ -z "$BACKUP_DIR" ]]; then
  BACKUP_DIR="$(find "${SUB2API_BACKUP_ROOT:-/opt/sub2api/backups/rolling}" -mindepth 1 -maxdepth 1 -type d -name 'daily-*' | sort | tail -n 1)"
fi
[[ -d "$BACKUP_DIR" ]] || { echo "Backup directory was not found." >&2; exit 1; }

for file in postgres.sql.gz redis-data.tar.gz manifest.txt; do
  [[ -s "$BACKUP_DIR/$file" ]] || { echo "Missing backup file: $file" >&2; exit 1; }
done

STAMP="$(date +%s)"
PG_CONTAINER="sub2api-restore-pg-$STAMP"
REDIS_CONTAINER="sub2api-restore-redis-$STAMP"
WORK_DIR="$(mktemp -d)"

cleanup() {
  docker rm -f "$PG_CONTAINER" "$REDIS_CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

(cd "$BACKUP_DIR" && grep -E '^[0-9a-f]{64}  ' manifest.txt | sha256sum -c -)

docker run -d --name "$PG_CONTAINER" \
  -e POSTGRES_USER=sub2api_restore \
  -e POSTGRES_PASSWORD=restore-only-password \
  -e POSTGRES_DB=sub2api_restore \
  postgres:18-alpine >/dev/null

for _ in $(seq 1 30); do
  if docker exec "$PG_CONTAINER" pg_isready -U sub2api_restore -d sub2api_restore >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$PG_CONTAINER" pg_isready -U sub2api_restore -d sub2api_restore >/dev/null
zcat "$BACKUP_DIR/postgres.sql.gz" | docker exec -i "$PG_CONTAINER" \
  psql -v ON_ERROR_STOP=1 -U sub2api_restore -d sub2api_restore >/dev/null

TABLE_COUNTS="$(docker exec "$PG_CONTAINER" psql -At -U sub2api_restore -d sub2api_restore -c \
  "SELECT 'users=' || count(*) FROM users
   UNION ALL SELECT 'payment_orders=' || count(*) FROM payment_orders
   UNION ALL SELECT 'user_subscriptions=' || count(*) FROM user_subscriptions
   UNION ALL SELECT 'api_keys=' || count(*) FROM api_keys;")"

tar -xzf "$BACKUP_DIR/redis-data.tar.gz" -C "$WORK_DIR"
docker run -d --name "$REDIS_CONTAINER" \
  -v "$WORK_DIR/redis_data:/data" \
  redis:8-alpine redis-server --appendonly yes >/dev/null
for _ in $(seq 1 20); do
  if docker exec "$REDIS_CONTAINER" redis-cli ping 2>/dev/null | grep -q PONG; then
    break
  fi
  sleep 1
done
docker exec "$REDIS_CONTAINER" redis-cli ping | grep -q PONG
REDIS_KEYS="$(docker exec "$REDIS_CONTAINER" redis-cli dbsize | tr -d '\r')"

printf 'PostgreSQL restore verified:\n%s\n' "$TABLE_COUNTS"
printf 'Redis restore verified: keys=%s\n' "$REDIS_KEYS"
