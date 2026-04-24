---
name: set-limit
description: Bedrock コスト制限値（日次・月次・警告割合）を対話的に変更する
version: 0.1.1
---

以下の手順で Bedrock コスト制限の設定を変更してください。

## 手順

1. 設定ファイルのパスを特定する:
   ```bash
   echo "${CLAUDE_PLUGIN_ROOT}/config.json"
   ```

2. そのパスの config.json を Read ツールで読み込み、現在の設定値を日本語で表示する:
   - 日次上限: `daily_limit_usd` USD
   - 月次上限: `monthly_limit_usd` USD
   - 警告しきい値: `warn_percent` %
   - 集計対象: `bedrock_only` が true なら「Bedrock のみ」、false なら「全プロバイダ」

3. ユーザーの指示から変更する項目と値を読み取る。指示がなければ何を変更するか聞く。

   変更可能な項目:
   | 項目 | キー | 型 | 説明 |
   |---|---|---|---|
   | 日次上限 | `daily_limit_usd` | 数値 (USD) | 1日あたりのコスト上限 |
   | 月次上限 | `monthly_limit_usd` | 数値 (USD) | 1ヶ月あたりのコスト上限 |
   | 警告しきい値 | `warn_percent` | 整数 (%) | この割合を超えると警告を表示 |
   | 集計対象 | `bedrock_only` | true/false | Bedrock のみ集計するか |

4. 変更内容をユーザーに確認してから Edit ツールで config.json を更新する

5. 更新後の設定を表示して完了を伝える
