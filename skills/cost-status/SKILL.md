---
name: cost-status
description: 現在の Bedrock 使用コスト（日次・月次）をターミナルに表示する
version: 1.0.0
---

以下のコマンドを実行して、現在の Bedrock 使用コストを表示してください:

```bash
python3 "${CLAUDE_PLUGIN_ROOT}/hooks/bedrock_limiter.py" --status
```

出力例:
```
今日の使用コスト:  $2.1400 / $5.00 (42.8%)  [████████░░░░░░░░░░░░]
今月の使用コスト:  $18.5000 / $50.00 (37.0%) [███████░░░░░░░░░░░░░]

設定ファイル: /path/to/bedrock-limiter/config.json
料金情報:     https://aws.amazon.com/bedrock/pricing/
```

制限値の変更は `config.json` の `daily_limit_usd` / `monthly_limit_usd` を編集してください。
