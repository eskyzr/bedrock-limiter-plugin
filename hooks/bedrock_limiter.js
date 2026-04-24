#!/usr/bin/env node
'use strict';

const fs = require('fs');
const https = require('https');
const path = require('path');
const os = require('os');

const SCRIPT_DIR = path.dirname(__dirname);
const CONFIG_FILE = path.join(SCRIPT_DIR, 'config.json');
const PRICING_CACHE_FILE = path.join(SCRIPT_DIR, 'pricing_cache.json');
const LITELLM_PRICING_URL =
  'https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json';
const PRICING_CACHE_TTL_DAYS = 7;

const FALLBACK_PRICING = {
  opus:    { input: 5.5,  output: 27.5, cache_write: 6.875, cache_read: 0.55 },
  sonnet:  { input: 3.0,  output: 15.0, cache_write: 3.75,  cache_read: 0.3  },
  haiku:   { input: 1.0,  output: 5.0,  cache_write: 1.25,  cache_read: 0.1  },
  default: { input: 3.0,  output: 15.0, cache_write: 3.75,  cache_read: 0.3  },
};

const DEFAULT_CONFIG = {
  daily_limit_usd: 5.0,
  monthly_limit_usd: 50.0,
  warn_percent: 80,
  bedrock_only: true,
};

function loadConfig() {
  if (fs.existsSync(CONFIG_FILE)) {
    try { return JSON.parse(fs.readFileSync(CONFIG_FILE, 'utf8')); } catch (e) {}
  }
  fs.writeFileSync(CONFIG_FILE, JSON.stringify(DEFAULT_CONFIG, null, 2));
  return DEFAULT_CONFIG;
}

function fetchLitellmPricing() {
  return new Promise((resolve) => {
    const req = https.get(LITELLM_PRICING_URL, { timeout: 5000 }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const raw = JSON.parse(data);
          const pricing = {};
          for (const [key, val] of Object.entries(raw)) {
            if (typeof val !== 'object' || !val) continue;
            if (!(val.litellm_provider || '').includes('bedrock')) continue;

            let normalized = key;
            for (const region of ['us.', 'eu.', 'au.', 'apac.', 'global.']) {
              if (normalized.startsWith(region)) { normalized = normalized.slice(region.length); break; }
            }
            if (normalized.startsWith('anthropic.')) normalized = normalized.slice('anthropic.'.length);

            pricing[normalized] = {
              input:       (val.input_cost_per_token || 0) * 1_000_000,
              output:      (val.output_cost_per_token || 0) * 1_000_000,
              cache_write: (val.cache_creation_input_token_cost || 0) * 1_000_000,
              cache_read:  (val.cache_read_input_token_cost || 0) * 1_000_000,
            };
          }
          resolve(Object.keys(pricing).length > 0 ? pricing : null);
        } catch (e) { resolve(null); }
      });
    });
    req.on('error', () => resolve(null));
    req.on('timeout', () => { req.destroy(); resolve(null); });
  });
}

async function loadPricing() {
  let cachedPricing = null;
  let cacheFresh = false;

  if (fs.existsSync(PRICING_CACHE_FILE)) {
    try {
      const cacheData = JSON.parse(fs.readFileSync(PRICING_CACHE_FILE, 'utf8'));
      const ageDays = (Date.now() - new Date(cacheData.cached_at).getTime()) / 86_400_000;
      cachedPricing = cacheData.pricing;
      cacheFresh = ageDays < PRICING_CACHE_TTL_DAYS;
    } catch (e) {}
  }

  if (cacheFresh && cachedPricing) return cachedPricing;

  const fetched = await fetchLitellmPricing();
  if (fetched) {
    try {
      fs.writeFileSync(PRICING_CACHE_FILE, JSON.stringify(
        { cached_at: new Date().toISOString(), pricing: fetched }, null, 2));
    } catch (e) {}
    return fetched;
  }

  return cachedPricing || {};
}

function getPrice(modelName, litellmPricing) {
  if (litellmPricing[modelName]) return litellmPricing[modelName];
  for (const [key, price] of Object.entries(litellmPricing)) {
    if (key.startsWith(modelName)) return price;
  }
  const lower = modelName.toLowerCase();
  for (const family of ['opus', 'sonnet', 'haiku']) {
    if (lower.includes(family)) return FALLBACK_PRICING[family];
  }
  return FALLBACK_PRICING.default;
}

function calcEntryCost(usage, modelName, litellmPricing) {
  const p = getPrice(modelName, litellmPricing);
  return (
    (usage.input_tokens || 0)                * p.input       / 1_000_000 +
    (usage.output_tokens || 0)               * p.output      / 1_000_000 +
    (usage.cache_creation_input_tokens || 0) * p.cache_write / 1_000_000 +
    (usage.cache_read_input_tokens || 0)     * p.cache_read  / 1_000_000
  );
}

function isBedrockEntry(entry) {
  return !(entry.requestId || '').startsWith('req_');
}

function localDateStrings() {
  const now = new Date();
  const pad = n => String(n).padStart(2, '0');
  const today = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
  const month = `${now.getFullYear()}-${pad(now.getMonth() + 1)}`;
  return { today, month };
}

function scanCosts(today, month, litellmPricing, bedrockOnly) {
  let dailyCost = 0;
  let monthlyCost = 0;
  const projectsDir = path.join(os.homedir(), '.claude', 'projects');

  if (!fs.existsSync(projectsDir)) return [dailyCost, monthlyCost];

  for (const project of fs.readdirSync(projectsDir)) {
    const projectPath = path.join(projectsDir, project);
    try { if (!fs.statSync(projectPath).isDirectory()) continue; } catch (e) { continue; }

    for (const file of fs.readdirSync(projectPath)) {
      if (!file.endsWith('.jsonl')) continue;
      const filePath = path.join(projectPath, file);

      try {
        const mtimeMonth = fs.statSync(filePath).mtime.toISOString().slice(0, 7);
        if (mtimeMonth < month) continue;
      } catch (e) { continue; }

      try {
        const lines = fs.readFileSync(filePath, 'utf8').split('\n');
        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed) continue;
          let entry;
          try { entry = JSON.parse(trimmed); } catch (e) { continue; }
          if (entry.type !== 'assistant') continue;
          const ts = entry.timestamp || '';
          if (!ts.startsWith(month)) continue;
          if (bedrockOnly && !isBedrockEntry(entry)) continue;
          const usage = (entry.message || {}).usage || {};
          if (!Object.keys(usage).length) continue;
          const model = (entry.message || {}).model || 'default';
          const cost = calcEntryCost(usage, model, litellmPricing);
          monthlyCost += cost;
          if (ts.startsWith(today)) dailyCost += cost;
        }
      } catch (e) { continue; }
    }
  }

  return [dailyCost, monthlyCost];
}

function buildBar(ratio, width = 20) {
  const filled = Math.min(Math.floor(ratio * width), width);
  return '█'.repeat(filled) + '░'.repeat(width - filled);
}

async function cmdUpdatePricing() {
  process.stdout.write('料金データを取得中... ');
  const fetched = await fetchLitellmPricing();
  if (!fetched) {
    console.log('失敗');
    console.error('ネットワークに接続できないか、URL が変更された可能性があります。');
    process.exit(1);
  }
  fs.writeFileSync(PRICING_CACHE_FILE, JSON.stringify(
    { cached_at: new Date().toISOString(), pricing: fetched }, null, 2));
  console.log(`完了 (${Object.keys(fetched).length} モデル)`);
  console.log(`キャッシュ: ${PRICING_CACHE_FILE}`);
}

async function cmdStatus(config, litellmPricing) {
  const { today, month } = localDateStrings();
  const bedrockOnly = config.bedrock_only !== false;
  const [dailyCost, monthlyCost] = scanCosts(today, month, litellmPricing, bedrockOnly);
  const dailyLimit = config.daily_limit_usd || 5.0;
  const monthlyLimit = config.monthly_limit_usd || 50.0;

  const dailyRatio   = dailyLimit   > 0 ? dailyCost   / dailyLimit   : 0;
  const monthlyRatio = monthlyLimit > 0 ? monthlyCost / monthlyLimit : 0;

  const modeLabel  = bedrockOnly ? 'Bedrock のみ' : '全プロバイダ';
  const cacheLabel = fs.existsSync(PRICING_CACHE_FILE) ? 'LiteLLM キャッシュ' : 'フォールバック';

  console.log(`集計対象: ${modeLabel}  |  料金データ: ${cacheLabel}`);
  console.log(`今日の使用コスト:  $${dailyCost.toFixed(4)} / $${dailyLimit.toFixed(2)} (${(dailyRatio * 100).toFixed(1)}%)  [${buildBar(dailyRatio)}]`);
  console.log(`今月の使用コスト:  $${monthlyCost.toFixed(4)} / $${monthlyLimit.toFixed(2)} (${(monthlyRatio * 100).toFixed(1)}%) [${buildBar(monthlyRatio)}]`);
  console.log(`\n設定ファイル: ${CONFIG_FILE}`);
  console.log(`料金更新:     node ${path.basename(__filename)} --update-pricing`);
}

async function cmdCheck(config, litellmPricing) {
  const { today, month } = localDateStrings();
  const bedrockOnly = config.bedrock_only !== false;
  const [dailyCost, monthlyCost] = scanCosts(today, month, litellmPricing, bedrockOnly);
  const dailyLimit   = config.daily_limit_usd   || 5.0;
  const monthlyLimit = config.monthly_limit_usd || 50.0;
  const warnPct      = (config.warn_percent || 80) / 100;

  const warnings = [];
  let blocked = false;

  if (dailyCost >= dailyLimit) {
    process.stderr.write(`⛔ 日次コスト上限超過: $${dailyCost.toFixed(4)} / $${dailyLimit.toFixed(2)}\n`);
    process.stderr.write(`   上限を上げるには以下のファイルの daily_limit_usd を編集してください:\n`);
    process.stderr.write(`   ${CONFIG_FILE}\n`);
    process.stderr.write(`   （または明日以降に再開してください）\n`);
    blocked = true;
  } else if (dailyCost >= dailyLimit * warnPct) {
    const pct = dailyCost / dailyLimit * 100;
    warnings.push(`⚠️ Bedrock 日次コスト警告: $${dailyCost.toFixed(4)} / $${dailyLimit.toFixed(2)} (${pct.toFixed(0)}%)`);
  }

  if (monthlyCost >= monthlyLimit) {
    process.stderr.write(`⛔ 月次コスト上限超過: $${monthlyCost.toFixed(4)} / $${monthlyLimit.toFixed(2)}\n`);
    process.stderr.write(`   上限を上げるには以下のファイルの monthly_limit_usd を編集してください:\n`);
    process.stderr.write(`   ${CONFIG_FILE}\n`);
    process.stderr.write(`   （または来月以降に再開してください）\n`);
    blocked = true;
  } else if (monthlyCost >= monthlyLimit * warnPct) {
    const pct = monthlyCost / monthlyLimit * 100;
    warnings.push(`⚠️ Bedrock 月次コスト警告: $${monthlyCost.toFixed(4)} / $${monthlyLimit.toFixed(2)} (${pct.toFixed(0)}%)`);
  }

  if (blocked) process.exit(2);

  if (warnings.length > 0) {
    console.log(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'UserPromptSubmit',
        additionalContext: warnings.join('\n'),
      },
    }));
  }

  process.exit(0);
}

async function main() {
  const config = loadConfig();
  const arg = process.argv[2];

  if (arg === '--update-pricing') {
    await cmdUpdatePricing();
    return;
  }

  if (arg === '--status') {
    const litellmPricing = await loadPricing();
    await cmdStatus(config, litellmPricing);
    return;
  }

  // hook モード: stdin から JSON を読む（UserPromptSubmit）
  try {
    fs.readFileSync(0, 'utf8'); // fd 0 = stdin
  } catch (e) {
    process.exit(0);
  }

  const litellmPricing = await loadPricing();
  await cmdCheck(config, litellmPricing);
}

main().catch(() => process.exit(0));
