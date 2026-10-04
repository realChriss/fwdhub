#!/bin/sh
set -eu
cd "$(dirname "$0")"
version=${1:-dev}

trap 'rm -f src/rsrc_windows_*.syso' EXIT
go run scripts/winicon.go

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  ext=""
  if [ "$os" = windows ]; then ext=.exe; fi
  echo "building $os/$arch"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w -X main.version=$version" -o "dist/fwdhub-$os-$arch$ext" ./src
done
