package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oak/crypto-trading-bot/internal/agents"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/executors"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init(cfg.DebugMode)
	log := logger.Global

	log.Header("加密货币交易机器人 - Go 版本 (Eino Graph)", '=', 80)
	log.Info(fmt.Sprintf("交易对: %s", cfg.CryptoSymbol))
	log.Info(fmt.Sprintf("时间周期: %s", cfg.CryptoTimeframe))
	log.Info(fmt.Sprintf("回看天数: %d", cfg.CryptoLookbackDays))
	log.Info(fmt.Sprintf("杠杆倍数: %dx", cfg.BinanceLeverage))

	if cfg.BinanceTestMode {
		log.Success("🟢 运行模式: 测试模式（模拟交易）")
	} else {
		log.Warning("🔴 运行模式: 实盘模式（真实交易！）")
	}

	// Initialize executor
	executor := executors.NewBinanceExecutor(cfg, log)

	// Initialize storage
	log.Subheader("初始化数据库", '─', 80)

	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Error(fmt.Sprintf("创建数据库目录失败: %v", err))
		os.Exit(1)
	}

	db, err := storage.NewStorage(cfg.DatabasePath)
	if err != nil {
		log.Error(fmt.Sprintf("初始化数据库失败: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	log.Success(fmt.Sprintf("数据库已连接: %s", cfg.DatabasePath))

	// Display statistics
	stats, err := db.GetSessionStats(cfg.CryptoSymbol)
	if err != nil {
		log.Warning(fmt.Sprintf("获取历史统计失败: %v", err))
	} else if stats["total_sessions"].(int) > 0 {
		log.Info(fmt.Sprintf("历史会话总数: %d", stats["total_sessions"].(int)))
		log.Info(fmt.Sprintf("已执行交易数: %d", stats["executed_count"].(int)))
		log.Info(fmt.Sprintf("执行率: %.1f%%", stats["execution_rate"].(float64)))
	}

	ctx := context.Background()

	// Setup exchange
	log.Subheader("设置交易所参数", '─', 80)
	if err := executor.SetupExchange(ctx, cfg.CryptoSymbol, cfg.BinanceLeverage); err != nil {
		log.Error(fmt.Sprintf("设置交易所失败: %v", err))
		os.Exit(1)
	}

	// Create and run the trading graph workflow
	log.Subheader("初始化 Eino Graph 工作流", '─', 80)
	log.Info("创建多智能体分析系统...")
	log.Info("  • 市场分析师 (Market Analyst)")
	log.Info("  • 加密货币分析师 (Crypto Analyst)")
	log.Info("  • 情绪分析师 (Sentiment Analyst)")
	log.Info("  • 交易员 (Trader)")
	log.Info("")

	tradingGraph := agents.NewSimpleTradingGraph(cfg, log, executor)

	// Run the graph workflow
	result, err := tradingGraph.Run(ctx)
	if err != nil {
		log.Error(fmt.Sprintf("工作流执行失败: %v", err))
		os.Exit(1)
	}

	// Display final results
	log.Subheader("工作流执行结果", '─', 80)

	var decision string
	if d, ok := result["decision"].(string); ok {
		decision = d
		log.Info("最终交易决策:")
		log.Info(decision)
	}

	// Display agent state
	state := tradingGraph.GetState()
	log.Subheader("分析师报告摘要", '─', 80)
	log.Info(fmt.Sprintf("✅ 市场分析: %d 字符", len(state.MarketReport)))
	log.Info(fmt.Sprintf("✅ 加密货币分析: %d 字符", len(state.CryptoReport)))
	log.Info(fmt.Sprintf("✅ 情绪分析: %d 字符", len(state.SentimentReport)))
	log.Info(fmt.Sprintf("✅ 持仓信息: %d 字符", len(state.PositionInfo)))

	// Save session to database
	log.Subheader("保存分析结果", '─', 80)
	session := &storage.TradingSession{
		Symbol:          cfg.CryptoSymbol,
		Timeframe:       cfg.CryptoTimeframe,
		CreatedAt:       time.Now(),
		MarketReport:    state.MarketReport,
		CryptoReport:    state.CryptoReport,
		SentimentReport: state.SentimentReport,
		PositionInfo:    state.PositionInfo,
		Decision:        decision,
		Executed:        false,
		ExecutionResult: "",
	}

	sessionID, err := db.SaveSession(session)
	if err != nil {
		log.Error(fmt.Sprintf("保存会话失败: %v", err))
	} else {
		log.Success(fmt.Sprintf("会话已保存到数据库 (ID: %d)", sessionID))
		log.Info(fmt.Sprintf("数据库路径: %s", cfg.DatabasePath))
	}

	log.Header("执行完成", '=', 80)
	log.Success("Eino Graph 工作流执行成功！")
	log.Info("")
	log.Info("📊 已实现的功能:")
	log.Info("  ✅ 配置管理系统 (Viper)")
	log.Info("  ✅ 彩色日志系统 (Zerolog)")
	log.Info("  ✅ Binance API 客户端封装")
	log.Info("  ✅ OHLCV 数据获取")
	log.Info("  ✅ 技术指标计算 (RSI, MACD, BB, SMA, EMA, ATR)")
	log.Info("  ✅ 资金费率、订单簿、24h统计")
	log.Info("  ✅ 市场情绪分析 (CryptoOracle)")
	log.Info("  ✅ Eino Graph 工作流编排")
	log.Info("  ✅ 并行执行优化 (市场+情绪并行)")
	log.Info("  ✅ 4个分析师 Agent 系统")
	log.Info("  ✅ 币安期货执行器")
	log.Info("  ✅ 调度器系统")
	log.Info("  ✅ SQLite 结果存储")
	log.Info("")
	log.Info("⏳ 待实现的功能:")
	log.Info("  🔲 LLM 集成 (OpenAI API)")
	log.Info("  🔲 Web 监控界面 (Hertz)")
	log.Info("  🔲 完整测试套件")
}