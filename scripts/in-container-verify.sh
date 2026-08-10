#!/usr/bin/env sh
set -eu

cd /workspace
bun --version
go version
wails3 version

bun install --cwd frontend --frozen-lockfile
GOOS=windows CGO_ENABLED=0 wails3 generate bindings -clean=true -ts -i
bun run --cwd frontend typecheck
bun run --cwd frontend build

test -z "$(gofmt -l .)"
go vet ./internal/...
go test ./internal/...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags production -trimpath -buildvcs=false -o /tmp/penguinspace.exe .
