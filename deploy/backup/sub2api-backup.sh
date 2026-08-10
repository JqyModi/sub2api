#!/usr/bin/env bash
# Create a root-readable, rolling backup for a local-directory Docker deployment.
set -euo pipefail

MODE="${1:-daily}"
case "$MODE" in
  daily) KEEP=7 ;;
  weekly) KEEP=4 ;;
  *) echo "Usage: $0 {daily|weekly}" >&2; exit 2 ;;
esac

DEPLOY_DIR="${SUB2API_DEPLOY_DIR:-/opt/sub2api/deploy}"
BACKUP_ROOT="${SUB2API_BACKUP_ROOT:-/opt/sub2api/backups/rolling}"
POSTGRES_CONTAINER="${SUB2API_POSTGRES_CONTAINER:-sub2api-postgres}"
LOCK_FILE="${SUB2API_BACKUP_LOCK:-/run/lock/sub2api-backup.lock}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="$BACKUP_ROOT/$MODE-$STAMP"

exec 9>"$LOCK_FILE"
flock -n 9 || { echo "A Sub2API backup is already running." >&2; exit 1; }

umask 077
install -d -m 700 "$DEST"
cleanup() { rm -rf "$DEST"; }
trap cleanup ERR

cd "$DEPLOY_DIR"

docker exec "$POSTGRES_CONTAINER" sh -c \
  'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges' \
  | gzip -9 > "$DEST/postgres.sql.gz"

if [[ -d redis_data ]]; then
  tar -C . -czf "$DEST/redis-data.tar.gz" redis_data
fi

CONFIG_FILES=()
for file in .env docker-compose.local.yml docker-compose.yml Caddyfile; do
  [[ -f "$file" ]] && CONFIG_FILES+=("$file")
done
if (( ${#CONFIG_FILES[@]} )); then
  tar -C . -czf "$DEST/config.tar.gz" "${CONFIG_FILES[@]}"
fi

{
  printf 'created_at_utc=%s\n' "$STAMP"
  printf 'mode=%s\n' "$MODE"
  printf 'postgres_container=%s\n' "$POSTGRES_CONTAINER"
  (cd "$DEST" && sha256sum *.gz)
} > "$DEST/manifest.txt"

mapfile -t OLD_BACKUPS < <(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name "$MODE-*" -printf '%f\n' | sort -r | tail -n +$((KEEP + 1)))
for backup in "${OLD_BACKUPS[@]}"; do
  rm -rf -- "$BACKUP_ROOT/$backup"
done

trap - ERR
echo "Sub2API $MODE backup created: $DEST"
