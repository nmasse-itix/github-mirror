#!/bin/sh

set -xe
test -n "$1"

GOOS=darwin GOARCH=amd64 go build -o github-mirror-darwin-amd64-$1
GOOS=linux GOARCH=amd64 go build -o github-mirror-linux-amd64-$1
GOOS=linux GOARCH=arm64 go build -o github-mirror-linux-arm64-$1
GOOS=linux GOARCH=arm go build -o github-mirror-linux-arm-$1
shasum -a 256 github-mirror-* > SHA256SUMS
