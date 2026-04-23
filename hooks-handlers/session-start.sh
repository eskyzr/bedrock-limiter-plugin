#!/usr/bin/env bash

# Python3 がインストールされているか確認し、なければ警告を会話に注入する

if command -v python3 &>/dev/null; then
  exit 0
fi

cat << 'EOF'
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "⚠️ **bedrock-limiter: Python3 が見つかりません**\n\nこのプラグインのコスト制限 hook は Python3 が必要です。インストールしてください。\n\n```bash\n# Debian / Ubuntu\nsudo apt-get install -y python3\n\n# macOS (Homebrew)\nbrew install python3\n\n# Amazon Linux / RHEL\nsudo yum install -y python3\n```\n\nインストール後、Claude Code を再起動してください。"
  }
}
EOF

exit 0
