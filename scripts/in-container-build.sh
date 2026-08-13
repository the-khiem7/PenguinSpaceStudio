#!/usr/bin/env sh
set -eu

: "${VERSION:?VERSION is required (for example, VERSION=0.1.0)}"
case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "VERSION must be a semantic version without a leading v (for example, 0.1.0): $VERSION" >&2; exit 1 ;;
esac

cd /workspace
artifact="out/penguinspace-v${VERSION}.exe"
if [ -e "$artifact" ]; then
  echo "Refusing to overwrite existing artifact: $artifact" >&2
  exit 1
fi

bun install --cwd frontend --frozen-lockfile
wails3 build GOOS=windows GOARCH=amd64
test -f "$artifact"
printf 'Built Windows artifact: %s\n' "$artifact"
