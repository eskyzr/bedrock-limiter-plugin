#!/bin/bash
# hook 設定を settings.json から除去する

SETTINGS="$HOME/.claude/settings.json"

if [ ! -f "$SETTINGS" ]; then
  echo "settings.json が見つかりません"
  exit 0
fi

python3 - <<PYEOF
import json
from pathlib import Path

settings_path = Path("$SETTINGS")
settings = json.loads(settings_path.read_text())

hooks = settings.get("hooks", {})
if "UserPromptSubmit" in hooks:
    del hooks["UserPromptSubmit"]
    if not hooks:
        del settings["hooks"]
    settings_path.write_text(json.dumps(settings, indent=2, ensure_ascii=False))
    print("✅ UserPromptSubmit hook を削除しました")
else:
    print("hook は登録されていませんでした")
PYEOF
