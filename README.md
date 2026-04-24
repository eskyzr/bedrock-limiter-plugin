# Claude Code Bedrock コスト制限プラグイン

Claude Code を Amazon Bedrock 経由で使用するときの**日次・月次コスト制限**プラグインです。

設定した上限を超えるとプロンプトの処理をブロックし、指定した割合に達した段階で警告を表示します。

```
● UserPromptSubmit operation blocked by hook:                                                  
  [${CLAUDE_PLUGIN_ROOT}/hooks/bedrock_limiter.sh]: ⛔ 日次コスト上限超過:             
  $3.9745 / $3.00                                                                              
     上限を上げるには以下のファイルの daily_limit_usd を編集してください:
     /path/to/config.json
   
  Original prompt: こんにちは
```

## 前提条件

なし（バイナリ同梱のため追加インストール不要）

対応プラットフォーム: Linux (amd64/arm64)、macOS (amd64/arm64)、Windows (amd64)

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

インストール後、`/cost-status` を実行して動作確認してください。

動作確認できたら、初回のみ料金キャッシュを取得してください。

```bash
PLUGIN_ROOT=~/.claude/plugins/cache/ie/bedrock-limiter/1.0.0
"$PLUGIN_ROOT/hooks/bedrock_limiter.sh" --update-pricing
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
PLUGIN_ROOT=~/.claude/plugins/cache/ie/bedrock-limiter/1.0.0

# コマンドラインから確認
"$PLUGIN_ROOT/hooks/bedrock_limiter.sh" --status

# 料金キャッシュを手動更新
"$PLUGIN_ROOT/hooks/bedrock_limiter.sh" --update-pricing
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

## devcontainer での利用

devcontainer 内で Claude Code を動かす場合、transcript がコンテナ内に書かれるためホスト側からは見えません。
`devcontainer.json` に以下を追加してホストの `~/.claude/` をマウントしてください。

```json
"mounts": [
  "source=${localEnv:HOME}/.claude,target=/home/devcontainer/.claude,type=bind"
]
```

## 仕組みの話

`UserPromptSubmit` hook が毎プロンプト送信前に発火します。

料金データは[LiteLLM](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json)を見ています。
ClaudeCodeは.claude配下に利用しているモデル名、トークン数などのデータを保管しており、このモデル名と料金データを突合して料金を計算しています。

```
~/.claude/projects/
  ├── -workspaces-re-terraform-aws/
  │   └── 5e5b7945-xxxx.jsonl
  ├── -home-uzresk-claudecode-bedrock-limiter/
  │   └── 664a89b3-xxxx.jsonl
  └── -home-uzresk-nginx-proxy/
      └── b66af9d9-xxxx.jsonl
```

Anthropic API or Bedrockは以下のように判断しています
- Anthropic API / Max プラン → requestId: "req_011CaK..."（req_ で始まる）
- Bedrock → requestId: ""（空）
