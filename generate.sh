#!/usr/bin/env bash
set -euo pipefail
sdk_root=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
if [[ "$(protoc-gen-go --version)" != 'protoc-gen-go v1.36.11' ]] ||
   [[ "$(protoc-gen-go-grpc --version)" != 'protoc-gen-go-grpc 1.5.1' ]]; then
  echo 'Use protoc-gen-go v1.36.11 and protoc-gen-go-grpc v1.5.1; see the SDK README.' >&2
  exit 1
fi
output_root="$sdk_root"
if [[ "${1:-}" == '--check' ]]; then
  output_root=$(mktemp -d)
  trap 'rm -rf "$output_root"' EXIT
elif [[ $# -ne 0 ]]; then
  echo 'Usage: generate.sh [--check]' >&2
  exit 1
fi
protoc -I "$sdk_root" --go_out="$output_root" --go_opt=paths=source_relative \
  --go-grpc_out="$output_root" --go-grpc_opt=paths=source_relative \
  proto/v1/extension.proto
if [[ "${1:-}" == '--check' ]]; then
  for generated in extension.pb.go extension_grpc.pb.go; do
    # A different protoc patch version changes its informational header only.
    diff -u <(sed '/^\/\/.*protoc .*v[0-9]/d' "$sdk_root/proto/v1/$generated") \
      <(sed '/^\/\/.*protoc .*v[0-9]/d' "$output_root/proto/v1/$generated")
  done
fi
