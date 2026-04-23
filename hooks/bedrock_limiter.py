#!/usr/bin/env python3
"""
Claude Code Bedrock コスト制限 hook

UserPromptSubmit hook として動作し、日次・月次のコスト上限に達した場合に
プロンプトの処理をブロックする。

設定ファイル:   ${CLAUDE_PLUGIN_ROOT}/config.json
手動確認:       python3 bedrock_limiter.py --status
料金キャッシュ更新: python3 bedrock_limiter.py --update-pricing
"""

import json
import sys
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


SCRIPT_DIR = Path(__file__).parent.parent
CONFIG_FILE = SCRIPT_DIR / "config.json"
PRICING_CACHE_FILE = SCRIPT_DIR / "pricing_cache.json"
LITELLM_PRICING_URL = (
    "https://raw.githubusercontent.com/BerriAI/litellm/main"
    "/model_prices_and_context_window.json"
)
PRICING_CACHE_TTL_DAYS = 7

# フォールバック用ハードコード料金 ($/1M tokens, 2026年4月時点)
FALLBACK_PRICING = {
    "opus":   {"input": 5.5,  "output": 27.5, "cache_write": 6.875, "cache_read": 0.55},
    "sonnet": {"input": 3.0,  "output": 15.0, "cache_write": 3.75,  "cache_read": 0.3},
    "haiku":  {"input": 1.0,  "output": 5.0,  "cache_write": 1.25,  "cache_read": 0.1},
    "default":{"input": 3.0,  "output": 15.0, "cache_write": 3.75,  "cache_read": 0.3},
}

DEFAULT_CONFIG = {
    "daily_limit_usd": 5.0,
    "monthly_limit_usd": 50.0,
    "warn_percent": 80,
    # true: requestId が "req_" 以外のエントリ(Bedrock)だけ集計する
    # false: すべてのエントリを集計する
    "bedrock_only": True,
}


# ---------------------------------------------------------------------------
# 設定ファイル
# ---------------------------------------------------------------------------

def load_config() -> dict:
    if CONFIG_FILE.exists():
        try:
            return json.loads(CONFIG_FILE.read_text())
        except Exception:
            pass
    CONFIG_FILE.write_text(json.dumps(DEFAULT_CONFIG, indent=2, ensure_ascii=False))
    return DEFAULT_CONFIG


# ---------------------------------------------------------------------------
# LiteLLM 料金キャッシュ
# ---------------------------------------------------------------------------

def _fetch_litellm_pricing() -> dict | None:
    """LiteLLM の料金 JSON を取得して Bedrock エントリだけ正規化して返す"""
    try:
        with urllib.request.urlopen(LITELLM_PRICING_URL, timeout=5) as resp:
            raw = json.loads(resp.read())
    except Exception:
        return None

    pricing: dict[str, dict] = {}
    for key, data in raw.items():
        if not isinstance(data, dict):
            continue
        provider = data.get("litellm_provider", "")
        if "bedrock" not in provider:
            continue

        # キーを正規化: "us.anthropic.claude-opus-4-7" → "claude-opus-4-7"
        normalized = key
        for region in ("us.", "eu.", "au.", "apac.", "global."):
            if normalized.startswith(region):
                normalized = normalized[len(region):]
                break
        if normalized.startswith("anthropic."):
            normalized = normalized[len("anthropic."):]

        pricing[normalized] = {
            "input":       (data.get("input_cost_per_token") or 0) * 1_000_000,
            "output":      (data.get("output_cost_per_token") or 0) * 1_000_000,
            "cache_write": (data.get("cache_creation_input_token_cost") or 0) * 1_000_000,
            "cache_read":  (data.get("cache_read_input_token_cost") or 0) * 1_000_000,
        }

    return pricing if pricing else None


def load_pricing() -> dict:
    """
    キャッシュが新鮮なら使い、期限切れなら LiteLLM から更新を試みる。
    失敗時はフォールバック料金を返す。
    """
    cache_fresh = False
    cached_pricing: dict | None = None

    if PRICING_CACHE_FILE.exists():
        try:
            cache_data = json.loads(PRICING_CACHE_FILE.read_text())
            cached_at = datetime.fromisoformat(cache_data["cached_at"])
            age_days = (datetime.now(timezone.utc) - cached_at).days
            cached_pricing = cache_data["pricing"]
            cache_fresh = age_days < PRICING_CACHE_TTL_DAYS
        except Exception:
            pass

    if cache_fresh and cached_pricing:
        return cached_pricing

    # キャッシュ期限切れ or 未作成 → 更新を試みる
    fetched = _fetch_litellm_pricing()
    if fetched:
        try:
            PRICING_CACHE_FILE.write_text(json.dumps({
                "cached_at": datetime.now(timezone.utc).isoformat(),
                "pricing": fetched,
            }, indent=2))
        except Exception:
            pass
        return fetched

    # 更新失敗 → キャッシュがあれば古くても使う
    if cached_pricing:
        return cached_pricing

    return {}  # フォールバック料金で対応


def get_price(model_name: str, litellm_pricing: dict) -> dict:
    """
    transcript の model 名から料金を解決する。
    1. LiteLLM キャッシュで完全一致
    2. LiteLLM キャッシュで前方一致（バージョンサフィックス違い）
    3. フォールバック料金（opus/sonnet/haiku のキーワード一致）
    """
    # 完全一致
    if model_name in litellm_pricing:
        return litellm_pricing[model_name]

    # 前方一致（"claude-opus-4-7" が "claude-opus-4-7-20250514-v1:0" の先頭に含まれる）
    for key, price in litellm_pricing.items():
        if key.startswith(model_name):
            return price

    # フォールバック
    model_lower = model_name.lower()
    for family in ("opus", "sonnet", "haiku"):
        if family in model_lower:
            return FALLBACK_PRICING[family]
    return FALLBACK_PRICING["default"]


# ---------------------------------------------------------------------------
# コスト計算
# ---------------------------------------------------------------------------

def calc_entry_cost(usage: dict, model_name: str, litellm_pricing: dict) -> float:
    p = get_price(model_name, litellm_pricing)
    return (
        usage.get("input_tokens", 0)                * p["input"]       / 1_000_000
        + usage.get("output_tokens", 0)             * p["output"]      / 1_000_000
        + usage.get("cache_creation_input_tokens", 0) * p["cache_write"] / 1_000_000
        + usage.get("cache_read_input_tokens", 0)   * p["cache_read"]  / 1_000_000
    )


def is_bedrock_entry(entry: dict) -> bool:
    """
    - Anthropic API / Max plan: requestId が "req_" で始まる
    - Bedrock: requestId が AWS UUID 形式またはそれ以外
    """
    return not entry.get("requestId", "").startswith("req_")


def scan_costs(today: str, month: str, litellm_pricing: dict, bedrock_only: bool) -> tuple[float, float]:
    daily_cost = 0.0
    monthly_cost = 0.0
    projects_dir = Path.home() / ".claude" / "projects"

    if not projects_dir.exists():
        return daily_cost, monthly_cost

    for jsonl_file in projects_dir.glob("*/*.jsonl"):
        try:
            mtime = datetime.fromtimestamp(jsonl_file.stat().st_mtime)
            if mtime.strftime("%Y-%m") < month:
                continue
        except OSError:
            continue

        try:
            with open(jsonl_file, encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    entry = json.loads(line)
                    if entry.get("type") != "assistant":
                        continue
                    ts = entry.get("timestamp", "")
                    if not ts.startswith(month):
                        continue
                    if bedrock_only and not is_bedrock_entry(entry):
                        continue
                    message = entry.get("message", {})
                    usage = message.get("usage", {})
                    model = message.get("model", "default")
                    if not usage:
                        continue
                    cost = calc_entry_cost(usage, model, litellm_pricing)
                    monthly_cost += cost
                    if ts.startswith(today):
                        daily_cost += cost
        except (OSError, json.JSONDecodeError):
            continue

    return daily_cost, monthly_cost


# ---------------------------------------------------------------------------
# コマンド
# ---------------------------------------------------------------------------

def build_bar(ratio: float, width: int = 20) -> str:
    filled = min(int(ratio * width), width)
    return "█" * filled + "░" * (width - filled)


def cmd_update_pricing():
    """LiteLLM から料金キャッシュを強制更新する"""
    print("料金データを取得中...", end=" ", flush=True)
    fetched = _fetch_litellm_pricing()
    if not fetched:
        print("失敗")
        print("ネットワークに接続できないか、URL が変更された可能性があります。", file=sys.stderr)
        sys.exit(1)

    PRICING_CACHE_FILE.write_text(json.dumps({
        "cached_at": datetime.now(timezone.utc).isoformat(),
        "pricing": fetched,
    }, indent=2))
    print(f"完了 ({len(fetched)} モデル)")
    print(f"キャッシュ: {PRICING_CACHE_FILE}")


def cmd_status(config: dict, litellm_pricing: dict):
    now = datetime.now()
    today = now.strftime("%Y-%m-%d")
    month = now.strftime("%Y-%m")

    bedrock_only = config.get("bedrock_only", True)
    daily_cost, monthly_cost = scan_costs(today, month, litellm_pricing, bedrock_only)
    daily_limit = config.get("daily_limit_usd", 5.0)
    monthly_limit = config.get("monthly_limit_usd", 50.0)

    daily_ratio = daily_cost / daily_limit if daily_limit > 0 else 0
    monthly_ratio = monthly_cost / monthly_limit if monthly_limit > 0 else 0

    mode_label = "Bedrock のみ" if bedrock_only else "全プロバイダ"
    cache_label = "LiteLLM キャッシュ" if PRICING_CACHE_FILE.exists() else "フォールバック"

    print(f"集計対象: {mode_label}  |  料金データ: {cache_label}")
    print(f"今日の使用コスト:  ${daily_cost:.4f} / ${daily_limit:.2f} ({daily_ratio*100:.1f}%)  [{build_bar(daily_ratio)}]")
    print(f"今月の使用コスト:  ${monthly_cost:.4f} / ${monthly_limit:.2f} ({monthly_ratio*100:.1f}%) [{build_bar(monthly_ratio)}]")
    print(f"\n設定ファイル: {CONFIG_FILE}")
    print(f"料金更新:     python3 {Path(__file__).name} --update-pricing")


def cmd_check(config: dict, litellm_pricing: dict):
    """hook モード: 上限超過なら exit 2 でブロック"""
    bedrock_only = config.get("bedrock_only", True)
    now = datetime.now()
    today = now.strftime("%Y-%m-%d")
    month = now.strftime("%Y-%m")

    daily_cost, monthly_cost = scan_costs(today, month, litellm_pricing, bedrock_only)
    daily_limit = config.get("daily_limit_usd", 5.0)
    monthly_limit = config.get("monthly_limit_usd", 50.0)
    warn_pct = config.get("warn_percent", 80) / 100.0

    blocked = False

    if daily_cost >= daily_limit:
        print(f"⛔ 日次コスト上限超過: ${daily_cost:.4f} / ${daily_limit:.2f}", file=sys.stderr)
        print(f"   config.json の daily_limit_usd を変更するか、明日以降に再開してください。", file=sys.stderr)
        blocked = True
    elif daily_cost >= daily_limit * warn_pct:
        pct = daily_cost / daily_limit * 100
        print(f"⚠️  日次コスト警告: ${daily_cost:.4f} / ${daily_limit:.2f} ({pct:.0f}%)", file=sys.stderr)

    if monthly_cost >= monthly_limit:
        print(f"⛔ 月次コスト上限超過: ${monthly_cost:.4f} / ${monthly_limit:.2f}", file=sys.stderr)
        print(f"   config.json の monthly_limit_usd を変更するか、来月以降に再開してください。", file=sys.stderr)
        blocked = True
    elif monthly_cost >= monthly_limit * warn_pct:
        pct = monthly_cost / monthly_limit * 100
        print(f"⚠️  月次コスト警告: ${monthly_cost:.4f} / ${monthly_limit:.2f} ({pct:.0f}%)", file=sys.stderr)

    sys.exit(2 if blocked else 0)


# ---------------------------------------------------------------------------
# エントリポイント
# ---------------------------------------------------------------------------

def main():
    config = load_config()

    if len(sys.argv) > 1:
        if sys.argv[1] == "--update-pricing":
            cmd_update_pricing()
            return
        if sys.argv[1] == "--status":
            litellm_pricing = load_pricing()
            cmd_status(config, litellm_pricing)
            return

    # hook モード: stdin から JSON を読む（UserPromptSubmit）
    try:
        json.loads(sys.stdin.read())
    except (json.JSONDecodeError, OSError):
        sys.exit(0)

    litellm_pricing = load_pricing()
    cmd_check(config, litellm_pricing)


if __name__ == "__main__":
    main()
