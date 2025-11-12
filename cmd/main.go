package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oak/crypto-trading-bot/internal/agents"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/constant"
	"github.com/oak/crypto-trading-bot/internal/executors"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/portfolio"
	"github.com/oak/crypto-trading-bot/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig(constant.BlankStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init(cfg.DebugMode)
	log := logger.Global

	log.Header("加密货币交易机器人 - Go 版本 (Eino Graph)", '=', 80)
	log.Info(fmt.Sprintf("交易对: %v", cfg.CryptoSymbols))
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

	// Display statistics for all symbols
	// 显示所有交易对的统计信息
	for _, symbol := range cfg.CryptoSymbols {
		stats, err := db.GetSessionStats(symbol)
		if err != nil {
			log.Warning(fmt.Sprintf("获取 %s 历史统计失败: %v", symbol, err))
		} else if stats["total_sessions"].(int) > 0 {
			log.Info(fmt.Sprintf("【%s】历史会话: %d, 已执行: %d, 执行率: %.1f%%",
				symbol,
				stats["total_sessions"].(int),
				stats["executed_count"].(int),
				stats["execution_rate"].(float64)))
		}
	}

	ctx := context.Background()

	// Setup exchange for all symbols
	// 为所有交易对设置交易所参数
	log.Subheader("设置交易所参数", '─', 80)
	for _, symbol := range cfg.CryptoSymbols {
		if err := executor.SetupExchange(ctx, symbol, cfg.BinanceLeverage); err != nil {
			log.Error(fmt.Sprintf("设置 %s 交易所失败: %v", symbol, err))
			os.Exit(1)
		}
		log.Success(fmt.Sprintf("✅ %s 交易所设置完成", symbol))
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

	// ! 启动交易员分析流程
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
	for _, symbol := range cfg.CryptoSymbols {
		reports := state.GetSymbolReports(symbol)
		if reports != nil {
			log.Info(fmt.Sprintf("【%s】", symbol))
			log.Info(fmt.Sprintf("  ✅ 市场分析: %d 字符", len(reports.MarketReport)))
			log.Info(fmt.Sprintf("  ✅ 加密货币分析: %d 字符", len(reports.CryptoReport)))
			log.Info(fmt.Sprintf("  ✅ 情绪分析: %d 字符", len(reports.SentimentReport)))
			log.Info(fmt.Sprintf("  ✅ 持仓信息: %d 字符", len(reports.PositionInfo)))
		}
	}

	// Save session to database for each symbol
	// 为每个交易对保存分析结果到数据库
	log.Subheader("保存分析结果", '─', 80)
	for _, symbol := range cfg.CryptoSymbols {
		reports := state.GetSymbolReports(symbol)
		if reports == nil {
			continue
		}

		session := &storage.TradingSession{
			Symbol:          symbol,
			Timeframe:       cfg.CryptoTimeframe,
			CreatedAt:       time.Now(),
			MarketReport:    reports.MarketReport,
			CryptoReport:    reports.CryptoReport,
			SentimentReport: reports.SentimentReport,
			PositionInfo:    reports.PositionInfo,
			Decision:        decision, // 所有交易对共享同一个综合决策 / All symbols share the same comprehensive decision
			Executed:        false,
			ExecutionResult: "",
		}

		sessionID, err := db.SaveSession(session)
		if err != nil {
			log.Error(fmt.Sprintf("保存 %s 会话失败: %v", symbol, err))
		} else {
			log.Success(fmt.Sprintf("【%s】会话已保存到数据库 (ID: %d)", symbol, sessionID))
		}
	}
	log.Info(fmt.Sprintf("数据库路径: %s", cfg.DatabasePath))

	// Auto-execution logic
	// 自动执行交易逻辑
	if cfg.AutoExecute {
		log.Subheader("自动执行交易", '─', 80)
		log.Info("🚀 自动执行模式已启用")

		// Parse multi-currency decision
		// 解析多币种决策
		decisions := agents.ParseMultiCurrencyDecision(decision, cfg.CryptoSymbols)

		// Initialize portfolio manager
		// 初始化投资组合管理器
		portfolioMgr := portfolio.NewPortfolioManager(cfg, executor, log)
		if err := portfolioMgr.UpdateBalance(ctx); err != nil {
			log.Error(fmt.Sprintf("获取账户余额失败: %v", err))
		}

		// Update positions for all symbols
		// 更新所有交易对的持仓信息
		for _, symbol := range cfg.CryptoSymbols {
			if err := portfolioMgr.UpdatePosition(ctx, symbol); err != nil {
				log.Warning(fmt.Sprintf("⚠️  获取 %s 持仓信息失败: %v", symbol, err))
			}
		}

		log.Info(portfolioMgr.GetPortfolioSummary())

		// Initialize trade coordinator
		// 初始化交易协调器
		coordinator := executors.NewTradeCoordinator(cfg, executor, log)

		// Initialize stop-loss manager
		// 初始化止损管理器
		stopLossManager := executors.NewStopLossManager(cfg, executor, log)

		// Start real-time position monitoring in background
		// 在后台启动实时持仓监控
		go stopLossManager.MonitorPositions(10 * time.Second) // 每 10 秒检查一次

		// Execute trades for each symbol
		// 为每个交易对执行交易
		executionResults := make(map[string]string)

		for symbol, symbolDecision := range decisions {
			log.Subheader(fmt.Sprintf("处理 %s 交易决策", symbol), '-', 60)

			if !symbolDecision.Valid {
				log.Warning(fmt.Sprintf("⚠️  %s 决策无效: %s", symbol, symbolDecision.Reason))
				executionResults[symbol] = fmt.Sprintf("决策无效: %s", symbolDecision.Reason)
				continue
			}

			log.Info(fmt.Sprintf("交易对: %s", symbol))
			log.Info(fmt.Sprintf("动作: %s", symbolDecision.Action))
			log.Info(fmt.Sprintf("置信度: %.2f", symbolDecision.Confidence))
			log.Info(fmt.Sprintf("理由: %s", symbolDecision.Reason))

			// Skip HOLD actions
			// 跳过 HOLD 动作
			if symbolDecision.Action == executors.ActionHold {
				log.Info("💤 观望决策，不执行交易")
				executionResults[symbol] = "观望，不执行交易"
				continue
			}

			// Update position info for this symbol
			// 更新该交易对的持仓信息
			if err := portfolioMgr.UpdatePosition(ctx, symbol); err != nil {
				log.Warning(fmt.Sprintf("⚠️  获取 %s 持仓信息失败: %v", symbol, err))
			}

			// Get current position
			// 获取当前持仓
			currentPosition, err := executor.GetCurrentPosition(ctx, symbol)
			if err != nil {
				log.Warning(fmt.Sprintf("⚠️  获取 %s 当前持仓失败: %v", symbol, err))
			}

			// Validate decision against current position
			// 验证决策与当前持仓的一致性
			if err := agents.ValidateDecision(symbolDecision, currentPosition); err != nil {
				log.Error(fmt.Sprintf("❌ %s 决策验证失败: %v", symbol, err))
				executionResults[symbol] = fmt.Sprintf("决策验证失败: %v", err)
				continue
			}

			// Execute the trade using coordinator
			// 使用协调器执行交易
			result, err := coordinator.ExecuteDecision(ctx, symbol, symbolDecision.Action, symbolDecision.Reason)
			if err != nil {
				log.Error(fmt.Sprintf("❌ %s 交易执行失败: %v", symbol, err))
				executionResults[symbol] = fmt.Sprintf("执行失败: %v", err)
				continue
			}

			// Display execution summary
			// 显示执行摘要
			log.Info(coordinator.GetExecutionSummary(result))

			if result.Success {
				executionResults[symbol] = fmt.Sprintf("✅ 成功执行 %s", result.Action)

				// Register position for stop-loss management (only for opening positions)
				// 注册持仓到止损管理器（仅开仓时）
				if symbolDecision.Action == executors.ActionBuy || symbolDecision.Action == executors.ActionSell {
					// Validate and get leverage to use
					// 验证并获取要使用的杠杆
					leverageToUse := agents.ValidateLeverage(
						symbolDecision.Leverage,
						cfg.BinanceLeverageMin,
						cfg.BinanceLeverageMax,
						cfg.BinanceLeverageDynamic,
					)

					if cfg.BinanceLeverageDynamic {
						log.Info(fmt.Sprintf("💡 LLM 选择杠杆: %dx (范围: %d-%d)", leverageToUse, cfg.BinanceLeverageMin, cfg.BinanceLeverageMax))
					} else {
						log.Info(fmt.Sprintf("💡 使用固定杠杆: %dx", leverageToUse))
					}

					// Calculate initial stop-loss if not provided by LLM
					// 如果 LLM 未提供止损价格，则计算初始止损
					initialStopLoss := symbolDecision.StopLoss
					if initialStopLoss == 0 {
						// Use 2.5% default stop-loss
						// 使用 2.5% 默认止损
						if symbolDecision.Action == executors.ActionBuy {
							initialStopLoss = result.Price * 0.975 // -2.5%
						} else {
							initialStopLoss = result.Price * 1.025 // +2.5%
						}
						log.Info(fmt.Sprintf("LLM 未提供止损价格，使用默认 2.5%% 止损: %.2f", initialStopLoss))
					}

					// Get ATR value from indicators for dynamic trailing stop
					// 从指标中获取 ATR 值用于动态追踪止损
					var atrValue float64
					reports := state.GetSymbolReports(symbol)
					if reports != nil && reports.TechnicalIndicators != nil {
						indicators := reports.TechnicalIndicators
						if len(indicators.ATR) > 0 {
							// Get latest ATR value
							lastIdx := len(indicators.ATR) - 1
							if lastIdx >= 0 && !math.IsNaN(indicators.ATR[lastIdx]) {
								atrValue = indicators.ATR[lastIdx]
								atrPercent := (atrValue / result.Price) * 100
								log.Info(fmt.Sprintf("当前 ATR: %.2f (%.2f%% of price)", atrValue, atrPercent))
							}
						}
					}

					// Create position
					// 创建持仓
					// Determine position side from action
					// 从动作确定持仓方向
					positionSide := "long"
					if symbolDecision.Action == executors.ActionSell {
						positionSide = "short"
					}

					position := &executors.Position{
						ID:              fmt.Sprintf("%s-%d", symbol, time.Now().Unix()),
						Symbol:          symbol,
						Side:            positionSide,
						EntryPrice:      result.Price,
						EntryTime:       time.Now(),
						Quantity:        result.Amount,
						Leverage:        leverageToUse,
						InitialStopLoss: initialStopLoss,
						CurrentStopLoss: initialStopLoss,
						StopLossType:    "fixed",
						OpenReason:      symbolDecision.Reason,
						ATR:             atrValue, // Add ATR for dynamic trailing stop
					}

					// Register to stop-loss manager
					// 注册到止损管理器
					stopLossManager.RegisterPosition(position)

					// Place initial stop-loss order
					// 下初始止损单
					if err := stopLossManager.PlaceInitialStopLoss(ctx, position); err != nil {
						log.Warning(fmt.Sprintf("⚠️  下初始止损单失败: %v", err))
					} else {
						log.Success(fmt.Sprintf("✅ 初始止损单已下达: %.2f", initialStopLoss))
					}
				}
			} else {
				executionResults[symbol] = fmt.Sprintf("❌ 执行失败: %s", result.Message)
			}
		}

		// Update portfolio summary after execution
		// 执行后更新投资组合摘要
		log.Subheader("执行后投资组合状态", '─', 80)
		if err := portfolioMgr.UpdateBalance(ctx); err != nil {
			log.Warning(fmt.Sprintf("⚠️  获取更新后的余额失败: %v", err))
		}

		// Update positions for all symbols
		// 更新所有交易对的持仓信息
		for _, symbol := range cfg.CryptoSymbols {
			if err := portfolioMgr.UpdatePosition(ctx, symbol); err != nil {
				log.Warning(fmt.Sprintf("⚠️  获取 %s 持仓信息失败: %v", symbol, err))
			}
		}

		log.Info(portfolioMgr.GetPortfolioSummary())

		// Display execution summary
		// 显示执行摘要
		log.Subheader("执行结果摘要", '─', 80)
		for symbol, result := range executionResults {
			log.Info(fmt.Sprintf("【%s】%s", symbol, result))
		}

		// Build execution result string
		// 构建执行结果字符串
		var resultBuilder strings.Builder
		for symbol, result := range executionResults {
			resultBuilder.WriteString(fmt.Sprintf("%s: %s\n", symbol, result))
		}

		// Update database with execution results
		// 更新数据库中的执行结果
		log.Info("更新数据库执行记录...")
		executionResultStr := resultBuilder.String()
		for _, symbol := range cfg.CryptoSymbols {
			if err := db.UpdateLatestSessionExecution(symbol, cfg.CryptoTimeframe, true, executionResultStr); err != nil {
				log.Warning(fmt.Sprintf("⚠️  更新 %s 执行记录失败: %v", symbol, err))
			}
		}

		log.Success("✅ 自动执行流程完成")
	} else {
		log.Info("💤 自动执行模式未启用 (设置 AUTO_EXECUTE=true 以启用)")
	}

}
