#!/usr/bin/env bash
set -euo pipefail

IFS=$'\n\t'
umask 077

readonly DATA_DIR="${FORGE_DATA_DIR:-/var/lib/forge/data}"
readonly BACKUP_DIR="${FORGE_BACKUP_DIR:-/var/backups/forge}"
readonly SERVICE="${FORGE_SERVICE:-forge}"
readonly DB_NAME="${FORGE_DB_NAME:-forge.db}"
readonly LOCK_FILE="${FORGE_BACKUP_LOCK:-${BACKUP_DIR}/.forge-backup.lock}"
readonly DB_PATH="${DATA_DIR}/${DB_NAME}"
readonly TESTDATA_DIR="${DATA_DIR}/testdata"

STAGE_DIR=""
SERVICE_NEEDS_RESTART=0

log() {
    printf '[forge-backup] %s\n' "$*" >&2
}

die() {
    printf '[forge-backup] ERROR: %s\n' "$*" >&2
    exit 1
}

cleanup() {
    local status=$?
    if [[ -n "$STAGE_DIR" && -d "$STAGE_DIR" ]]; then
        rm -rf -- "$STAGE_DIR"
    fi
    if [[ "$SERVICE_NEEDS_RESTART" -eq 1 ]]; then
        log "restarting ${SERVICE}.service"
        if ! systemctl start "$SERVICE"; then
            printf '[forge-backup] ERROR: failed to restart %s.service\n' "$SERVICE" >&2
        fi
    fi
    exit "$status"
}

trap cleanup EXIT

for command in flock sqlite3 tar gzip sha256sum systemctl find sort; do
    command -v "$command" >/dev/null 2>&1 || die "required command not found: $command"
done

[[ "$DATA_DIR" == /* ]] || die "FORGE_DATA_DIR must be an absolute path"
[[ "$BACKUP_DIR" == /* ]] || die "FORGE_BACKUP_DIR must be an absolute path"
[[ "$BACKUP_DIR" != "/" ]] || die "FORGE_BACKUP_DIR must not be /"
[[ -d "$DATA_DIR" ]] || die "data directory does not exist: $DATA_DIR"
[[ -f "$DB_PATH" ]] || die "SQLite database does not exist: $DB_PATH"

mkdir -p -- "$BACKUP_DIR"
chmod 0700 -- "$BACKUP_DIR"

exec 9>"$LOCK_FILE"
flock -n 9 || die "another backup is already running"

STAGE_DIR="$(mktemp -d "${BACKUP_DIR}/.forge-backup.XXXXXX")"
mkdir -p -- "${STAGE_DIR}/data"

if systemctl is-active --quiet "$SERVICE"; then
    SERVICE_NEEDS_RESTART=1
    log "stopping ${SERVICE}.service"
    systemctl stop "$SERVICE" ||
        die "failed to stop ${SERVICE}.service"
fi

log "checkpointing SQLite WAL"
sqlite3 -batch "$DB_PATH" 'PRAGMA wal_checkpoint(TRUNCATE);' >/dev/null

log "checking SQLite integrity"
integrity="$(sqlite3 -batch "$DB_PATH" 'PRAGMA integrity_check;')"
[[ "$integrity" == "ok" ]] ||
    die "SQLite integrity_check failed: ${integrity}"

log "copying database"
for suffix in '' '-wal' '-shm'; do
    source="${DB_PATH}${suffix}"
    if [[ -e "$source" ]]; then
        cp -a -- "$source" "${STAGE_DIR}/data/"
    fi
done

if [[ -d "$TESTDATA_DIR" ]]; then
    log "copying testdata"
    cp -a -- "$TESTDATA_DIR" "${STAGE_DIR}/testdata"
else
    log "testdata directory not present; recording an empty testdata tree"
    mkdir -p -- "${STAGE_DIR}/testdata"
fi

created_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
{
    printf 'format=forge-cold-backup-v1\n'
    printf 'created_at=%s\n' "$created_at"
    printf 'service=%s\n' "$SERVICE"
    printf 'data_dir=%s\n' "$DATA_DIR"
    printf 'database=%s\n' "$DB_NAME"
    printf 'integrity_check=%s\n' "$integrity"
} >"${STAGE_DIR}/manifest.txt"

log "writing checksums"
(
    cd -- "$STAGE_DIR"
    find . -type f ! -name SHA256SUMS -print0 |
        LC_ALL=C sort -z |
        xargs -0 -r sha256sum >SHA256SUMS
)

archive="${BACKUP_DIR}/forge-backup-$(date -u +'%Y%m%dT%H%M%SZ').tar.gz"
tar -C "$STAGE_DIR" -czf "$archive" .
sha256sum "$archive" >"${archive}.sha256"
chmod 0600 -- "$archive" "${archive}.sha256"

log "backup created: $archive"
log "checksum file: ${archive}.sha256"
