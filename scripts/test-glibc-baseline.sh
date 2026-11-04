#!/usr/bin/env bash
# Run the CGO-free application and embedded library against real glibc 2.17.
set -euo pipefail
repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
mkdir -p "$repo_dir/.build"
cd "$repo_dir/tests/integration"
CGO_ENABLED=0 go test -c -o "$repo_dir/.build/glibc-baseline.test"
container_engine="${CONTAINER_ENGINE:-docker}"
"$container_engine" run --rm --network none --tmpfs /tmp:exec,mode=1777 \
  -v "$repo_dir:/src:ro" \
  quay.io/centos/centos@sha256:e4ca2ed0202e76be184e75fb26d14bf974193579039d5573fb2348664deef76e \
  sh -ec 'getconf GNU_LIBC_VERSION; test "$(getconf GNU_LIBC_VERSION)" = "glibc 2.17"; /src/.build/glibc-baseline.test -test.v'
