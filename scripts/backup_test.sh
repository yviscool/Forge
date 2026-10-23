#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ROOT="$(mktemp -d)"
trap 'rm -rf -- "$TEST_ROOT"' EXIT

DATA_DIR="${TEST_ROOT}/data"
BACKUP_DIR="${TEST_ROOT}/backups"
STATE_DIR="${TEST_ROOT}/state"
BIN_DIR="${TEST_ROOT}/bin"

mkdir -p "$DATA_DIR/testdata/contest-a" "$BACKUP_DIR" "$STATE_DIR" "$BIN_DIR"
printf 'sqlite-test-database\n' >"${DATA_DIR}/forge.db"
printf 'wal\n' >"${DATA_DIR}/forge.db-wal"
printf 'shm\n' >"${DATA_DIR}/forge.db-shm"
printf '1\n2\n' >"${DATA_DIR}/testdata/contest-a/answer.txt"
touch "${STATE_DIR}/active"

cat >"${BIN_DIR}/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
state="${SYSTEMCTL_STATE:?}"
case "${1:?}" in
    is-active)
        [[ -f "${state}/active" ]]
        ;;
    stop)
        rm -f "${state}/active"
        printf 'stop\n' >>"${state}/calls"
        ;;
    start)
        touch "${state}/active"
        printf 'start\n' >>"${state}/calls"
        ;;
    *)
        exit 2
        ;;
esac
EOF

cat >"${BIN_DIR}/flock" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

cat >"${BIN_DIR}/sqlite3" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-batch" ]]; then
    shift
fi
[[ $# -ge 2 ]] || exit 2
query="${*: -1}"
if [[ "$query" == "PRAGMA integrity_check;" ]]; then
    if [[ "${SQLITE_INTEGRITY_RESULT:-ok}" == "ok" ]]; then
        printf 'ok\n'
    else
        printf '%s\n' "$SQLITE_INTEGRITY_RESULT"
    fi
fi
EOF

chmod +x "${BIN_DIR}/systemctl" "${BIN_DIR}/flock" "${BIN_DIR}/sqlite3"

run_backup() {
    PATH="${BIN_DIR}:${PATH}" \
        SYSTEMCTL_STATE="$STATE_DIR" \
        FORGE_DATA_DIR="$DATA_DIR" \
        FORGE_BACKUP_DIR="$BACKUP_DIR" \
        FORGE_SERVICE=forge-test \
        bash "${ROOT}/scripts/backup.sh"
}

run_backup >/dev/null

[[ -f "${STATE_DIR}/active" ]]
[[ "$(cat "${STATE_DIR}/calls")" == $'stop\nstart' ]]

archive="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'forge-backup-*.tar.gz' -print -quit)"
[[ -n "$archive" && -f "$archive" ]]
[[ -f "${archive}.sha256" ]]
(
    cd "$BACKUP_DIR"
    sha256sum -c "$(basename "${archive}.sha256")"
)

extract_dir="${TEST_ROOT}/extract"
mkdir -p "$extract_dir"
tar -xzf "$archive" -C "$extract_dir"
[[ -f "${extract_dir}/data/forge.db" ]]
[[ -f "${extract_dir}/data/forge.db-wal" ]]
[[ -f "${extract_dir}/data/forge.db-shm" ]]
[[ -f "${extract_dir}/testdata/contest-a/answer.txt" ]]
[[ -f "${extract_dir}/manifest.txt" ]]
(
    cd "$extract_dir"
    sha256sum -c SHA256SUMS
)

before="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'forge-backup-*.tar.gz' | wc -l)"
if SQLITE_INTEGRITY_RESULT='database disk image is malformed' run_backup >/dev/null 2>&1; then
    printf 'backup unexpectedly succeeded with a corrupt database\n' >&2
    exit 1
fi
after="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'forge-backup-*.tar.gz' | wc -l)"
[[ "$before" == "$after" ]]
[[ -f "${STATE_DIR}/active" ]]

printf 'backup script tests passed\n'
