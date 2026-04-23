#!/bin/bash
# Claude Code Bedrock コスト制限 - 手動インストールスクリプト
# プラグインシステムが使えない場合のフォールバック

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HOOK_SCRIPT="$SCRIPT_DIR/hooks/bedrock_limiter.py"
SETTINGS="$HOME/.claude/settings.json"

echo "=== Claude Code Bedrock Limiter インストール ==="

# Python3 確認
if ! command -v python3 &>/dev/null; then
  echo "❌ Python3 が見つかりません。インストールしてください。"
  exit 1
fi

# 実行権限付与
chmod +x "$HOOK_SCRIPT"
echo "✅ $HOOK_SCRIPT に実行権限を付与しました"

# settings.json バックアップ & hook 追加
if [ ! -f "$SETTINGS" ]; then
  echo "{}" > "$SETTINGS"
fi

cp "$SETTINGS" "$SETTINGS.bak"
echo "✅ $SETTINGS をバックアップしました ($SETTINGS.bak)"

python3 - <<PYEOF
import json
from pathlib import Path

settings_path = Path("$SETTINGS")
settings = json.loads(settings_path.read_text()) if settings_path.exists() else {}

settings.setdefault("hooks", {})
settings["hooks"]["UserPromptSubmit"] = [
    {
        "hooks": [
            {
                "type": "command",
                "command": "python3 $HOOK_SCRIPT"
            }
        ]
    }
]

settings_path.write_text(json.dumps(settings, indent=2, ensure_ascii=False))
PYEOF

echo "✅ ~/.claude/settings.json に UserPromptSubmit hook を登録しました"
echo ""
echo "=== インストール完了 ==="
echo ""
echo "設定ファイル: $SCRIPT_DIR/config.json (初回起動時に自動生成)"
echo "コスト確認:   python3 $HOOK_SCRIPT --status"
echo "アンインストール: bash $SCRIPT_DIR/uninstall.sh"
