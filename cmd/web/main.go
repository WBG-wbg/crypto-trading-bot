package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/oak/crypto-trading-bot/internal/agents"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/executors"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/scheduler"
	"github.com/oak/crypto-trading-bot/internal/storage"
	"github.com/oak/crypto-trading-bot/internal/web"
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

	log.Header("加密货币交易机器人 - Web 监控模式", '=', 80)
	log.Info(fmt.Sprintf("交易对: %s", cfg.CryptoSymbol))
	log.Info(fmt.Sprintf("时间周期: %s", cfg.CryptoTimeframe))
	log.Info(fmt.Sprintf("Web 端口: %d", cfg.WebPort))

	if cfg.BinanceTestMode {
		log.Success("🟢 运行模式: 测试模式（模拟交易）")
	} else {
		log.Warning("🔴 运行模式: 实盘模式（真实交易！）")
	}

	// Initialize executor
	executor := executors.NewBinanceExecutor(cfg, log)

	// Initialize storage
	log.Subheader("初始化数据库", '─', 80)
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

	// Setup exchange
	ctx := context.Background()
	log.Subheader("设置交易所参数", '─', 80)
	if err := executor.SetupExchange(ctx, cfg.CryptoSymbol, cfg.BinanceLeverage); err != nil {
		log.Error(fmt.Sprintf("设置交易所失败: %v", err))
		os.Exit(1)
	}

	// Start web server
	webServer := web.NewServer(cfg, log, db)
	go func() {
		if err := webServer.Start(); err != nil {
			log.Error(fmt.Sprintf("Web 服务器启动失败: %v", err))
		}
	}()

	// Initialize scheduler
	tradingScheduler, err := scheduler.NewTradingScheduler(cfg.CryptoTimeframe)
	if err != nil {
		log.Error(fmt.Sprintf("调度器初始化失败: %v", err))
		os.Exit(1)
	}

	log.Success(fmt.Sprintf("调度器已初始化 (时间周期: %s)", cfg.CryptoTimeframe))
	log.Info(fmt.Sprintf("下一次分析时间: %s", tradingScheduler.GetNextTimeframeTime().Format("2006-01-02 15:04:05")))
	log.Info("")
	log.Info("按 Ctrl+C 停止程序")
	log.Header("开始循环执行", '=', 80)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Trading loop
	runCount := 0
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			log.Warning("\n收到停止信号，正在关闭...")
			if err := webServer.Stop(ctx); err != nil {
				log.Warning(fmt.Sprintf("Web 服务器停止失败: %v", err))
			}
			return

		case <-ticker.C:
			// Check if it's time to run
			if tradingScheduler.IsOnTimeframe() {
				runCount++
				log.Header(fmt.Sprintf("第 %d 次执行", runCount), '=', 80)
				log.Info(fmt.Sprintf("执行时间: %s", time.Now().Format("2006-01-02 15:04:05")))

				// Run trading analysis
				if err := runTradingAnalysis(ctx, cfg, log, executor, db); err != nil {
					log.Error(fmt.Sprintf("交易分析失败: %v", err))
				}

				// Calculate next run time
				nextTime := tradingScheduler.GetNextTimeframeTime()
				log.Info(fmt.Sprintf("下次执行时间: %s", nextTime.Format("2006-01-02 15:04:05")))
				log.Header("等待下一次执行", '=', 80)
			}
		}
	}
}

func runTradingAnalysis(ctx context.Context, cfg *config.Config, log *logger.ColorLogger, executor *executors.BinanceExecutor, db *storage.Storage) error {
	// Create trading graph
	tradingGraph := agents.NewSimpleTradingGraph(cfg, log, executor)

	// Run the graph workflow
	result, err := tradingGraph.Run(ctx)
	if err != nil {
		return fmt.Errorf("工作流执行失败: %w", err)
	}

	// Get decision
	var decision string
	if d, ok := result["decision"].(string); ok {
		decision = d
	}

	// Get agent state
	state := tradingGraph.GetState()

	// Save session to database
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
		log.Warning(fmt.Sprintf("保存会话失败: %v", err))
	} else {
		log.Success(fmt.Sprintf("会话已保存 (ID: %d)", sessionID))
	}

	log.Success("✅ 本次执行完成")
	return nil
}
