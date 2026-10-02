#!/usr/bin/env bash
# Install beside compose.yaml and .env. Requires Linux, Docker Compose v2+, root.
set -Eeuo pipefail
umask 077

die() { echo "Error: $*" >&2; exit 1; }
[[ $# -le 1 ]] || die "Usage: $0 [latest|vX.Y.Z|sha-TAG|clean]"
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
if [[ $tag == clean ]]; then
    # No daemon-wide prune: other applications, containers and volumes are untouched.
    protected=$(docker ps -aq | xargs -r docker inspect --format '{{.Image}}')
    if [[ -f .previous-image ]]; then
        protected+=$'\n'$(cat .previous-image)
    fi
    if [[ -f .image.env ]]; then
        current_ref=$(sed -n 's/^WIRE_BOARD_IMAGE=//p' .image.env)
        current_id=$(docker image inspect --format '{{.Id}}' "$current_ref" 2>/dev/null || true)
        protected+=$'\n'"$current_id"
    fi
    candidates=$({ docker image ls "$repo" --quiet --no-trunc; docker image ls --filter 'label=org.opencontainers.image.source=https://github.com/k0ngk0ng/wire-board' --quiet --no-trunc; } | sort -u)
    for image_id in $candidates; do
        if grep -Fxq -- "$image_id" <<< "$protected"; then
            echo "Keeping in-use or rollback image $image_id"
            continue
        fi
        refs=$(docker image inspect --format '{{range .RepoTags}}{{println .}}{{end}}' "$image_id")
        digests=$(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$image_id")
        safe=true
        while IFS= read -r ref; do
            [[ -z $ref || $ref == "$repo"@* ]] || safe=false
        done <<< "$digests"
        while IFS= read -r ref; do
            [[ -z $ref || $ref == "$repo":* ]] || safe=false
        done <<< "$refs"
        [[ $safe == true ]] || continue
        if [[ -n $refs ]]; then
            while IFS= read -r ref; do
                [[ -z $ref ]] || docker image rm "$ref"
            done <<< "$refs"
        else
            docker image rm "$image_id"
        fi
    done
    echo "Wire Board image cleanup complete. Current/previous images, containers, volumes and backups retained."
    exit 0
fi
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
new_image=$(docker inspect --format '{{.Image}}' "$container")
if [[ -n $old_image && $old_image != "$new_image" ]]; then
    printf '%s\n' "$old_image" > .previous-image
fi
mv -f -- "$candidate" .image.env
committed=true
echo "Running $digest"
echo "Updates preserve .env and the data volume. Backups are never automatically deleted."
