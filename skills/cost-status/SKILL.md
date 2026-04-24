---
name: cost-status
description: 現在の Bedrock 使用コスト（日次・月次）をターミナルに表示する
version: 1.0.0
---

以下のコマンドを実行して、現在の Bedrock 使用コストを表示してください:

```bash
"${CLAUDE_PLUGIN_ROOT}/hooks/bedrock_limiter.sh" --status
```

出力例:
```
集計対象: Bedrock のみ  |  料金データ: LiteLLM キャッシュ
今日の使用コスト:  $2.1400 / $5.00 (42.8%)  [████████░░░░░░░░░░░░]
今月の使用コスト:  $18.5000 / $50.00 (37.0%) [███████░░░░░░░░░░░░░]

設定ファイル: /path/to/bedrock-limiter/config.json
```

エラーが出た場合はエラーメッセージをそのまま伝えてください。

制限値の変更は `/set-limit` スキルを使うか、`config.json` の `daily_limit_usd` / `monthly_limit_usd` を直接編集してください。
