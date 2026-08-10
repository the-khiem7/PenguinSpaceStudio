#!/usr/bin/env sh
set -eu

cd /workspace
bun install --cwd frontend --frozen-lockfile
wails3 build GOOS=windows GOARCH=amd64
test -f out/penguinspace.exe
