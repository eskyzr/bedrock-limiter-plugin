# Claude Code Bedrock コスト制限プラグイン

Claude Code を Amazon Bedrock 経由で使用するときの**日次・月次コスト制限**プラグインです。

設定した上限を超えるとプロンプトの処理をブロックし、指定した割合に達した段階で警告を表示します。

## 仕組み

- `UserPromptSubmit` hook が毎プロンプト送信前に発火
- `~/.claude/projects/*/` 配下の transcript JSONL を走査してトークン使用量を集計
- Bedrock エントリの識別: `requestId` が空（Bedrock）か `req_` 始まり（Anthropic API）かで判定
- 上限超過 → `exit 2` でブロック / `warn_percent` 以上 → 警告のみ
- 料金データは [LiteLLM](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json) から自動取得（7日ごとに更新）

## devcontainer での利用

devcontainer 内で Claude Code を動かす場合、transcript がコンテナ内に書かれるためホスト側からは見えません。
`devcontainer.json` に以下を追加してホストの `~/.claude/` をマウントしてください。

```json
"mounts": [
  "source=${localEnv:HOME}/.claude,target=/root/.claude,type=bind"
]
```

## インストール

### 社内マーケットプレイス (推奨)

```
/plugin install bedrock-limiter@ie
```

### 直接インストール

```
/plugin install https://github.com/eskyzr/bedrock-limiter-plugin.git
```

## セットアップ

インストール後、初回のみ料金キャッシュを取得してください。

```bash
python3 ~/.claude/plugins/cache/ie/bedrock-limiter/0.1.0/hooks/bedrock_limiter.py --update-pricing
```

## 設定

プラグインルート直下の `config.json` を編集します。初回プロンプト送信時にデフォルト値で自動生成されます。

```json
{
  "daily_limit_usd": 5.0,
  "monthly_limit_usd": 50.0,
  "warn_percent": 80,
  "bedrock_only": true
}
```

| 項目 | 説明 |
|---|---|
| `daily_limit_usd` | 1日あたりの上限（USD） |
| `monthly_limit_usd` | 1ヶ月あたりの上限（USD） |
| `warn_percent` | この割合を超えると警告を表示（デフォルト: 80%） |
| `bedrock_only` | `true`: Bedrock のみ集計 / `false`: 全プロバイダ集計 |

## 使用量確認

```bash
PLUGIN_ROOT=~/.claude/plugins/cache/ie/bedrock-limiter/0.1.0

# コマンドラインから確認
python3 $PLUGIN_ROOT/hooks/bedrock_limiter.py --status

# 料金キャッシュを手動更新
python3 $PLUGIN_ROOT/hooks/bedrock_limiter.py --update-pricing
```

出力例:
```
集計対象: Bedrock のみ  |  料金データ: LiteLLM キャッシュ
今日の使用コスト:  $0.9982 / $5.00 (20.0%)  [███░░░░░░░░░░░░░░░░░]
今月の使用コスト:  $0.9982 / $50.00 (2.0%) [░░░░░░░░░░░░░░░░░░░░]
```

## 注意事項

- Claude Code 経由の Bedrock 呼び出しのみ追跡します（SDK 直接呼び出しは対象外）
- transcript JSONL のタイムスタンプは UTC です
- 表示コストは推計値です。実際の請求額は AWS コンソールで確認してください
- 料金データの最新値は [AWS Bedrock 料金ページ](https://aws.amazon.com/bedrock/pricing/) を参照してください
