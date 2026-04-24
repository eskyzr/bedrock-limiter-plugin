package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	litellmPricingURL   = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	pricingCacheTTLDays = 7
)

// ---------------------------------------------------------------------------
// 型定義
// ---------------------------------------------------------------------------

type modelPricing struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cache_write"`
	CacheRead  float64 `json:"cache_read"`
}

type appConfig struct {
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
	WarnPercent     int     `json:"warn_percent"`
	BedrockOnly     bool    `json:"bedrock_only"`
}

type pricingCache struct {
	CachedAt string                  `json:"cached_at"`
	Pricing  map[string]modelPricing `json:"pricing"`
}

type usageData struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type messageData struct {
	Model string    `json:"model"`
	Usage usageData `json:"usage"`
}

type transcriptEntry struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	RequestID string      `json:"requestId"`
	Message   messageData `json:"message"`
}

// ---------------------------------------------------------------------------
// デフォルト値
// ---------------------------------------------------------------------------

var fallbackPricing = map[string]modelPricing{
	"opus":    {Input: 5.5, Output: 27.5, CacheWrite: 6.875, CacheRead: 0.55},
	"sonnet":  {Input: 3.0, Output: 15.0, CacheWrite: 3.75, CacheRead: 0.3},
	"haiku":   {Input: 1.0, Output: 5.0, CacheWrite: 1.25, CacheRead: 0.1},
	"default": {Input: 3.0, Output: 15.0, CacheWrite: 3.75, CacheRead: 0.3},
}

var defaultConfig = appConfig{
	DailyLimitUSD:   5.0,
	MonthlyLimitUSD: 50.0,
	WarnPercent:     80,
	BedrockOnly:     true,
}

// ---------------------------------------------------------------------------
// パス解決
// ---------------------------------------------------------------------------

func pluginRoot() string {
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		return root
	}
	// go/bin/bedrock-limiter-xxx → go/bin → go → plugin root
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(exe)))
}

func configFile() string    { return filepath.Join(pluginRoot(), "config.json") }
func cachePricingFile() string { return filepath.Join(pluginRoot(), "pricing_cache.json") }

// ---------------------------------------------------------------------------
// 設定ファイル
// ---------------------------------------------------------------------------

func loadConfig() appConfig {
	data, err := os.ReadFile(configFile())
	if err == nil {
		var cfg appConfig
		if json.Unmarshal(data, &cfg) == nil {
			return cfg
		}
	}
	data, _ = json.MarshalIndent(defaultConfig, "", "  ")
	os.WriteFile(configFile(), data, 0644)
	return defaultConfig
}

// ---------------------------------------------------------------------------
// LiteLLM 料金キャッシュ
// ---------------------------------------------------------------------------

func fetchLitellmPricing() (map[string]modelPricing, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(litellmPricingURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	result := make(map[string]modelPricing)
	for key, val := range raw {
		m, ok := val.(map[string]any)
		if !ok {
			continue
		}
		if !strings.Contains(fmt.Sprint(m["litellm_provider"]), "bedrock") {
			continue
		}

		normalized := key
		for _, prefix := range []string{"us.", "eu.", "au.", "apac.", "global."} {
			if strings.HasPrefix(normalized, prefix) {
				normalized = normalized[len(prefix):]
				break
			}
		}
		if strings.HasPrefix(normalized, "anthropic.") {
			normalized = normalized[len("anthropic."):]
		}

		toPerMillion := func(k string) float64 {
			v, _ := m[k].(float64)
			return v * 1_000_000
		}
		result[normalized] = modelPricing{
			Input:      toPerMillion("input_cost_per_token"),
			Output:     toPerMillion("output_cost_per_token"),
			CacheWrite: toPerMillion("cache_creation_input_token_cost"),
			CacheRead:  toPerMillion("cache_read_input_token_cost"),
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no bedrock entries found")
	}
	return result, nil
}

func loadPricing() map[string]modelPricing {
	var stale map[string]modelPricing

	if data, err := os.ReadFile(cachePricingFile()); err == nil {
		var c pricingCache
		if json.Unmarshal(data, &c) == nil {
			if t, err := time.Parse(time.RFC3339, c.CachedAt); err == nil {
				if time.Since(t).Hours()/24 < pricingCacheTTLDays {
					return c.Pricing
				}
			}
			stale = c.Pricing
		}
	}

	if fetched, err := fetchLitellmPricing(); err == nil {
		c := pricingCache{CachedAt: time.Now().UTC().Format(time.RFC3339), Pricing: fetched}
		if data, err := json.MarshalIndent(c, "", "  "); err == nil {
			os.WriteFile(cachePricingFile(), data, 0644)
		}
		return fetched
	}

	if stale != nil {
		return stale
	}
	return map[string]modelPricing{}
}

// ---------------------------------------------------------------------------
// コスト計算
// ---------------------------------------------------------------------------

func getPrice(modelName string, pricing map[string]modelPricing) modelPricing {
	if p, ok := pricing[modelName]; ok {
		return p
	}
	for key, p := range pricing {
		if strings.HasPrefix(key, modelName) {
			return p
		}
	}
	lower := strings.ToLower(modelName)
	for _, family := range []string{"opus", "sonnet", "haiku"} {
		if strings.Contains(lower, family) {
			return fallbackPricing[family]
		}
	}
	return fallbackPricing["default"]
}

func calcEntryCost(usage usageData, model string, pricing map[string]modelPricing) float64 {
	p := getPrice(model, pricing)
	return (float64(usage.InputTokens)*p.Input +
		float64(usage.OutputTokens)*p.Output +
		float64(usage.CacheCreationInputTokens)*p.CacheWrite +
		float64(usage.CacheReadInputTokens)*p.CacheRead) / 1_000_000
}

func isBedrockEntry(e transcriptEntry) bool {
	return !strings.HasPrefix(e.RequestID, "req_")
}

func localDateStrings() (today, month string) {
	now := time.Now()
	return now.Format("2006-01-02"), now.Format("2006-01")
}

func scanCosts(today, month string, pricing map[string]modelPricing, bedrockOnly bool) (daily, monthly float64) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	projectsDir := filepath.Join(home, ".claude", "projects")
	topEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return
	}

	for _, proj := range topEntries {
		if !proj.IsDir() {
			continue
		}
		projPath := filepath.Join(projectsDir, proj.Name())
		files, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			filePath := filepath.Join(projPath, f.Name())

			info, err := f.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Format("2006-01") < month {
				continue
			}

			func() {
				file, err := os.Open(filePath)
				if err != nil {
					return
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line == "" {
						continue
					}
					var entry transcriptEntry
					if err := json.Unmarshal([]byte(line), &entry); err != nil {
						continue
					}
					if entry.Type != "assistant" {
						continue
					}
					if !strings.HasPrefix(entry.Timestamp, month) {
						continue
					}
					if bedrockOnly && !isBedrockEntry(entry) {
						continue
					}
					u := entry.Message.Usage
					if u.InputTokens == 0 && u.OutputTokens == 0 &&
						u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
						continue
					}
					cost := calcEntryCost(u, entry.Message.Model, pricing)
					monthly += cost
					if strings.HasPrefix(entry.Timestamp, today) {
						daily += cost
					}
				}
			}()
		}
	}
	return
}

// ---------------------------------------------------------------------------
// コマンド
// ---------------------------------------------------------------------------

func buildBar(ratio float64, width int) string {
	filled := int(ratio * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func cmdUpdatePricing() {
	fmt.Print("料金データを取得中... ")
	fetched, err := fetchLitellmPricing()
	if err != nil {
		fmt.Println("失敗")
		fmt.Fprintln(os.Stderr, "ネットワークに接続できないか、URL が変更された可能性があります。")
		os.Exit(1)
	}
	c := pricingCache{CachedAt: time.Now().UTC().Format(time.RFC3339), Pricing: fetched}
	data, _ := json.MarshalIndent(c, "", "  ")
	os.WriteFile(cachePricingFile(), data, 0644)
	fmt.Printf("完了 (%d モデル)\n", len(fetched))
	fmt.Printf("キャッシュ: %s\n", cachePricingFile())
}

func cmdStatus(cfg appConfig, pricing map[string]modelPricing) {
	today, month := localDateStrings()
	daily, monthly := scanCosts(today, month, pricing, cfg.BedrockOnly)

	dailyRatio, monthlyRatio := 0.0, 0.0
	if cfg.DailyLimitUSD > 0 {
		dailyRatio = daily / cfg.DailyLimitUSD
	}
	if cfg.MonthlyLimitUSD > 0 {
		monthlyRatio = monthly / cfg.MonthlyLimitUSD
	}

	modeLabel := "Bedrock のみ"
	if !cfg.BedrockOnly {
		modeLabel = "全プロバイダ"
	}
	cacheLabel := "フォールバック"
	if _, err := os.Stat(cachePricingFile()); err == nil {
		cacheLabel = "LiteLLM キャッシュ"
	}

	fmt.Printf("集計対象: %s  |  料金データ: %s\n", modeLabel, cacheLabel)
	fmt.Printf("今日の使用コスト:  $%.4f / $%.2f (%.1f%%)  [%s]\n",
		daily, cfg.DailyLimitUSD, dailyRatio*100, buildBar(dailyRatio, 20))
	fmt.Printf("今月の使用コスト:  $%.4f / $%.2f (%.1f%%) [%s]\n",
		monthly, cfg.MonthlyLimitUSD, monthlyRatio*100, buildBar(monthlyRatio, 20))
	fmt.Printf("\n設定ファイル: %s\n", configFile())
	fmt.Printf("料金更新:     %s --update-pricing\n", os.Args[0])
}

func cmdCheck(cfg appConfig, pricing map[string]modelPricing) {
	today, month := localDateStrings()
	daily, monthly := scanCosts(today, month, pricing, cfg.BedrockOnly)
	warnPct := float64(cfg.WarnPercent) / 100.0

	var warnings []string
	blocked := false

	if daily >= cfg.DailyLimitUSD {
		fmt.Fprintf(os.Stderr, "⛔ 日次コスト上限超過: $%.4f / $%.2f\n", daily, cfg.DailyLimitUSD)
		fmt.Fprintf(os.Stderr, "   上限を上げるには以下のファイルの daily_limit_usd を編集してください:\n")
		fmt.Fprintf(os.Stderr, "   %s\n", configFile())
		fmt.Fprintf(os.Stderr, "   （または明日以降に再開してください）\n")
		blocked = true
	} else if daily >= cfg.DailyLimitUSD*warnPct {
		warnings = append(warnings, fmt.Sprintf(
			"⚠️ Bedrock 日次コスト警告: $%.4f / $%.2f (%.0f%%)",
			daily, cfg.DailyLimitUSD, daily/cfg.DailyLimitUSD*100))
	}

	if monthly >= cfg.MonthlyLimitUSD {
		fmt.Fprintf(os.Stderr, "⛔ 月次コスト上限超過: $%.4f / $%.2f\n", monthly, cfg.MonthlyLimitUSD)
		fmt.Fprintf(os.Stderr, "   上限を上げるには以下のファイルの monthly_limit_usd を編集してください:\n")
		fmt.Fprintf(os.Stderr, "   %s\n", configFile())
		fmt.Fprintf(os.Stderr, "   （または来月以降に再開してください）\n")
		blocked = true
	} else if monthly >= cfg.MonthlyLimitUSD*warnPct {
		warnings = append(warnings, fmt.Sprintf(
			"⚠️ Bedrock 月次コスト警告: $%.4f / $%.2f (%.0f%%)",
			monthly, cfg.MonthlyLimitUSD, monthly/cfg.MonthlyLimitUSD*100))
	}

	if blocked {
		os.Exit(2)
	}

	if len(warnings) > 0 {
		output := map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "UserPromptSubmit",
				"additionalContext": strings.Join(warnings, "\n"),
			},
		}
		data, _ := json.Marshal(output)
		fmt.Println(string(data))
	}

	os.Exit(0)
}

// ---------------------------------------------------------------------------
// エントリポイント
// ---------------------------------------------------------------------------

func main() {
	cfg := loadConfig()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--update-pricing":
			cmdUpdatePricing()
			return
		case "--status":
			cmdStatus(cfg, loadPricing())
			return
		}
	}

	// hook モード: stdin から JSON を読む（UserPromptSubmit）
	if _, err := io.ReadAll(os.Stdin); err != nil {
		os.Exit(0)
	}

	cmdCheck(cfg, loadPricing())
}
