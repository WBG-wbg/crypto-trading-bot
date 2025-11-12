package config

import (
	"fmt"
	"github.com/oak/crypto-trading-bot/internal/constant"
	"github.com/spf13/viper"
	"os"
	"strings"
)

// Config holds all configuration for the crypto trading bot
type Config struct {
	// Project paths
	ProjectDir   string
	ResultsDir   string
	DataCacheDir string
	DatabasePath string

	// LLM Configuration
	LLMProvider      string
	DeepThinkLLM     string
	QuickThinkLLM    string
	BackendURL       string
	APIKey           string
	TraderPromptPath string // 交易策略 Prompt 文件路径 / Path to trader strategy prompt file

	// Agent behavior
	MaxDebateRounds      int
	MaxRiskDiscussRounds int
	MaxRecurLimit        int

	// Data vendors
	DataVendorStock      string
	DataVendorIndicators string
	DataVendorNews       string
	DataVendorCrypto     string

	// Binance trading configuration
	// 币安交易配置
	BinanceAPIKey               string
	BinanceAPISecret            string
	BinanceProxy                string
	BinanceProxyInsecureSkipTLS bool // 是否跳过代理 TLS 验证（某些代理需要）/ Skip TLS verification for proxy (required by some proxies)
	BinanceLeverage             int  // 固定杠杆（向后兼容）/ Fixed leverage (backward compatible)
	BinanceLeverageMin          int  // 最小杠杆 / Minimum leverage
	BinanceLeverageMax          int  // 最大杠杆 / Maximum leverage
	BinanceLeverageDynamic      bool // 是否启用动态杠杆 / Enable dynamic leverage
	BinanceTestMode             bool
	BinancePositionMode         string

	// Trading parameters
	// 交易参数
	CryptoSymbols      []string // 交易对列表（支持单个或多个，用逗号分隔）/ Trading pairs list (supports single or multiple, comma-separated)
	CryptoTimeframe    string
	CryptoLookbackDays int
	PositionSize       float64
	MaxPositionSize    float64

	// Stop-loss management configuration
	// 止损管理配置
	StopLossStrategy         string  // fixed, breakeven, trailing / 止损策略
	EnableBreakeven          bool    // 是否启用保本 / Enable breakeven
	BreakevenTrigger         float64 // 保本触发点（盈亏比）/ Breakeven trigger ratio
	EnableTrailing           bool    // 是否启用追踪止损 / Enable trailing stop
	TrailingTrigger          float64 // 追踪止损触发点（盈亏比）/ Trailing trigger ratio
	TrailingDistanceInitial  float64 // 初始追踪距离 / Initial trailing distance
	TrailingDistanceTight    float64 // 收紧后的距离 / Tightened distance
	TrailingTightenProfit    float64 // 收紧触发利润 / Profit to tighten
	EnablePartialTakeProfit  bool    // 是否启用分批止盈 / Enable partial TP
	PartialTakeProfitRatio   float64 // 分批止盈比例 / Partial TP ratio
	PartialTakeProfitTrigger float64 // 分批止盈触发点 / Partial TP trigger

	// Memory system
	UseMemory  bool
	MemoryTopK int

	// Debug options
	DebugMode        bool
	SelectedAnalysts []string
	AutoExecute      bool

	// Web monitoring
	WebPort int
}

// LoadConfig loads configuration from .env file or a custom path
// LoadConfig 从 .env 文件或自定义路径加载配置
func LoadConfig(pathToEnv string) (*Config, error) {
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	// Determine which config file to load
	configPath := ".env" // default path / 默认路径
	if pathToEnv != constant.BlankStr {
		configPath = pathToEnv
	}

	viper.SetConfigFile(configPath)

	// Attempt to read config file, but don't fail if it doesn't exist
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file from %s: %w", configPath, err)
		}
	}

	// Set defaults
	setDefaults()

	cfg := &Config{
		// Project paths
		ProjectDir:   getProjectDir(),
		ResultsDir:   viper.GetString("RESULTS_DIR"),
		DataCacheDir: viper.GetString("DATA_CACHE_DIR"),
		DatabasePath: viper.GetString("DATABASE_PATH"),

		// LLM Configuration
		LLMProvider:      viper.GetString("LLM_PROVIDER"),
		DeepThinkLLM:     viper.GetString("DEEP_THINK_LLM"),
		QuickThinkLLM:    viper.GetString("QUICK_THINK_LLM"),
		BackendURL:       viper.GetString("LLM_BACKEND_URL"),
		APIKey:           viper.GetString("OPENAI_API_KEY"),
		TraderPromptPath: viper.GetString("TRADER_PROMPT_PATH"),

		// Agent behavior
		MaxDebateRounds:      viper.GetInt("MAX_DEBATE_ROUNDS"),
		MaxRiskDiscussRounds: viper.GetInt("MAX_RISK_DISCUSS_ROUNDS"),
		MaxRecurLimit:        viper.GetInt("MAX_RECUR_LIMIT"),

		// Data vendors
		DataVendorStock:      viper.GetString("DATA_VENDOR_STOCK"),
		DataVendorIndicators: viper.GetString("DATA_VENDOR_INDICATORS"),
		DataVendorNews:       viper.GetString("DATA_VENDOR_NEWS"),
		DataVendorCrypto:     viper.GetString("DATA_VENDOR_CRYPTO"),

		// Binance trading configuration
		BinanceAPIKey:               viper.GetString("BINANCE_API_KEY"),
		BinanceAPISecret:            viper.GetString("BINANCE_API_SECRET"),
		BinanceProxy:                viper.GetString("BINANCE_PROXY"),
		BinanceProxyInsecureSkipTLS: viper.GetBool("BINANCE_PROXY_INSECURE_SKIP_TLS"),
		BinanceLeverage:             viper.GetInt("BINANCE_LEVERAGE"),
		BinanceTestMode:             viper.GetBool("BINANCE_TEST_MODE"),
		BinancePositionMode:         viper.GetString("BINANCE_POSITION_MODE"),

		// Trading parameters
		CryptoTimeframe:    viper.GetString("CRYPTO_TIMEFRAME"),
		CryptoLookbackDays: viper.GetInt("CRYPTO_LOOKBACK_DAYS"),
		PositionSize:       viper.GetFloat64("POSITION_SIZE"),
		MaxPositionSize:    viper.GetFloat64("MAX_POSITION_SIZE"),

		// Stop-loss management
		StopLossStrategy:         viper.GetString("STOPLOSS_STRATEGY"),
		EnableBreakeven:          viper.GetBool("STOPLOSS_ENABLE_BREAKEVEN"),
		BreakevenTrigger:         viper.GetFloat64("STOPLOSS_BREAKEVEN_TRIGGER"),
		EnableTrailing:           viper.GetBool("STOPLOSS_ENABLE_TRAILING"),
		TrailingTrigger:          viper.GetFloat64("STOPLOSS_TRAILING_TRIGGER"),
		TrailingDistanceInitial:  viper.GetFloat64("STOPLOSS_TRAILING_DISTANCE_INITIAL"),
		TrailingDistanceTight:    viper.GetFloat64("STOPLOSS_TRAILING_DISTANCE_TIGHT"),
		TrailingTightenProfit:    viper.GetFloat64("STOPLOSS_TRAILING_TIGHTEN_PROFIT"),
		EnablePartialTakeProfit:  viper.GetBool("STOPLOSS_ENABLE_PARTIAL_TP"),
		PartialTakeProfitRatio:   viper.GetFloat64("STOPLOSS_PARTIAL_TP_RATIO"),
		PartialTakeProfitTrigger: viper.GetFloat64("STOPLOSS_PARTIAL_TP_TRIGGER"),

		// Memory system
		UseMemory:  viper.GetBool("USE_MEMORY"),
		MemoryTopK: viper.GetInt("MEMORY_TOP_K"),

		// Debug options
		DebugMode:        viper.GetBool("DEBUG_MODE"),
		SelectedAnalysts: strings.Split(viper.GetString("SELECTED_ANALYSTS"), ","),
		AutoExecute:      viper.GetBool("AUTO_EXECUTE"),

		// Web monitoring
		WebPort: viper.GetInt("WEB_PORT"),
	}

	// Auto-calculate lookback days if not set
	// 如果未设置回看天数，自动计算
	if cfg.CryptoLookbackDays == 0 {
		cfg.CryptoLookbackDays = calculateLookbackDays(cfg.CryptoTimeframe)
	}

	// Parse crypto symbols (supports single or multiple, comma-separated)
	// 解析加密货币交易对（支持单个或多个，用逗号分隔）
	symbolsStr := viper.GetString("CRYPTO_SYMBOLS")
	if symbolsStr != "" {
		cfg.CryptoSymbols = strings.Split(symbolsStr, ",")
		// Trim spaces from each symbol
		// 去除每个交易对的空格
		for i := range cfg.CryptoSymbols {
			cfg.CryptoSymbols[i] = strings.TrimSpace(cfg.CryptoSymbols[i])
		}
	} else {
		// Default to BTC/USDT if not specified
		// 如果未指定，默认使用 BTC/USDT
		cfg.CryptoSymbols = []string{"BTC/USDT"}
	}

	// Parse leverage range (support "10-20" format)
	// 解析杠杆范围（支持 "10-20" 格式）
	leverageStr := viper.GetString("BINANCE_LEVERAGE")
	if strings.Contains(leverageStr, "-") {
		// Dynamic leverage: parse min and max
		// 动态杠杆：解析最小值和最大值
		parts := strings.Split(leverageStr, "-")
		if len(parts) == 2 {
			minLev := 0
			maxLev := 0
			fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &minLev)
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &maxLev)

			if minLev > 0 && maxLev > 0 && minLev <= maxLev && maxLev <= 125 {
				cfg.BinanceLeverageMin = minLev
				cfg.BinanceLeverageMax = maxLev
				cfg.BinanceLeverageDynamic = true
				cfg.BinanceLeverage = minLev // Default to min for safety
			} else {
				// Invalid range, fallback to default
				// 无效范围，回退到默认值
				cfg.BinanceLeverage = 10
				cfg.BinanceLeverageMin = 10
				cfg.BinanceLeverageMax = 10
				cfg.BinanceLeverageDynamic = false
			}
		}
	} else {
		// Fixed leverage
		// 固定杠杆
		cfg.BinanceLeverageMin = cfg.BinanceLeverage
		cfg.BinanceLeverageMax = cfg.BinanceLeverage
		cfg.BinanceLeverageDynamic = false
	}

	return cfg, nil
}

func setDefaults() {
	viper.SetDefault("RESULTS_DIR", "./crypto_results")
	viper.SetDefault("DATA_CACHE_DIR", "./internal/dataflows/data_cache")
	viper.SetDefault("DATABASE_PATH", "./data/trading.db")

	viper.SetDefault("LLM_PROVIDER", "openai")
	viper.SetDefault("DEEP_THINK_LLM", "gpt-4o")
	viper.SetDefault("QUICK_THINK_LLM", "gpt-4o-mini")
	viper.SetDefault("LLM_BACKEND_URL", "https://api.openai.com/v1")
	viper.SetDefault("TRADER_PROMPT_PATH", "prompts/trader_system.txt")

	viper.SetDefault("MAX_DEBATE_ROUNDS", 2)
	viper.SetDefault("MAX_RISK_DISCUSS_ROUNDS", 2)
	viper.SetDefault("MAX_RECUR_LIMIT", 100)

	viper.SetDefault("DATA_VENDOR_STOCK", "ccxt")
	viper.SetDefault("DATA_VENDOR_INDICATORS", "ccxt")
	viper.SetDefault("DATA_VENDOR_NEWS", "alpha_vantage")
	viper.SetDefault("DATA_VENDOR_CRYPTO", "ccxt")

	viper.SetDefault("BINANCE_LEVERAGE", 10)
	viper.SetDefault("BINANCE_TEST_MODE", true)
	viper.SetDefault("BINANCE_POSITION_MODE", "auto")

	viper.SetDefault("CRYPTO_SYMBOL", "BTC/USDT")
	viper.SetDefault("CRYPTO_TIMEFRAME", "1h")
	viper.SetDefault("POSITION_SIZE", 0.001)
	viper.SetDefault("MAX_POSITION_SIZE", 0.01)

	// Stop-loss management defaults (based on trading philosophy)
	// 止损管理默认值（基于交易哲学）
	viper.SetDefault("STOPLOSS_STRATEGY", "trailing")            // trailing (推荐), breakeven, fixed
	viper.SetDefault("STOPLOSS_ENABLE_BREAKEVEN", true)          // 启用保本 / Enable breakeven
	viper.SetDefault("STOPLOSS_BREAKEVEN_TRIGGER", 0.025)        // 2.5% 利润时保本 / Breakeven at 2.5% profit (1:1 risk/reward)
	viper.SetDefault("STOPLOSS_ENABLE_TRAILING", true)           // 启用追踪止损 / Enable trailing
	viper.SetDefault("STOPLOSS_TRAILING_TRIGGER", 0.05)          // 5% 利润启动追踪 / Start trailing at 5% profit (2:1 risk/reward)
	viper.SetDefault("STOPLOSS_TRAILING_DISTANCE_INITIAL", 0.03) // 初始追踪距离 3% / Initial trailing distance
	viper.SetDefault("STOPLOSS_TRAILING_DISTANCE_TIGHT", 0.02)   // 收紧到 2% / Tighten to 2%
	viper.SetDefault("STOPLOSS_TRAILING_TIGHTEN_PROFIT", 0.10)   // 10% 利润时收紧 / Tighten at 10% profit
	viper.SetDefault("STOPLOSS_ENABLE_PARTIAL_TP", false)        // 不推荐分批止盈 / Partial TP not recommended
	viper.SetDefault("STOPLOSS_PARTIAL_TP_RATIO", 0.3)           // 平 30% 仓位 / Close 30% of position
	viper.SetDefault("STOPLOSS_PARTIAL_TP_TRIGGER", 0.075)       // 7.5% 利润触发 / Trigger at 7.5% profit (3:1 risk/reward)

	viper.SetDefault("USE_MEMORY", true)
	viper.SetDefault("MEMORY_TOP_K", 3)

	viper.SetDefault("DEBUG_MODE", false)
	viper.SetDefault("SELECTED_ANALYSTS", "market,crypto,sentiment")
	viper.SetDefault("AUTO_EXECUTE", false)

	viper.SetDefault("WEB_PORT", 8080)
}

func getProjectDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// calculateLookbackDays returns optimal lookback days based on timeframe
func calculateLookbackDays(timeframe string) int {
	switch timeframe {
	case "15m":
		return 5 // ~480 candles
	case "1h":
		return 10 // ~240 candles
	case "4h":
		return 15 // ~90 candles
	case "1d":
		return 60 // ~60 candles
	default:
		return 10
	}
}

// GetBinanceSymbolFor converts a specific symbol format from "BTC/USDT" to "BTCUSDT"
// GetBinanceSymbolFor 将特定交易对格式从 "BTC/USDT" 转换为 "BTCUSDT"
func (c *Config) GetBinanceSymbolFor(symbol string) string {
	return strings.ReplaceAll(symbol, "/", "")
}

// GetAllBinanceSymbols returns all trading pairs in Binance format
// GetAllBinanceSymbols 返回所有交易对的币安格式
func (c *Config) GetAllBinanceSymbols() []string {
	symbols := make([]string, len(c.CryptoSymbols))
	for i, symbol := range c.CryptoSymbols {
		symbols[i] = strings.ReplaceAll(symbol, "/", "")
	}
	return symbols
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required")
	}

	if c.BinanceAPIKey == "" || c.BinanceAPISecret == "" {
		return fmt.Errorf("BINANCE_API_KEY and BINANCE_API_SECRET are required")
	}

	if c.PositionSize <= 0 {
		return fmt.Errorf("POSITION_SIZE must be greater than 0")
	}

	if c.MaxPositionSize < c.PositionSize {
		return fmt.Errorf("MAX_POSITION_SIZE must be >= POSITION_SIZE")
	}

	return nil
}
