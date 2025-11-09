package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the crypto trading bot
type Config struct {
	// Project paths
	ProjectDir   string
	ResultsDir   string
	DataCacheDir string
	DatabasePath string

	// LLM Configuration
	LLMProvider    string
	DeepThinkLLM   string
	QuickThinkLLM  string
	BackendURL     string
	APIKey         string

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
	BinanceAPIKey      string
	BinanceAPISecret   string
	BinanceProxy       string
	BinanceLeverage    int
	BinanceTestMode    bool
	BinancePositionMode string

	// Trading parameters
	CryptoSymbol      string
	CryptoTimeframe   string
	CryptoLookbackDays int
	PositionSize      float64
	MaxPositionSize   float64

	// Risk management
	MaxDrawdown         float64
	RiskPerTrade        float64
	VolatilityMultiplier float64

	// Memory system
	UseMemory   bool
	MemoryTopK  int

	// Debug options
	DebugMode        bool
	SelectedAnalysts []string
	AutoExecute      bool

	// Web monitoring
	WebPort int
}

// LoadConfig loads configuration from .env file
func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	// Attempt to read config file, but don't fail if it doesn't exist
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
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
		LLMProvider:   viper.GetString("LLM_PROVIDER"),
		DeepThinkLLM:  viper.GetString("DEEP_THINK_LLM"),
		QuickThinkLLM: viper.GetString("QUICK_THINK_LLM"),
		BackendURL:    viper.GetString("LLM_BACKEND_URL"),
		APIKey:        viper.GetString("OPENAI_API_KEY"),

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
		BinanceAPIKey:       viper.GetString("BINANCE_API_KEY"),
		BinanceAPISecret:    viper.GetString("BINANCE_API_SECRET"),
		BinanceProxy:        viper.GetString("BINANCE_PROXY"),
		BinanceLeverage:     viper.GetInt("BINANCE_LEVERAGE"),
		BinanceTestMode:     viper.GetBool("BINANCE_TEST_MODE"),
		BinancePositionMode: viper.GetString("BINANCE_POSITION_MODE"),

		// Trading parameters
		CryptoSymbol:       viper.GetString("CRYPTO_SYMBOL"),
		CryptoTimeframe:    viper.GetString("CRYPTO_TIMEFRAME"),
		CryptoLookbackDays: viper.GetInt("CRYPTO_LOOKBACK_DAYS"),
		PositionSize:       viper.GetFloat64("POSITION_SIZE"),
		MaxPositionSize:    viper.GetFloat64("MAX_POSITION_SIZE"),

		// Risk management
		MaxDrawdown:          viper.GetFloat64("MAX_DRAWDOWN"),
		RiskPerTrade:         viper.GetFloat64("RISK_PER_TRADE"),
		VolatilityMultiplier: viper.GetFloat64("VOLATILITY_MULTIPLIER"),

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
	if cfg.CryptoLookbackDays == 0 {
		cfg.CryptoLookbackDays = calculateLookbackDays(cfg.CryptoTimeframe)
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

	viper.SetDefault("MAX_DRAWDOWN", 0.15)
	viper.SetDefault("RISK_PER_TRADE", 0.02)
	viper.SetDefault("VOLATILITY_MULTIPLIER", 1.5)

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

// GetBinanceSymbol converts symbol format from "BTC/USDT" to "BTCUSDT"
func (c *Config) GetBinanceSymbol() string {
	return strings.ReplaceAll(c.CryptoSymbol, "/", "")
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