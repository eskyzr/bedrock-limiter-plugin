---
name: cost-status
description: 現在の Bedrock 使用コスト（日次・月次）をターミナルに表示する
version: 1.0.0
---

## 手順

まず Python3 が使えるか確認してください:

```bash
python3 --version
```

Python3 が見つからない場合は以下でインストールしてから再実行してください:

```bash
# Ubuntu / Debian
sudo apt-get install -y python3
```

Python3 が使える場合は以下を実行してください:

```bash
python3 "${CLAUDE_PLUGIN_ROOT}/hooks/bedrock_limiter.py" --status
```

出力例:
```
集計対象: Bedrock のみ  |  料金データ: LiteLLM キャッシュ
今日の使用コスト:  $2.1400 / $5.00 (42.8%)  [████████░░░░░░░░░░░░]
今月の使用コスト:  $18.5000 / $50.00 (37.0%) [███████░░░░░░░░░░░░░]

設定ファイル: /path/to/bedrock-limiter/config.json
料金更新:     python3 bedrock_limiter.py --update-pricing
```

エラーが出た場合はエラーメッセージをそのまま伝えてください。

制限値の変更は `/set-limit` スキルを使うか、`config.json` の `daily_limit_usd` / `monthly_limit_usd` を直接編集してください。
