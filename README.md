# Claude Code Bedrock コスト制限プラグイン

Claude Code を Amazon Bedrock 経由で使用するときの**日次・月次コスト制限**プラグインです。

設定した上限を超えるとプロンプトの処理をブロックし、80% に達した段階で警告を表示します。

## 仕組み

- `UserPromptSubmit` hook が毎プロンプト送信前に発火
- `~/.claude/projects/*/` 配下の transcript JSONL を走査してトークン使用量を集計
- 上限超過 → `exit 2` でブロック / 80% 以上 → 警告のみ

## インストール

### プラグインとして (推奨)

```bash
claude /plugin install https://github.com/<user>/claudecode-bedrock-limiter
```

### 手動インストール

```bash
git clone https://github.com/<user>/claudecode-bedrock-limiter
cd claudecode-bedrock-limiter
bash install.sh
```

## 設定

初回起動時に `config.json` が自動生成されます。

```json
{
  "daily_limit_usd": 5.0,
  "monthly_limit_usd": 50.0,
  "warn_percent": 80,
  "pricing": {
    "opus":   {"input": 5.0,  "output": 25.0, "cache_write": 6.25, "cache_read": 0.5},
    "sonnet": {"input": 3.0,  "output": 15.0, "cache_write": 3.75, "cache_read": 0.3},
    "haiku":  {"input": 0.8,  "output": 4.0,  "cache_write": 1.0,  "cache_read": 0.08},
    "default":{"input": 3.0,  "output": 15.0, "cache_write": 3.75, "cache_read": 0.3}
  }
}
```

- `daily_limit_usd` / `monthly_limit_usd`: 上限額（USD）
- `warn_percent`: この割合を超えると警告を表示（デフォルト: 80%）
- `pricing`: モデルごとの料金（$/1M tokens）。最新値は [AWS Bedrock 料金ページ](https://aws.amazon.com/bedrock/pricing/) を参照

## 使用量確認

```bash
# コマンドラインから確認
python3 hooks/bedrock_limiter.py --status

# Claude Code 内から確認
/cost-status
```

出力例:
```
今日の使用コスト:  $2.1400 / $5.00 (42.8%)  [████████░░░░░░░░░░░░]
今月の使用コスト:  $18.5000 / $50.00 (37.0%) [███████░░░░░░░░░░░░░]
```

## アンインストール

```bash
bash uninstall.sh
```

## 注意事項

- Claude Code 経由の Bedrock 呼び出しのみ追跡します（SDK 直接呼び出しは対象外）
- transcript JSONL のタイムスタンプは UTC です
- 料金はユーザー設定の値を使用します。実際の請求額は AWS コンソールで確認してください
