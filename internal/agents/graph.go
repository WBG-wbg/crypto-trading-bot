package agents

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	openaiComponent "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/dataflows"
	"github.com/oak/crypto-trading-bot/internal/executors"
	"github.com/oak/crypto-trading-bot/internal/logger"
)

// AgentState holds the state of all analysts' reports
type AgentState struct {
	Symbol             string
	Timeframe          string
	MarketReport       string
	CryptoReport       string
	SentimentReport    string
	PositionInfo       string
	FinalDecision      string
	OHLCVData          []dataflows.OHLCV
	TechnicalIndicators *dataflows.TechnicalIndicators
	mu                 sync.RWMutex
}

// NewAgentState creates a new agent state
func NewAgentState(symbol, timeframe string) *AgentState {
	return &AgentState{
		Symbol:    symbol,
		Timeframe: timeframe,
	}
}

// SetMarketReport sets the market analysis report
func (s *AgentState) SetMarketReport(report string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MarketReport = report
}

// SetCryptoReport sets the crypto analysis report
func (s *AgentState) SetCryptoReport(report string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CryptoReport = report
}

// SetSentimentReport sets the sentiment analysis report
func (s *AgentState) SetSentimentReport(report string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SentimentReport = report
}

// SetPositionInfo sets the position information
func (s *AgentState) SetPositionInfo(info string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PositionInfo = info
}

// SetFinalDecision sets the final trading decision
func (s *AgentState) SetFinalDecision(decision string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FinalDecision = decision
}

// GetAllReports returns all reports as a formatted string
func (s *AgentState) GetAllReports() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("=== 市场技术分析 ===\n")
	sb.WriteString(s.MarketReport)
	sb.WriteString("\n\n=== 加密货币专属分析 ===\n")
	sb.WriteString(s.CryptoReport)
	sb.WriteString("\n\n=== 市场情绪分析 ===\n")
	sb.WriteString(s.SentimentReport)
	sb.WriteString("\n\n=== 当前持仓信息 ===\n")
	sb.WriteString(s.PositionInfo)

	return sb.String()
}

// SimpleTradingGraph creates a simplified trading workflow using Eino Graph
type SimpleTradingGraph struct {
	config   *config.Config
	logger   *logger.ColorLogger
	executor *executors.BinanceExecutor
	state    *AgentState
}

// NewSimpleTradingGraph creates a new simple trading graph
func NewSimpleTradingGraph(cfg *config.Config, log *logger.ColorLogger, executor *executors.BinanceExecutor) *SimpleTradingGraph {
	return &SimpleTradingGraph{
		config:   cfg,
		logger:   log,
		executor: executor,
		state:    NewAgentState(cfg.CryptoSymbol, cfg.CryptoTimeframe),
	}
}

// BuildGraph constructs the trading workflow graph with parallel execution
func (g *SimpleTradingGraph) BuildGraph(ctx context.Context) (compose.Runnable[map[string]any, map[string]any], error) {
	graph := compose.NewGraph[map[string]any, map[string]any]()

	marketData := dataflows.NewMarketData(g.config)

	// Market Analyst Lambda - Fetches market data and calculates indicators
	marketAnalyst := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🔍 市场分析师：正在获取市场数据...")

		symbol := g.config.GetBinanceSymbol()
		timeframe := g.config.CryptoTimeframe
		lookbackDays := g.config.CryptoLookbackDays

		// Fetch OHLCV data
		ohlcvData, err := marketData.GetOHLCV(ctx, symbol, timeframe, lookbackDays)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch OHLCV: %w", err)
		}

		// Calculate indicators
		indicators := dataflows.CalculateIndicators(ohlcvData)

		// Generate report
		report := dataflows.FormatIndicatorReport(g.config.CryptoSymbol, timeframe, ohlcvData, indicators)

		// Save to state
		g.state.OHLCVData = ohlcvData
		g.state.TechnicalIndicators = indicators
		g.state.SetMarketReport(report)

		g.logger.Success("✅ 市场分析完成")

		return map[string]any{
			"market_report": report,
			"ohlcv_data":    ohlcvData,
			"indicators":    indicators,
		}, nil
	})

	// Crypto Analyst Lambda - Fetches funding rate, order book, 24h stats
	cryptoAnalyst := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🔍 加密货币分析师：正在获取链上数据...")

		symbol := g.config.GetBinanceSymbol()
		var reportBuilder strings.Builder

		// Funding rate
		fundingRate, err := marketData.GetFundingRate(ctx, symbol)
		if err != nil {
			reportBuilder.WriteString(fmt.Sprintf("资金费率获取失败: %v\n", err))
		} else {
			reportBuilder.WriteString(fmt.Sprintf("资金费率: %.6f (%.4f%%)\n", fundingRate, fundingRate*100))
		}

		// Order book
		orderBook, err := marketData.GetOrderBook(ctx, symbol, 20)
		if err != nil {
			reportBuilder.WriteString(fmt.Sprintf("订单簿获取失败: %v\n", err))
		} else {
			reportBuilder.WriteString(fmt.Sprintf("订单簿 - 买单量: %.2f, 卖单量: %.2f, 买卖比: %.2f\n",
				orderBook["bid_volume"], orderBook["ask_volume"], orderBook["bid_ask_ratio"]))
		}

		// 24h stats
		stats, err := marketData.Get24HrStats(ctx, symbol)
		if err != nil {
			reportBuilder.WriteString(fmt.Sprintf("24h统计获取失败: %v\n", err))
		} else {
			reportBuilder.WriteString(fmt.Sprintf("24h统计 - 价格变化: %s%%, 最高: $%s, 最低: $%s, 成交量: %s\n",
				stats["price_change_percent"], stats["high_price"], stats["low_price"], stats["volume"]))
		}

		report := reportBuilder.String()
		g.state.SetCryptoReport(report)

		g.logger.Success("✅ 加密货币分析完成")

		return map[string]any{
			"crypto_report": report,
		}, nil
	})

	// Sentiment Analyst Lambda - Fetches market sentiment
	sentimentAnalyst := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🔍 情绪分析师：正在获取市场情绪...")

		sentiment := dataflows.GetSentimentIndicators(ctx, "BTC")
		report := dataflows.FormatSentimentReport(sentiment)

		g.state.SetSentimentReport(report)

		g.logger.Success("✅ 情绪分析完成")

		return map[string]any{
			"sentiment_report": report,
			"sentiment_data":   sentiment,
		}, nil
	})

	// Position Info Lambda - Gets current position
	positionInfo := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("📊 获取持仓信息...")

		posInfo := g.executor.GetPositionSummary(ctx, g.config.CryptoSymbol)
		g.state.SetPositionInfo(posInfo)

		return map[string]any{
			"position_info": posInfo,
		}, nil
	})

	// Trader Lambda - Makes final decision using LLM
	trader := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		g.logger.Info("🤖 交易员：正在制定交易策略...")

		allReports := g.state.GetAllReports()

		// Try to use LLM for decision, fall back to simple rules if LLM fails
		var decision string
		var err error

		// Check if API key is configured
		if g.config.APIKey != "" && g.config.APIKey != "your_openai_key" {
			decision, err = g.makeLLMDecision(ctx)
			if err != nil {
				g.logger.Warning(fmt.Sprintf("LLM 决策失败: %v", err))
				decision = g.makeSimpleDecision()
			}
		} else {
			g.logger.Info("OpenAI API Key 未配置，使用简单规则决策")
			decision = g.makeSimpleDecision()
		}

		g.state.SetFinalDecision(decision)

		g.logger.Decision(decision)

		return map[string]any{
			"decision":    decision,
			"all_reports": allReports,
		}, nil
	})

	// Add nodes to graph
	if err := graph.AddLambdaNode("market_analyst", marketAnalyst); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("crypto_analyst", cryptoAnalyst); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("sentiment_analyst", sentimentAnalyst); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("position_info", positionInfo); err != nil {
		return nil, err
	}
	if err := graph.AddLambdaNode("trader", trader); err != nil {
		return nil, err
	}

	// Parallel execution: market_analyst and sentiment_analyst run in parallel
	if err := graph.AddEdge(compose.START, "market_analyst"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(compose.START, "sentiment_analyst"); err != nil {
		return nil, err
	}

	// After market_analyst completes, run crypto_analyst
	if err := graph.AddEdge("market_analyst", "crypto_analyst"); err != nil {
		return nil, err
	}

	// After crypto_analyst completes, get position info
	if err := graph.AddEdge("crypto_analyst", "position_info"); err != nil {
		return nil, err
	}

	// Wait for both sentiment_analyst and position_info before trader
	if err := graph.AddEdge("sentiment_analyst", "trader"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("position_info", "trader"); err != nil {
		return nil, err
	}

	// Trader outputs to END
	if err := graph.AddEdge("trader", compose.END); err != nil {
		return nil, err
	}

	// Compile with AllPredecessor trigger mode (wait for all inputs)
	return graph.Compile(ctx, compose.WithNodeTriggerMode(compose.AllPredecessor))
}

// makeSimpleDecision creates a simple rule-based decision (fallback when LLM is disabled)
func (g *SimpleTradingGraph) makeSimpleDecision() string {
	var decision strings.Builder

	decision.WriteString("=== 交易决策分析 ===\n\n")

	// Analyze technical indicators
	if g.state.TechnicalIndicators != nil && len(g.state.OHLCVData) > 0 {
		lastIdx := len(g.state.OHLCVData) - 1
		rsi := g.state.TechnicalIndicators.RSI
		macd := g.state.TechnicalIndicators.MACD
		signal := g.state.TechnicalIndicators.Signal

		decision.WriteString("技术面分析:\n")

		// RSI analysis
		if len(rsi) > lastIdx {
			rsiVal := rsi[lastIdx]
			decision.WriteString(fmt.Sprintf("- RSI(14): %.2f ", rsiVal))
			if rsiVal > 70 {
				decision.WriteString("(超买区域，可能回调)\n")
			} else if rsiVal < 30 {
				decision.WriteString("(超卖区域，可能反弹)\n")
			} else {
				decision.WriteString("(中性区域)\n")
			}
		}

		// MACD analysis
		if len(macd) > lastIdx && len(signal) > lastIdx {
			macdVal := macd[lastIdx]
			signalVal := signal[lastIdx]
			decision.WriteString(fmt.Sprintf("- MACD: %.2f, Signal: %.2f ", macdVal, signalVal))
			if macdVal > signalVal {
				decision.WriteString("(MACD在Signal之上，多头信号)\n")
			} else {
				decision.WriteString("(MACD在Signal之下，空头信号)\n")
			}
		}
	}

	decision.WriteString("\n**最终建议**: HOLD（观望）\n")
	decision.WriteString("\n说明: 这是基于规则的简单决策（LLM 未启用）。\n")

	return decision.String()
}

// makeLLMDecision uses LLM to generate trading decision
func (g *SimpleTradingGraph) makeLLMDecision(ctx context.Context) (string, error) {
	// Create OpenAI config
	cfg := &openaiComponent.ChatModelConfig{
		APIKey:  g.config.APIKey,
		BaseURL: g.config.BackendURL,
		Model:   g.config.DeepThinkLLM,
	}

	// Create ChatModel
	chatModel, err := openaiComponent.NewChatModel(ctx, cfg)
	if err != nil {
		g.logger.Warning(fmt.Sprintf("LLM 初始化失败，使用简单规则决策: %v", err))
		return g.makeSimpleDecision(), nil
	}

	// Prepare the prompt with all reports
	allReports := g.state.GetAllReports()

	systemPrompt := `你是一个专业的加密货币交易分析师。你需要综合分析市场技术面、链上数据、市场情绪和当前持仓信息，给出明确的交易决策。

你的决策必须包含以下内容：
1. **技术面分析总结**：基于 RSI、MACD、布林带等指标
2. **链上数据分析**：资金费率、订单簿、成交量的含义
3. **市场情绪分析**：正面/负面情绪对价格的影响
4. **持仓分析**：当前持仓状态和盈亏情况
5. **最终决策**：必须是以下之一
   - **BUY**（做多）- 预期价格上涨
   - **SELL**（做空）- 预期价格下跌
   - **HOLD**（观望）- 市场不明朗，保持观望
   - **CLOSE_LONG**（平多仓）- 当前有多仓需要平仓
   - **CLOSE_SHORT**（平空仓）- 当前有空仓需要平仓
6. **风险提示**：主要风险因素

请用中文回答，语言简洁专业。`

	userPrompt := fmt.Sprintf(`请分析以下数据并给出交易决策：

%s

请给出你的分析和最终决策。`, allReports)

	// Create messages
	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}

	// Call LLM
	g.logger.Info("🤖 正在调用 LLM 生成交易决策...")
	response, err := chatModel.Generate(ctx, messages)
	if err != nil {
		g.logger.Warning(fmt.Sprintf("LLM 调用失败，使用简单规则决策: %v", err))
		return g.makeSimpleDecision(), nil
	}

	g.logger.Success("✅ LLM 决策生成完成")

	// Log token usage if available
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		g.logger.Info(fmt.Sprintf("Token 使用: %d (输入: %d, 输出: %d)",
			response.ResponseMeta.Usage.TotalTokens,
			response.ResponseMeta.Usage.PromptTokens,
			response.ResponseMeta.Usage.CompletionTokens))
	}

	return response.Content, nil
}

// Run executes the trading graph
func (g *SimpleTradingGraph) Run(ctx context.Context) (map[string]any, error) {
	g.logger.Header("启动交易分析工作流", '=', 80)

	compiled, err := g.BuildGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	input := map[string]any{
		"symbol":    g.config.CryptoSymbol,
		"timeframe": g.config.CryptoTimeframe,
	}

	result, err := compiled.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("graph execution failed: %w", err)
	}

	g.logger.Header("工作流执行完成", '=', 80)

	return result, nil
}

// GetState returns the current agent state
func (g *SimpleTradingGraph) GetState() *AgentState {
	return g.state
}
