#!/bin/sh
# OS/アーキテクチャを検出して対応するバイナリを実行する
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
esac
exec "${CLAUDE_PLUGIN_ROOT}/go/bin/bedrock-limiter-${OS}-${ARCH}" "$@"
