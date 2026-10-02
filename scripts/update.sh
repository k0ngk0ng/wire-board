#!/usr/bin/env bash
# Install beside compose.yaml and .env. Requires Linux, Docker Compose v2+, root.
set -Eeuo pipefail
umask 077

die() { echo "Error: $*" >&2; exit 1; }
[[ $# -le 1 ]] || die "Usage: $0 [latest|vX.Y.Z|sha-TAG]"
tag=${1:-latest}
[[ $tag =~ ^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$ ]] || die "Invalid image tag"
cd -- "$(dirname -- "$(readlink -f -- "$0")")"
[[ -f compose.yaml && -f .env ]] || die "Install update.sh beside compose.yaml and .env"
[[ $(id -u) -eq 0 ]] || die "Run as root (required for consistent data backup)"
command -v flock >/dev/null || die "flock is required"
docker compose version >/dev/null
exec 9>.update.lock
flock -n 9 || die "Another update is running"

# Ignore shell overrides: runtime settings belong to the private .env file.
unset WIRE_BOARD_IMAGE COMPOSE_FILE COMPOSE_PROJECT_NAME COMPOSE_PROFILES
unset BIND_ADDRESS PORT
repo=ghcr.io/k0ngk0ng/wire-board
candidate=$(mktemp .update.XXXXXX)
old_image=
stopped=false
committed=false
compose() { docker compose --project-name wire-board --env-file .env --env-file "$candidate" -f compose.yaml "$@"; }
finish() {
    rc=$?
    trap - EXIT INT TERM
    if [[ $stopped == true && $committed == false && -n $old_image ]]; then
        echo "Update failed; restarting the previous image. Data backup is retained." >&2
        printf 'WIRE_BOARD_IMAGE=%s\n' "$old_image" > "$candidate"
        if ! compose up -d --no-build --pull never --wait --wait-timeout 90; then
            echo "Rollback failed; inspect docker compose logs and backups/." >&2
        fi
    fi
    rm -f -- "$candidate"
    exit "$rc"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo "Pulling $repo:$tag from GitHub Container Registry..."
docker pull "$repo:$tag"
digest=$(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$repo:$tag" | awk -v prefix="$repo@sha256:" 'index($0,prefix)==1 {print; exit}')
[[ $digest =~ ^ghcr\.io/k0ngk0ng/wire-board@sha256:[a-f0-9]{64}$ ]] || die "Cannot resolve published image digest"
printf 'WIRE_BOARD_IMAGE=%s\n' "$digest" > "$candidate"
container=$(compose ps --all --quiet wire-board)
if [[ -n $container ]]; then
    old_image=$(docker inspect --format '{{.Image}}' "$container")
    source_dir=$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' "$container")
    [[ -n $source_dir && -d $source_dir ]] || die "Cannot locate existing data volume"
    mkdir -p backups
    backup="backups/data-$(date -u +%Y%m%dT%H%M%SZ)-$$.tar.gz"
    stopped=true
    compose stop wire-board
    tar -C "$source_dir" -czf "$backup" .
    echo "Consistent data backup: $backup"
fi

compose up -d --no-build --pull never --wait --wait-timeout 90
# The image health check probes the application's /healthz endpoint.
container=$(compose ps --quiet wire-board)
[[ -n $container ]] || die "Container is missing"
[[ $(docker inspect --format '{{.State.Health.Status}}' "$container") == healthy ]] || die "Application is not healthy"
mv -f -- "$candidate" .image.env
committed=true
echo "Running $digest"
echo "Updates preserve .env and the data volume. Backups are never automatically deleted."
