package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/executors"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/storage"
	"path/filepath"
	"strings"
	"testing"
)

// TestLLMJSONOutputWithHistoricalData 使用历史数据测试 LLM 的 JSON 输出
// 这是一个集成测试，需要真实的 OpenAI API Key
func TestLLMJSONOutputWithHistoricalData(t *testing.T) {
	// 加载配置（指定 .env 文件路径，相对于测试文件位置）
	envPath := filepath.Join("../../.env")
	cfg, err := config.LoadConfig(envPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 检查是否设置了 API Key
	if cfg.APIKey == "" || cfg.APIKey == "your_openai_key" {
		t.Skip("跳过集成测试：需要在 .env 中设置 OPENAI_API_KEY")
	}

	// 初始化日志
	log := logger.NewColorLogger(false)

	// 连接数据库
	dbPath := filepath.Join("../../data", "trading.db")
	db, err := storage.NewStorage(dbPath)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 获取最新的一条会话数据
	sessions, err := db.GetLatestSessions(1)
	if err != nil {
		t.Fatalf("查询历史会话失败: %v", err)
	}

	if len(sessions) == 0 {
		t.Skip("数据库中没有历史会话数据")
	}

	session := sessions[0]
	t.Logf("📊 使用历史会话数据: ID=%d, Symbol=%s, Time=%s",
		session.ID, session.Symbol, session.CreatedAt.Format("2006-01-02 15:04:05"))

	// 准备测试用的 SimpleTradingGraph（不需要真实的 executor）
	graph := &SimpleTradingGraph{
		config: cfg,
		logger: log,
		state:  NewAgentState([]string{session.Symbol}, session.Timeframe),
	}

	// 设置历史数据到 state
	graph.state.SetMarketReport(session.Symbol, session.MarketReport)
	graph.state.SetCryptoReport(session.Symbol, session.CryptoReport)
	graph.state.SetSentimentReport(session.Symbol, session.SentimentReport)
	graph.state.SetPositionInfo(session.Symbol, session.PositionInfo)

	// 临时修改配置使用 JSON Prompt
	originalPromptPath := cfg.TraderPromptPath
	cfg.TraderPromptPath = "../../prompts/trader_json.txt"
	defer func() { cfg.TraderPromptPath = originalPromptPath }()

	t.Log("🤖 调用 LLM 生成 JSON 格式决策...")

	// 调用 LLM（使用 JSON Schema 模式）
	ctx := context.Background()
	decision, err := graph.makeLLMDecision(ctx)
	if err != nil {
		t.Fatalf("LLM 决策失败: %v", err)
	}

	t.Log("✅ LLM 响应成功")
	t.Logf("\n📝 原始 LLM 输出（JSON）:\n%s\n", decision)

	// 解析 JSON
	var result TradeDecision
	if err := sonic.Unmarshal([]byte(decision), &result); err != nil {
		t.Fatalf("❌ JSON 解析失败: %v\n原始内容: %s", err, decision)
	}

	t.Log("✅ JSON 解析成功")

	// 验证必填字段
	if result.Symbol == "" {
		t.Error("❌ 验证失败: symbol 字段为空")
	}
	if result.Action == "" {
		t.Error("❌ 验证失败: action 字段为空")
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("❌ 验证失败: confidence 值异常: %.2f", result.Confidence)
	}

	// 打印解析后的结构化数据
	t.Log("\n📊 解析后的结构化数据:")
	t.Logf("  Symbol: %s", result.Symbol)
	t.Logf("  Action: %s", result.Action)
	t.Logf("  Confidence: %.2f", result.Confidence)
	t.Logf("  Leverage: %d", result.Leverage)
	t.Logf("  Position Size: %.1f%%", result.PositionSize)
	t.Logf("  Stop Loss: $%.2f", result.StopLoss)
	t.Logf("  Risk/Reward: %.1f:1", result.RiskRewardRatio)
	t.Logf("  Reasoning: %s", result.Reasoning)
	t.Logf("  Summary: %s", result.Summary)

	if result.CurrentPnlPercent != nil {
		t.Logf("  Current PnL: %.2f%%", *result.CurrentPnlPercent)
	}
	if result.NewStopLoss != nil {
		t.Logf("  New Stop Loss: $%.2f", *result.NewStopLoss)
	}
	if result.StopLossReason != nil {
		t.Logf("  Stop Loss Reason: %s", *result.StopLossReason)
	}

	// 对比历史决策（如果存在）
	if session.Decision != "" {
		t.Log("\n📜 历史决策（旧格式）:")
		t.Logf("%s", session.Decision)
	}

	t.Log("\n✅ 所有验证通过！JSON Schema 模式工作正常。")
}

// TestLLMJSONOutputWithMultipleHistoricalSessions 使用多个历史会话测试
func TestLLMJSONOutputWithMultipleHistoricalSessions(t *testing.T) {
	// 加载配置（指定 .env 文件路径，相对于测试文件位置）
	envPath := filepath.Join("../../.env")
	cfg, err := config.LoadConfig(envPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 检查是否设置了 API Key
	if cfg.APIKey == "" || cfg.APIKey == "your_openai_key" {
		t.Skip("跳过集成测试：需要在 .env 中设置 OPENAI_API_KEY")
	}

	log := logger.NewColorLogger(false)

	dbPath := filepath.Join("../../data", "trading.db")
	db, err := storage.NewStorage(dbPath)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 获取最新的 3 条会话
	sessions, err := db.GetLatestSessions(3)
	if err != nil {
		t.Fatalf("查询历史会话失败: %v", err)
	}

	if len(sessions) == 0 {
		t.Skip("数据库中没有历史会话数据")
	}

	t.Logf("📊 将测试 %d 个历史会话", len(sessions))

	// 临时修改配置使用 JSON Prompt
	originalPromptPath := cfg.TraderPromptPath
	cfg.TraderPromptPath = "../../prompts/trader_json.txt"
	defer func() { cfg.TraderPromptPath = originalPromptPath }()

	successCount := 0
	failCount := 0

	for i, session := range sessions {
		t.Logf("\n=== 测试会话 %d/%d ===", i+1, len(sessions))
		t.Logf("ID=%d, Symbol=%s, Time=%s",
			session.ID, session.Symbol, session.CreatedAt.Format("2006-01-02 15:04:05"))

		graph := &SimpleTradingGraph{
			config: cfg,
			logger: log,
			state:  NewAgentState([]string{session.Symbol}, session.Timeframe),
		}

		graph.state.SetMarketReport(session.Symbol, session.MarketReport)
		graph.state.SetCryptoReport(session.Symbol, session.CryptoReport)
		graph.state.SetSentimentReport(session.Symbol, session.SentimentReport)
		graph.state.SetPositionInfo(session.Symbol, session.PositionInfo)

		ctx := context.Background()
		decision, err := graph.makeLLMDecision(ctx)
		if err != nil {
			t.Logf("  ❌ LLM 调用失败: %v", err)
			failCount++
			continue
		}

		var result TradeDecision
		if err := sonic.Unmarshal([]byte(decision), &result); err != nil {
			t.Logf("  ❌ JSON 解析失败: %v", err)
			failCount++
			continue
		}

		// 快速验证
		if result.Symbol == "" || result.Action == "" {
			t.Logf("  ❌ 验证失败: 缺少必填字段")
			failCount++
			continue
		}

		t.Logf("  ✅ 成功 - Action: %s, Confidence: %.2f", result.Action, result.Confidence)
		successCount++
	}

	t.Logf("\n📈 测试结果统计:")
	t.Logf("  成功: %d/%d", successCount, len(sessions))
	t.Logf("  失败: %d/%d", failCount, len(sessions))

	if successCount == 0 {
		t.Fatal("所有测试都失败了")
	}

	t.Logf("\n✅ JSON Schema 模式批量测试完成！")
}

// TestEndToEndJSONOutput 端到端测试：使用真实配置完整运行交易逻辑并输出 JSON
// 这个测试最贴合实际使用场景，展示完整的工作流程
func TestEndToEndJSONOutput(t *testing.T) {
	t.Log("========================================")
	t.Log("🚀 端到端集成测试：完整交易逻辑 + JSON 输出")
	t.Log("========================================\n")

	// 1. 加载真实配置（使用用户的 .env 文件）
	t.Log("📂 步骤 1/6: 加载配置文件...")
	envPath := filepath.Join("../../.env")
	cfg, err := config.LoadConfig(envPath)
	if err != nil {
		t.Fatalf("❌ 加载配置失败: %v", err)
	}
	t.Logf("✅ 配置加载成功")
	t.Logf("   - 模型: %s", cfg.QuickThinkLLM)
	t.Logf("   - Prompt 路径: %s", cfg.TraderPromptPath)
	t.Logf("   - 交易对: %v", cfg.CryptoSymbols)
	t.Logf("   - 时间周期: %s\n", cfg.CryptoTimeframe)

	// 检查 API Key
	if cfg.APIKey == "" || cfg.APIKey == "your_openai_key" {
		t.Skip("⚠️  跳过测试：需要在 .env 中配置有效的 OPENAI_API_KEY")
	}

	// 2. 初始化日志
	t.Log("📝 步骤 2/6: 初始化日志系统...")
	log := logger.NewColorLogger(false)
	t.Log("✅ 日志系统初始化完成\n")

	// 3. Initialize database and trading components (for real position info, etc.)
	// 3. 初始化数据库与交易执行组件（用于真实持仓信息等）
	t.Log("🗄️  步骤 3/6: 初始化数据库与交易执行组件...")
	dbPath := filepath.Join("../../data", "trading.db")
	db, err := storage.NewStorage(dbPath)
	if err != nil {
		t.Fatalf("❌ 数据库连接失败: %v", err)
	}
	defer db.Close()

	executor := executors.NewBinanceExecutor(cfg, log)
	stopLossManager := executors.NewStopLossManager(cfg, executor, log, db)
	t.Log("✅ 数据库与交易执行组件初始化完成\n")

	// 4. Build trading graph and run with real Binance data
	// 4. 使用真实币安数据构建交易图并运行
	t.Log("📈 步骤 4/6: 使用真实币安数据运行交易图...")

	if len(cfg.CryptoSymbols) == 0 {
		t.Skip("⚠️  未配置交易对 (CRYPTO_SYMBOLS)，跳过测试")
	}

	// To control cost, only use the first symbol (still a full end-to-end pipeline)
	// 为控制测试成本，仅使用第一个交易对（仍然是端到端链路）
	//originalSymbols := cfg.CryptoSymbols
	//testSymbols := []string{cfg.CryptoSymbols[0]}
	//if len(cfg.CryptoSymbols) > 1 {
	//	t.Logf("   ℹ️  为控制测试成本，仅使用第一个交易对: %s (原配置: %v)", testSymbols[0], cfg.CryptoSymbols)
	//}
	//cfg.CryptoSymbols = testSymbols
	//defer func() { cfg.CryptoSymbols = originalSymbols }()

	// 临时修改配置使用 JSON Prompt（如果需要）
	originalPromptPath := cfg.TraderPromptPath
	if !strings.Contains(cfg.TraderPromptPath, "trader_json.txt") {
		cfg.TraderPromptPath = "../../prompts/trader_json.txt"
		t.Logf("   ℹ️  切换到 JSON Prompt: %s", cfg.TraderPromptPath)
	}
	defer func() { cfg.TraderPromptPath = originalPromptPath }()

	tradingGraph := NewSimpleTradingGraph(cfg, log, executor, stopLossManager)

	ctx := context.Background()
	runResult, err := tradingGraph.Run(ctx)
	if err != nil {
		t.Fatalf("❌ 交易图执行失败: %v", err)
	}
	t.Log("✅ 交易图执行完成（已通过 Binance API 获取 K 线 / 资金费率 / 订单簿 / 情绪 / 持仓 等数据）\n")

	// Extract LLM decision (JSON string) from workflow output
	// 从工作流输出中提取 LLM 决策（JSON 字符串）
	decisionValue, ok := runResult["decision"]
	if !ok {
		t.Fatalf("❌ 工作流结果中未找到 decision 字段")
	}
	decision, ok := decisionValue.(string)
	if !ok {
		t.Fatalf("❌ decision 字段类型不是字符串，实际类型: %T", decisionValue)
	}

	// Ensure each sub-agent has produced its reports (market_report / crypto_report / sentiment_report / position_info)
	// 确认各个子智能体已经生成报告（market_report / crypto_report / sentiment_report / position_info）
	state := tradingGraph.GetState()
	for _, symbol := range cfg.CryptoSymbols {
		reports := state.GetSymbolReports(symbol)
		if reports == nil {
			t.Fatalf("❌ 未找到 %s 的报告", symbol)
		}
		t.Logf("📊 %s 报告摘要: market=%d 字符, crypto=%d 字符, sentiment=%d 字符, position=%d 字符",
			symbol,
			len(reports.MarketReport),
			len(reports.CryptoReport),
			len(reports.SentimentReport),
			len(reports.PositionInfo),
		)
	}

	// 5. 展示和验证 JSON 输出
	t.Log("📊 步骤 5/6: 解析并验证 JSON 输出...")
	t.Log("========================================")
	t.Log("📝 LLM 原始输出（JSON 格式）:")
	t.Log("========================================")
	t.Logf("\n%s\n", decision)
	t.Log("========================================\n")

	// 解析 JSON，支持多币种 map 或单对象两种格式
	// Parse JSON, support both multi-symbol map and single-object formats
	t.Log("🔍 尝试解析多币种 JSON 输出...")

	var (
		multiDecisions map[string]TradeDecision
		singleDecision TradeDecision
	)

	parseErrors := []string{}

	if err := sonic.Unmarshal([]byte(decision), &multiDecisions); err == nil && len(multiDecisions) > 0 {
		t.Logf("✅ 检测到多币种 JSON 输出，共 %d 个交易对", len(multiDecisions))

		// 逐个交易对验证
		// Validate each symbol decision
		for symbol, d := range multiDecisions {
			t.Log("========================================")
			t.Logf("📋 交易对 %s 的结构化决策:", symbol)
			t.Log("========================================")
			t.Logf("🎯 交易对:       %s", d.Symbol)
			t.Logf("📈 交易动作:     %s", d.Action)
			t.Logf("💯 置信度:       %.2f (%.0f%%)", d.Confidence, d.Confidence*100)
			t.Logf("🔢 杠杆倍数:     %dx", d.Leverage)
			t.Logf("💰 建议仓位:     %.1f%%", d.PositionSize)
			t.Logf("🛑 止损价格:     $%.2f", d.StopLoss)
			t.Logf("⚖️  盈亏比:       %.1f:1", d.RiskRewardRatio)
			t.Logf("📝 交易理由:     %s", d.Reasoning)
			t.Logf("📄 决策总结:     %s", d.Summary)

			if d.CurrentPnlPercent != nil {
				t.Logf("💹 当前盈亏:     %.2f%%", *d.CurrentPnlPercent)
			}
			if d.NewStopLoss != nil {
				t.Logf("🔄 新止损价格:   $%.2f", *d.NewStopLoss)
			}
			if d.StopLossReason != nil {
				t.Logf("💡 止损调整理由: %s", *d.StopLossReason)
			}

			// 字段验证
			// Field validation
			validationErrors := []string{}

			if d.Symbol == "" {
				validationErrors = append(validationErrors, "symbol 字段为空")
			}
			if d.Action == "" {
				validationErrors = append(validationErrors, "action 字段为空")
			}
			if d.Confidence < 0 || d.Confidence > 1 {
				validationErrors = append(validationErrors, fmt.Sprintf("confidence 值异常: %.2f", d.Confidence))
			}
			if d.Action != "HOLD" && d.StopLoss <= 0 {
				validationErrors = append(validationErrors, "非 HOLD 动作但止损价格无效")
			}

			validActions := map[string]bool{
				"BUY": true, "SELL": true, "HOLD": true,
				"CLOSE_LONG": true, "CLOSE_SHORT": true,
			}
			if !validActions[d.Action] {
				validationErrors = append(validationErrors, fmt.Sprintf("action 值无效: %s", d.Action))
			}

			if len(validationErrors) > 0 {
				t.Error("❌ 字段验证失败:")
				for _, errMsg := range validationErrors {
					t.Errorf("   - %s", errMsg)
				}
				t.FailNow()
			}

			t.Log("✅ 该交易对的字段验证通过！\n")
		}
	} else {
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("多币种 map 解析失败: %v", err))
		}

		// 尝试单对象解析
		// Try single-object parsing
		if err := sonic.Unmarshal([]byte(decision), &singleDecision); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("单对象解析失败: %v", err))
			t.Errorf("❌ JSON 解析失败: %v", parseErrors)
			t.Logf("原始内容:\n%s", decision)
			t.FailNow()
		}

		t.Log("✅ 检测到单对象 JSON 决策输出\n")

		t.Log("========================================")
		t.Log("📋 解析后的结构化交易决策:")
		t.Log("========================================")
		t.Logf("🎯 交易对:       %s", singleDecision.Symbol)
		t.Logf("📈 交易动作:     %s", singleDecision.Action)
		t.Logf("💯 置信度:       %.2f (%.0f%%)", singleDecision.Confidence, singleDecision.Confidence*100)
		t.Logf("🔢 杠杆倍数:     %dx", singleDecision.Leverage)
		t.Logf("💰 建议仓位:     %.1f%%", singleDecision.PositionSize)
		t.Logf("🛑 止损价格:     $%.2f", singleDecision.StopLoss)
		t.Logf("⚖️  盈亏比:       %.1f:1", singleDecision.RiskRewardRatio)
		t.Logf("📝 交易理由:     %s", singleDecision.Reasoning)
		t.Logf("📄 决策总结:     %s", singleDecision.Summary)

		if singleDecision.CurrentPnlPercent != nil {
			t.Logf("💹 当前盈亏:     %.2f%%", *singleDecision.CurrentPnlPercent)
		}
		if singleDecision.NewStopLoss != nil {
			t.Logf("🔄 新止损价格:   $%.2f", *singleDecision.NewStopLoss)
		}
		if singleDecision.StopLossReason != nil {
			t.Logf("💡 止损调整理由: %s", *singleDecision.StopLossReason)
		}

		// 字段验证
		// Field validation
		t.Log("🔍 验证必填字段...")
		validationErrors := []string{}

		if singleDecision.Symbol == "" {
			validationErrors = append(validationErrors, "symbol 字段为空")
		}
		if singleDecision.Action == "" {
			validationErrors = append(validationErrors, "action 字段为空")
		}
		if singleDecision.Confidence < 0 || singleDecision.Confidence > 1 {
			validationErrors = append(validationErrors, fmt.Sprintf("confidence 值异常: %.2f", singleDecision.Confidence))
		}
		if singleDecision.Action != "HOLD" && singleDecision.StopLoss <= 0 {
			validationErrors = append(validationErrors, "非 HOLD 动作但止损价格无效")
		}

		validActions := map[string]bool{
			"BUY": true, "SELL": true, "HOLD": true,
			"CLOSE_LONG": true, "CLOSE_SHORT": true,
		}
		if !validActions[singleDecision.Action] {
			validationErrors = append(validationErrors, fmt.Sprintf("action 值无效: %s", singleDecision.Action))
		}

		if len(validationErrors) > 0 {
			t.Error("❌ 字段验证失败:")
			for _, errMsg := range validationErrors {
				t.Errorf("   - %s", errMsg)
			}
			t.FailNow()
		}

		t.Log("✅ 所有字段验证通过！\n")
	}

	// 最终总结
	t.Log("========================================")
	t.Log("✅ 测试完成总结:")
	t.Log("========================================")
	t.Logf("✓ 配置加载:     成功")
	t.Logf("✓ 工作流运行:   成功")
	t.Logf("✓ LLM 调用:     成功")
	t.Logf("✓ JSON 解析:    成功")
	t.Logf("✓ 字段验证:     通过")
	t.Logf("✓ 模型:         %s", cfg.QuickThinkLLM)
	t.Log("========================================")
	t.Log("🎉 端到端测试全部通过！JSON Schema 模式工作正常。")
	t.Log("========================================\n")
}

// BenchmarkJSONParsing 基准测试：JSON 解析性能
func BenchmarkJSONParsing(b *testing.B) {
	sampleJSON := `{
		"symbol": "BTC/USDT",
		"action": "BUY",
		"confidence": 0.92,
		"leverage": 15,
		"position_size": 10.0,
		"stop_loss": 50000.00,
		"reasoning": "强势突破",
		"risk_reward_ratio": 2.5,
		"summary": "高置信度机会"
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result TradeDecision
		_ = json.Unmarshal([]byte(sampleJSON), &result)
	}
}
