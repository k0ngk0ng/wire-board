FROM golang:1.26-alpine AS go-toolchain

FROM node:22-alpine AS frontend
COPY --from=go-toolchain /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
# Source builds reuse the same already-published commit assets as CI.
# Missing or mismatched arguments fail before any build can embed local assets.
ARG SOURCE_SHA
ARG WIRE_BOARD_ASSET_BASE
ENV SOURCE_SHA=${SOURCE_SHA} WIRE_BOARD_ASSET_BASE=${WIRE_BOARD_ASSET_BASE}
RUN node --input-type=module -e 'import assert from "node:assert/strict"; assert.match(process.env.SOURCE_SHA, /^[a-f0-9]{40}$/, "SOURCE_SHA must be the published source commit"); assert.equal(process.env.WIRE_BOARD_ASSET_BASE, "https://cdn.ichenj.com/bga/web/" + process.env.SOURCE_SHA + "/", "WIRE_BOARD_ASSET_BASE must match SOURCE_SHA");'
RUN apk add --no-cache python3 xz ca-certificates
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY games/ /src/games/
COPY scripts/ /src/scripts/
COPY internal/ /src/internal/
COPY go.mod go.sum /src/
RUN npm run build

FROM go-toolchain AS backend
RUN apk add --no-cache python3 xz ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist ./web/dist
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN python3 -B games/contra/build/ci_runtime.py --dist web/dist --out .local/contra-ci-runtime \
    && python3 -B scripts/cdn_delivery.py verify --report .local/cdn-delivery/verified.json \
    && python3 -B scripts/cdn_delivery.py trim --report .local/cdn-delivery/verified.json \
    && python3 -B games/contra/build/native.py --runtime .local/contra-ci-runtime/manifest.json --out .local/contra-container/wire-board --goos "$TARGETOS" --goarch "$TARGETARCH"

FROM alpine:3.23
RUN apk add --no-cache sqlite && addgroup -g 10001 board && adduser -D -u 10001 -G board board && mkdir /data && chown board:board /data
COPY --from=backend /src/.local/contra-container/wire-board /usr/local/bin/wire-board
ENV DATA_DIR=/data ADDR=:8080
USER 10001:10001
EXPOSE 8080
VOLUME /data
HEALTHCHECK --interval=2s --timeout=2s --start-period=2s --retries=15 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/wire-board"]
