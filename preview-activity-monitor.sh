#!/usr/bin/env bash
set -euo pipefail

# Manage an isolated preview stack. It never touches the production compose
# project (sub2api-prod), production containers, networks, or volumes.
#
# Usage:
#   ./preview-activity-monitor.sh up      # default: build + start
#   ./preview-activity-monitor.sh status
#   ./preview-activity-monitor.sh logs
#   ./preview-activity-monitor.sh down    # remove preview containers + volumes

ACTION="${1:-up}"
WORKTREE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_REPO_DIR="$(cd "$(dirname "$(git -C "$WORKTREE_DIR" rev-parse --git-common-dir)")" && pwd)"

if [[ -n "${PREVIEW_DB_DUMP:-}" ]]; then
  DUMP_FILE="$PREVIEW_DB_DUMP"
elif compgen -G "$MAIN_REPO_DIR/backups/*.sql.gz" >/dev/null; then
  DUMP_FILE="$(ls -1t "$MAIN_REPO_DIR"/backups/*.sql.gz | head -n 1)"
else
  echo "No preview dump found. Set PREVIEW_DB_DUMP=/absolute/path/to/backup.sql.gz" >&2
  exit 1
fi

if [[ ! -f "$DUMP_FILE" ]]; then
  echo "Preview dump not found: $DUMP_FILE" >&2
  exit 1
fi

export PREVIEW_DB_DUMP="$DUMP_FILE"
export PREVIEW_PORT="${PREVIEW_PORT:-18080}"

compose() {
  docker compose \
    -p sub2api-activity-preview \
    -f "$WORKTREE_DIR/docker-compose.activity-preview.yml" \
    "$@"
}

echo "Worktree:  $WORKTREE_DIR"
echo "Dump:      $DUMP_FILE"
echo "Preview:   http://127.0.0.1:${PREVIEW_PORT}/admin/activity-monitor"
echo "Prod note: only project sub2api-activity-preview is targeted; sub2api-prod is not referenced."
echo

cd "$WORKTREE_DIR"

case "$ACTION" in
  up|start)
    compose up -d --build
    echo
    compose ps
    echo
    echo "Follow API logs:  ./preview-activity-monitor.sh logs"
    echo "Stop and clean:   ./preview-activity-monitor.sh down"
    ;;
  status|ps)
    compose ps
    ;;
  logs)
    docker logs -f sub2api-activity-preview
    ;;
  down|stop)
    compose down -v
    ;;
  *)
    echo "Usage: $0 {up|status|logs|down}" >&2
    exit 1
    ;;
esac
