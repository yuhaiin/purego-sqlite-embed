#!/usr/bin/env bash
# Debian/Ubuntu: gcc-mipsel-linux-gnu libc6-dev-mipsel-cross qemu-user.
set -euo pipefail
repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"
export GOOS=linux GOARCH=mipsle GOMIPS=hardfloat CGO_ENABLED=1
export CC=mipsel-linux-gnu-gcc
export QEMU_LD_PREFIX=/usr/mipsel-linux-gnu
mkdir -p .build/tests
go test -c -o .build/tests/embed-mipsle.test .
SQLITE_EMBED_REQUIRE=1 qemu-mipsel .build/tests/embed-mipsle.test -test.v -test.timeout=3m
cd tests/integration
go test -c -o ../../.build/tests/integration-mipsle.test .
qemu-mipsel ../../.build/tests/integration-mipsle.test -test.v -test.timeout=3m
