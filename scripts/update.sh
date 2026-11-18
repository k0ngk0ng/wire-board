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
storage_marker=.storage-generation
storage_tmp=
saved_generation=0
minimum_generation=0
old_image=
old_generation=0
backup_tmp=
backup_partial=
switching=false
committed=false
compose() { docker compose --project-name wire-board --env-file .env --env-file "$candidate" -f compose.yaml "$@"; }
valid_generation() { [[ $1 =~ ^(0|[1-9][0-9]{0,8})$ ]]; }
image_generation() {
    local generation
    generation=$(docker image inspect --format '{{with .Config.Labels}}{{with index . "io.github.k0ngk0ng.wire-board.storage-generation"}}{{.}}{{end}}{{end}}' "$1") || return 1
    generation=${generation:-0}
    valid_generation "$generation" || return 1
    printf '%s\n' "$generation"
}
finish() {
    rc=$?
    trap - EXIT INT TERM
    if [[ $switching == true && $committed == false && -n $old_image ]]; then
        if (( old_generation < minimum_generation )); then
            echo "Update failed; the previous image cannot read storage generation $minimum_generation. Database and pre-update backup are retained." >&2
            if failed_container=$(compose ps --all --quiet wire-board) && [[ -n $failed_container ]] &&
                failed_image=$(docker inspect --format '{{.Image}}' "$failed_container") && [[ $failed_image != "$old_image" ]]; then
                if ! compose stop wire-board; then
                    echo "Could not stop the failed container; inspect docker compose logs." >&2
                fi
            fi
        else
            echo "Update failed; restarting the previous image. Data backup is retained." >&2
            previous_version=$(sed -n 's/^WIRE_BOARD_VERSION=//p' .image.env 2>/dev/null || true)
            printf 'WIRE_BOARD_IMAGE=%s\nWIRE_BOARD_VERSION=%s\n' "$old_image" "${previous_version:-rollback}" > "$candidate"
            if ! compose up -d --no-build --pull never --wait --wait-timeout 90; then
                echo "Rollback failed; inspect docker compose logs and backups/." >&2
            fi
        fi
    fi
    rm -f -- "$candidate"
    [[ -z $storage_tmp ]] || rm -f -- "$storage_tmp"
    [[ -z $backup_partial ]] || rm -f -- "$backup_partial"
    [[ -z $backup_tmp ]] || rm -rf -- "$backup_tmp"
    exit "$rc"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ -e $storage_marker || -L $storage_marker ]]; then
    [[ -f $storage_marker && ! -L $storage_marker ]] || die "Invalid storage generation record"
    saved_generation=$(<"$storage_marker")
    valid_generation "$saved_generation" || die "Invalid storage generation record"
    minimum_generation=$saved_generation
fi
echo "Pulling $repo:$tag from GitHub Container Registry..."
docker pull "$repo:$tag"
digest=$(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$repo:$tag" | awk -v prefix="$repo@sha256:" 'index($0,prefix)==1 {print; exit}')
[[ $digest =~ ^ghcr\.io/k0ngk0ng/wire-board@sha256:[a-f0-9]{64}$ ]] || die "Cannot resolve published image digest"
target_generation=$(image_generation "$digest") || die "Invalid storage generation on target image"
printf 'WIRE_BOARD_IMAGE=%s\nWIRE_BOARD_VERSION=%s\n' "$digest" "$tag" > "$candidate"
# Reject invalid Compose/environment settings before touching the running service.
compose config --quiet
container=$(compose ps --all --quiet wire-board)
if [[ -n $container ]]; then
    old_image=$(docker inspect --format '{{.Image}}' "$container")
    old_generation=$(image_generation "$old_image") || die "Invalid storage generation on current image"
    if (( old_generation > minimum_generation )); then minimum_generation=$old_generation; fi
fi
if (( target_generation < minimum_generation )); then
    die "Storage generation $target_generation cannot read the database generation $minimum_generation"
fi
if [[ -n $container ]]; then
    source_dir=$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' "$container")
    [[ -n $source_dir && -d $source_dir ]] || die "Cannot locate existing data volume"
    container_status=$(docker inspect --format '{{.State.Status}}' "$container")
    backup_mode=online
    if [[ $container_status == exited || $container_status == created ]]; then
        backup_mode=stopped
    fi
    mkdir -p backups
    backup="backups/data-$(date -u +%Y%m%dT%H%M%SZ)-$$.tar.gz"
    backup_partial="$backup.partial"
    backup_tmp=$(mktemp -d .update.backup.XXXXXX)
    echo "Taking an online SQLite backup while the current game remains available..."
    # Use the already pulled GitHub image as an isolated SQLite client. Never
    # launch a second game server against the live volume (rooms live in memory).
    # SQLite's backup API reads committed WAL pages into a standalone snapshot.
    if ! docker run --rm --network none --read-only --user 0:0 \
        --cap-drop ALL --cap-add DAC_READ_SEARCH --security-opt no-new-privileges:true \
        --tmpfs /tmp:rw,noexec,nosuid,size=32m \
        --mount "type=bind,source=$source_dir,target=/source,readonly" \
        --mount "type=bind,source=$PWD/$backup_tmp,target=/snapshot" \
        --entrypoint /bin/sh "$digest" -ec '
            source_db=/source/wire-board.db
            if [ "$1" = stopped ]; then
                mkdir /snapshot/stopped-source
                cp "$source_db" /snapshot/stopped-source/wire-board.db
                if [ -f "$source_db-wal" ]; then
                    cp "$source_db-wal" /snapshot/stopped-source/wire-board.db-wal
                fi
                source_db=/snapshot/stopped-source/wire-board.db
            fi
            sqlite3 -readonly "$source_db" ".timeout 5000" ".backup /snapshot/wire-board.db"
            sqlite3 /snapshot/wire-board.db "PRAGMA journal_mode=DELETE;" >/dev/null
            test "$(sqlite3 -readonly /snapshot/wire-board.db "PRAGMA quick_check;")" = ok
        ' backup "$backup_mode"; then
        die "Online backup failed (the selected image must include sqlite3); current service was not stopped"
    fi
    tar -C "$backup_tmp" -czf "$backup_partial" ./wire-board.db
    mv -f -- "$backup_partial" "$backup"
    backup_partial=
    rm -rf -- "$backup_tmp"
    backup_tmp=
    echo "Consistent pre-update database snapshot: $backup"
fi

# Download, validation and compression are finished. Compose now stops the sole
# writer and recreates it on the same data volume; no offline backup work remains.
echo "Switching containers; clients will reconnect automatically..."
if (( target_generation > saved_generation )); then
    storage_tmp=$(mktemp .update.storage.XXXXXX)
    printf '%s\n' "$target_generation" > "$storage_tmp"
    mv -f -- "$storage_tmp" "$storage_marker"
    storage_tmp=
fi
minimum_generation=$target_generation
switching=true
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
