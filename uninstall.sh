#!/bin/bash
# hook 設定を settings.json から除去する

SETTINGS="$HOME/.claude/settings.json"

if [ ! -f "$SETTINGS" ]; then
  echo "settings.json が見つかりません"
  exit 0
fi

if command -v jq &>/dev/null; then
  jq 'del(.hooks.UserPromptSubmit) | if .hooks == {} then del(.hooks) else . end' \
    "$SETTINGS" > "$SETTINGS.tmp" && mv "$SETTINGS.tmp" "$SETTINGS"
elif command -v python3 &>/dev/null; then
  python3 -c "
import json
s = json.load(open('$SETTINGS'))
h = s.get('hooks', {})
h.pop('UserPromptSubmit', None)
if not h: s.pop('hooks', None)
json.dump(s, open('$SETTINGS', 'w'), indent=2)
"
elif command -v node &>/dev/null; then
  node -e "
const fs = require('fs');
const s = JSON.parse(fs.readFileSync('$SETTINGS', 'utf8'));
delete (s.hooks || {}).UserPromptSubmit;
if (s.hooks && !Object.keys(s.hooks).length) delete s.hooks;
fs.writeFileSync('$SETTINGS', JSON.stringify(s, null, 2));
"
else
  echo "⚠️  jq / python3 / node が見つかりません。手動で settings.json を編集してください。"
  exit 1
fi

echo "✅ UserPromptSubmit hook を削除しました"
