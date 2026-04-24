#!/bin/bash
# Claude Code Bedrock コスト制限 - 手動インストールスクリプト
# プラグインシステムが使えない場合のフォールバック

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HOOK_SCRIPT="$SCRIPT_DIR/hooks/bedrock_limiter.sh"
SETTINGS="$HOME/.claude/settings.json"

echo "=== Claude Code Bedrock Limiter インストール ==="

chmod +x "$HOOK_SCRIPT"
echo "✅ $HOOK_SCRIPT に実行権限を付与しました"

if [ ! -f "$SETTINGS" ]; then
  echo "{}" > "$SETTINGS"
fi

cp "$SETTINGS" "$SETTINGS.bak"
echo "✅ $SETTINGS をバックアップしました ($SETTINGS.bak)"

# settings.json に hook を追加（Python / Node が不要な純粋シェル実装）
HOOK_JSON=$(cat <<EOF
{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"$HOOK_SCRIPT"}]}]}}
EOF
)

# jq があれば使い、なければ python3 / node でマージ
if command -v jq &>/dev/null; then
  jq --argjson hook "$HOOK_JSON" '. * $hook' "$SETTINGS" > "$SETTINGS.tmp" && mv "$SETTINGS.tmp" "$SETTINGS"
elif command -v python3 &>/dev/null; then
  python3 -c "
import json, sys
s = json.load(open('$SETTINGS'))
s.setdefault('hooks', {})['UserPromptSubmit'] = [{'hooks': [{'type': 'command', 'command': '$HOOK_SCRIPT'}]}]
json.dump(s, open('$SETTINGS', 'w'), indent=2)
"
elif command -v node &>/dev/null; then
  node -e "
const fs = require('fs');
const s = JSON.parse(fs.readFileSync('$SETTINGS', 'utf8'));
(s.hooks = s.hooks || {}).UserPromptSubmit = [{hooks:[{type:'command',command:'$HOOK_SCRIPT'}]}];
fs.writeFileSync('$SETTINGS', JSON.stringify(s, null, 2));
"
else
  echo "⚠️  jq / python3 / node のいずれも見つかりません。"
  echo "    $SETTINGS を手動で編集して UserPromptSubmit hook を追加してください。"
  echo "    コマンド: $HOOK_SCRIPT"
  exit 1
fi

echo "✅ ~/.claude/settings.json に UserPromptSubmit hook を登録しました"
echo ""
echo "=== インストール完了 ==="
echo ""
echo "設定ファイル: $SCRIPT_DIR/config.json (初回起動時に自動生成)"
echo "コスト確認:   $HOOK_SCRIPT --status"
echo "アンインストール: bash $SCRIPT_DIR/uninstall.sh"
