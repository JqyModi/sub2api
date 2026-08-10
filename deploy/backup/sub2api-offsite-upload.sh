#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-daily}"
case "$MODE" in
  daily|weekly) ;;
  *) echo "Usage: $0 {daily|weekly}" >&2; exit 2 ;;
esac

BACKUP_ROOT="${SUB2API_BACKUP_ROOT:-/opt/sub2api/backups/rolling}"
BUCKET="${SUB2API_OCI_BACKUP_BUCKET:-sub2api-backups}"
REGION="${SUB2API_OCI_REGION:-ap-tokyo-1}"
LOCK_FILE="${SUB2API_OFFSITE_LOCK:-/run/lock/sub2api-offsite-backup.lock}"

command -v oci >/dev/null || { echo "OCI CLI is not installed." >&2; exit 1; }

exec 9>"$LOCK_FILE"
flock -n 9 || { echo "An offsite backup upload is already running." >&2; exit 1; }

LATEST="$(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name "$MODE-*" | sort | tail -n 1)"
[[ -n "$LATEST" ]] || { echo "No $MODE backup is available." >&2; exit 1; }

STAMP="$(basename "$LATEST")"
WORK_DIR="$(mktemp -d)"
ARCHIVE="$WORK_DIR/$STAMP.tar.gz"
trap 'rm -rf "$WORK_DIR"' EXIT

tar -C "$BACKUP_ROOT" -czf "$ARCHIVE" "$STAMP"
sha256sum "$ARCHIVE" > "$ARCHIVE.sha256"

for file in "$ARCHIVE" "$ARCHIVE.sha256"; do
  oci os object put \
    --auth instance_principal \
    --region "$REGION" \
    --bucket-name "$BUCKET" \
    --name "$MODE/$(basename "$file")" \
    --file "$file" \
    --force \
    --no-multipart >/dev/null
done

echo "Uploaded $STAMP to OCI Object Storage bucket $BUCKET."
