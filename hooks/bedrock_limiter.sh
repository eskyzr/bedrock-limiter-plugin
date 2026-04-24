#!/bin/sh
# スクリプト自身の場所からプラグインルートを解決する
# hooks/bedrock_limiter.sh → ../  → plugin root
PLUGIN_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
esac
exec "${PLUGIN_ROOT}/go/bin/bedrock-limiter-${OS}-${ARCH}" "$@"
