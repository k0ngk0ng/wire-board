FROM golang:1.26-alpine AS go-toolchain

FROM node:22-alpine AS frontend
COPY --from=go-toolchain /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
ARG SOURCE_SHA
ARG WIRE_BOARD_ASSET_BASE
ARG PUBLIC_ORIGIN
ARG CDN_ORIGIN
ARG OSS_BUCKET
ARG OSS_ENDPOINT
ENV SOURCE_SHA=${SOURCE_SHA} GITHUB_SHA=${SOURCE_SHA} WIRE_BOARD_ASSET_BASE=${WIRE_BOARD_ASSET_BASE} PUBLIC_ORIGIN=${PUBLIC_ORIGIN} CDN_ORIGIN=${CDN_ORIGIN} OSS_BUCKET=${OSS_BUCKET} OSS_ENDPOINT=${OSS_ENDPOINT}
RUN node --input-type=module -e 'import assert from "node:assert/strict"; assert.match(process.env.SOURCE_SHA, /^[a-f0-9]{40}$/, "SOURCE_SHA must be the published source commit"); assert.equal(process.env.WIRE_BOARD_ASSET_BASE, process.env.CDN_ORIGIN + "/bga/", "WIRE_BOARD_ASSET_BASE must use the configured stable bga root");'
RUN apk add --no-cache python3 xz ca-certificates
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY games/ /src/games/
COPY scripts/ /src/scripts/
COPY internal/ /src/internal/
COPY go.mod go.sum /src/
COPY assets/ /src/assets/
RUN python3 -B /src/scripts/resource_store.py hydrate
RUN npm run build
RUN cd /src && python3 -B -c 'import sys; from pathlib import Path; sys.path.insert(0,"scripts"); from asset_reuse import prepare; prepare(Path("web/dist"),{"version":1,"files":{}})'

FROM go-toolchain AS backend
RUN apk add --no-cache python3 xz ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist ./web/dist
COPY --from=frontend /src/games/ ./games/
COPY --from=frontend /src/assets/ ./assets/
ARG PUBLIC_ORIGIN
ARG CDN_ORIGIN
ARG OSS_BUCKET
ARG OSS_ENDPOINT
ENV PUBLIC_ORIGIN=${PUBLIC_ORIGIN} CDN_ORIGIN=${CDN_ORIGIN} OSS_BUCKET=${OSS_BUCKET} OSS_ENDPOINT=${OSS_ENDPOINT}
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN python3 -B games/pal/build/prepare-server.py --out .local/pal/container-runtime/pal \
    && python3 -B games/contra/build/ci_runtime.py --dist web/dist --out .local/contra-ci-runtime \
    && python3 -B scripts/cdn_delivery.py verify --report .local/cdn-delivery/verified.json \
    && python3 -B scripts/cdn_delivery.py trim --report .local/cdn-delivery/verified.json \
    && python3 -B games/contra/build/native.py --runtime .local/contra-ci-runtime/manifest.json --out .local/contra-container/wire-board --goos "$TARGETOS" --goarch "$TARGETARCH" \
    && cp .local/contra-container/wire-board /wire-board

FROM alpine:3.23 AS runtime-base
RUN apk add --no-cache sqlite && addgroup -g 10001 board && adduser -D -u 10001 -G board board && mkdir /data && chown board:board /data

FROM runtime-base AS runtime
LABEL io.github.k0ngk0ng.wire-board.storage-generation="8"
COPY --from=backend /wire-board /usr/local/bin/wire-board
COPY --from=backend /src/.local/pal/container-runtime/pal /usr/local/bin/pal
COPY --from=backend /src/games/pal/native/upstream/LICENSE /usr/local/share/licenses/wire-board/LICENSE.SDLPAL
COPY --from=backend /src/docs/THIRD_PARTY.md /usr/local/share/licenses/wire-board/THIRD_PARTY.md
ENV DATA_DIR=/data ADDR=:8080
USER 10001:10001
EXPOSE 8080
VOLUME /data
HEALTHCHECK --interval=2s --timeout=2s --start-period=2s --retries=15 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/wire-board"]
