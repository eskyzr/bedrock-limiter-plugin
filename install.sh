#!/bin/bash
# Claude Code Bedrock コスト制限 - 手動インストールスクリプト
# プラグインシステムが使えない場合のフォールバック

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HOOK_SCRIPT="$SCRIPT_DIR/hooks/bedrock_limiter.js"
SETTINGS="$HOME/.claude/settings.json"

echo "=== Claude Code Bedrock Limiter インストール ==="

# Node.js 確認（Claude Code が動いている環境には必ず存在する）
if ! command -v node &>/dev/null; then
  echo "❌ Node.js が見つかりません。"
  exit 1
fi

# settings.json バックアップ & hook 追加
if [ ! -f "$SETTINGS" ]; then
  echo "{}" > "$SETTINGS"
fi

cp "$SETTINGS" "$SETTINGS.bak"
echo "✅ $SETTINGS をバックアップしました ($SETTINGS.bak)"

node - <<JSEOF
const fs = require('fs');
const settingsPath = "$SETTINGS";
const settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8') || '{}');

settings.hooks = settings.hooks || {};
settings.hooks.UserPromptSubmit = [
  { hooks: [{ type: 'command', command: 'node $HOOK_SCRIPT' }] }
];

fs.writeFileSync(settingsPath, JSON.stringify(settings, null, 2));
JSEOF

echo "✅ ~/.claude/settings.json に UserPromptSubmit hook を登録しました"
echo ""
echo "=== インストール完了 ==="
echo ""
echo "設定ファイル: $SCRIPT_DIR/config.json (初回起動時に自動生成)"
echo "コスト確認:   node $HOOK_SCRIPT --status"
echo "アンインストール: bash $SCRIPT_DIR/uninstall.sh"
