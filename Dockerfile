# syntax=docker/dockerfile:1

FROM oven/bun:1.3.14@sha256:e10577f0db68676a7024391c6e5cb4b879ebd17188ab750cf10024a6d700e5c4 AS bun

FROM golang:1.25-bookworm@sha256:908f8ff2ec296df2f349563072c7925775cd28b50361a52ed834a8a37399b9bf AS toolchain

COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun

ENV PATH="/usr/local/go/bin:/usr/local/bin:/go/bin:${PATH}" \
    GOBIN="/usr/local/bin" \
    GOMODCACHE="/go/pkg/mod" \
    GOCACHE="/root/.cache/go-build" \
    BUN_INSTALL_CACHE_DIR="/root/.bun/install/cache"

RUN apt-get update \
    && apt-get install --no-install-recommends -y ca-certificates git \
    && rm -rf /var/lib/apt/lists/* \
    && CGO_ENABLED=0 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.6 \
    && bun --version \
    && go version \
    && wails3 version

WORKDIR /workspace
