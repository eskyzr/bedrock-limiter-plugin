#!/bin/bash
# hook 設定を settings.json から除去する

SETTINGS="$HOME/.claude/settings.json"

if [ ! -f "$SETTINGS" ]; then
  echo "settings.json が見つかりません"
  exit 0
fi

node - <<JSEOF
const fs = require('fs');
const settingsPath = "$SETTINGS";
const settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));

const hooks = settings.hooks || {};
if ('UserPromptSubmit' in hooks) {
  delete hooks.UserPromptSubmit;
  if (Object.keys(hooks).length === 0) delete settings.hooks;
  fs.writeFileSync(settingsPath, JSON.stringify(settings, null, 2));
  console.log('✅ UserPromptSubmit hook を削除しました');
} else {
  console.log('hook は登録されていませんでした');
}
JSEOF
