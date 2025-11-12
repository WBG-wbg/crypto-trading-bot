package executors

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/jpillora/backoff"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
)

// TradeAction represents trading actions
type TradeAction string

const (
	ActionBuy        TradeAction = "BUY"
	ActionSell       TradeAction = "SELL"
	ActionCloseLong  TradeAction = "CLOSE_LONG"
	ActionCloseShort TradeAction = "CLOSE_SHORT"
	ActionHold       TradeAction = "HOLD"
)

// PositionMode represents the position mode
type PositionMode string

const (
	PositionModeOneWay PositionMode = "oneway"
	PositionModeHedge  PositionMode = "hedge"
)

// Position represents a trading position
type Position struct {
	// Basic position info
	// 基础持仓信息
	ID               string    // 持仓 ID / Position ID
	Symbol           string    // 交易对 / Trading pair
	Side             string    // long/short
	Size             float64   // 持仓大小 / Position size (same as Quantity)
	EntryPrice       float64   // 入场价格 / Entry price
	EntryTime        time.Time // 入场时间 / Entry time
	CurrentPrice     float64   // 当前价格 / Current price
	HighestPrice     float64   // 最高价（多仓）或最低价（空仓）/ Highest/lowest price
	Quantity         float64   // 持仓数量 / Quantity (same as Size)
	UnrealizedPnL    float64   // 未实现盈亏 / Unrealized PnL
	PositionAmt      float64   // 仓位金额 / Position amount
	Leverage         int       // 杠杆倍数 / Leverage
	LiquidationPrice float64   // 强平价格 / Liquidation price

	// Stop-loss management
	// 止损管理
	InitialStopLoss   float64 // 初始止损价格 / Initial stop-loss
	CurrentStopLoss   float64 // 当前止损价格 / Current stop-loss
	StopLossType      string  // 止损类型：fixed, breakeven, trailing
	TrailingDistance  float64 // 追踪距离（百分比）/ Trailing distance
	PartialTPExecuted bool    // 是否已执行分批止盈 / Whether partial TP has been executed
	ATR               float64 // ATR 值用于动态追踪距离 / ATR value for dynamic trailing distance

	// Order management
	// 订单管理
	StopLossOrderID string // 当前止损单 ID / Stop-loss order ID

	// History and context
	// 历史和上下文
	StopLossHistory []StopLossEvent // 止损变更历史 / Stop-loss history
	PriceHistory    []PricePoint    // 价格历史 / Price history
	OpenReason      string          // 开仓理由 / Opening reason
	LastLLMReview   time.Time       // 上次 LLM 复查时间 / Last LLM review
	LLMSuggestions  []string        // LLM 建议 / LLM suggestions
}

// StopLossEvent represents a stop-loss change event
// StopLossEvent 表示止损变更事件
type StopLossEvent struct {
	Time    time.Time
	OldStop float64
	NewStop float64
	Reason  string
	Trigger string // program or llm
}

// PricePoint represents a price point in time
// PricePoint 表示价格点
type PricePoint struct {
	Time  time.Time
	Price float64
}

// TradeResult represents the result of a trade execution
type TradeResult struct {
	Success     bool
	Action      TradeAction
	Symbol      string
	Amount      float64
	Timestamp   string
	Reason      string
	TestMode    bool
	OrderID     string
	Price       float64
	Filled      float64
	Message     string
	NewPosition *Position
}

// BinanceExecutor handles Binance futures trading
type BinanceExecutor struct {
	client       *futures.Client
	config       *config.Config
	testMode     bool
	positionMode PositionMode
	logger       *logger.ColorLogger
	tradeHistory []TradeResult
}

// NewBinanceExecutor creates a new BinanceExecutor
// NewBinanceExecutor 创建一个新的 BinanceExecutor
func NewBinanceExecutor(cfg *config.Config, log *logger.ColorLogger) *BinanceExecutor {
	futures.UseTestnet = cfg.BinanceTestMode

	client := futures.NewClient(cfg.BinanceAPIKey, cfg.BinanceAPISecret)

	// Set proxy if configured
	// 如果配置了代理，则设置代理
	if cfg.BinanceProxy != "" {
		proxyURL, err := url.Parse(cfg.BinanceProxy)
		if err != nil {
			log.Warning(fmt.Sprintf("代理 URL 解析失败: %v，将不使用代理", err))
		} else {
			// Create custom HTTP client with proxy
			// 创建带代理的自定义 HTTP 客户端
			httpClient := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: cfg.BinanceProxyInsecureSkipTLS, // 是否跳过 TLS 验证 / Skip TLS verification
					},
				},
				Timeout: 30 * time.Second,
			}
			client.HTTPClient = httpClient
			log.Success(fmt.Sprintf("已配置代理: %s (跳过TLS验证: %v)", cfg.BinanceProxy, cfg.BinanceProxyInsecureSkipTLS))
		}
	}

	executor := &BinanceExecutor{
		client:       client,
		config:       cfg,
		testMode:     cfg.BinanceTestMode,
		logger:       log,
		tradeHistory: make([]TradeResult, 0),
	}

	// Print mode
	// 打印模式
	if executor.testMode {
		log.Success("交易执行器：测试模式（模拟交易）")
	} else {
		log.Warning("交易执行器：实盘模式（真实交易！）")
	}

	return executor
}

// DetectPositionMode detects the current position mode
func (e *BinanceExecutor) DetectPositionMode(ctx context.Context) error {
	if e.positionMode != "" {
		return nil
	}

	// Check user configuration first
	configMode := e.config.BinancePositionMode
	if configMode == "oneway" || configMode == "hedge" {
		e.positionMode = PositionMode(configMode)
		modeName := "单向持仓模式（One-way）"
		if configMode == "hedge" {
			modeName = "双向持仓模式（Hedge）"
		}
		e.logger.Success(fmt.Sprintf("使用配置文件(本地)的持仓模式：%s", modeName))
		//return nil
	}

	// Auto-detect mode
	res, err := e.client.NewGetPositionModeService().Do(ctx)
	if err != nil {
		e.logger.Warning("无法自动检测持仓模式，默认使用单向持仓模式")
		e.positionMode = PositionModeOneWay
		return nil
	}

	if res.DualSidePosition {
		e.positionMode = PositionModeHedge
		e.logger.Success("检测到双向持仓模式（Hedge Mode）")
	} else {
		e.positionMode = PositionModeOneWay
		e.logger.Success("检测到单向持仓模式（One-way Mode）")
	}

	return nil
}

// SetupExchange sets up exchange parameters
func (e *BinanceExecutor) SetupExchange(ctx context.Context, symbol string, leverage int) error {
	// Detect position mode
	if err := e.DetectPositionMode(ctx); err != nil {
		return fmt.Errorf("failed to detect position mode: %w", err)
	}

	// Set leverage with retry
	err := e.withRetry(func() error {
		_, err := e.client.NewChangeLeverageService().
			Symbol(e.config.GetBinanceSymbolFor(symbol)).
			Leverage(leverage).
			Do(ctx)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	e.logger.Success(fmt.Sprintf("设置杠杆倍数: %dx", leverage))

	// Get balance
	account, err := e.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account info: %w", err)
	}

	for _, asset := range account.Assets {
		if asset.Asset == "USDT" {
			balance, _ := parseFloat(asset.AvailableBalance)
			e.logger.Success(fmt.Sprintf("当前 USDT 余额: %.2f", balance))
			break
		}
	}

	return nil
}

// GetCurrentPosition gets the current position for a symbol
func (e *BinanceExecutor) GetCurrentPosition(ctx context.Context, symbol string) (*Position, error) {
	var position *Position

	err := e.withRetry(func() error {
		positions, err := e.client.NewGetPositionRiskService().
			Symbol(e.config.GetBinanceSymbolFor(symbol)).
			Do(ctx)

		if err != nil {
			return err
		}

		for _, pos := range positions {
			posAmt, _ := parseFloat(pos.PositionAmt)
			if posAmt != 0 {
				entryPrice, _ := parseFloat(pos.EntryPrice)
				unrealizedPnL, _ := parseFloat(pos.UnRealizedProfit)
				liquidationPrice, _ := parseFloat(pos.LiquidationPrice)
				leverage, _ := parseInt(pos.Leverage)

				side := "long"
				if posAmt < 0 {
					side = "short"
				}

				position = &Position{
					Side:             side,
					Size:             math.Abs(posAmt),
					EntryPrice:       entryPrice,
					UnrealizedPnL:    unrealizedPnL,
					PositionAmt:      posAmt,
					Symbol:           pos.Symbol,
					Leverage:         leverage,
					LiquidationPrice: liquidationPrice,
				}
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get position: %w", err)
	}

	return position, nil
}

// ExecuteTrade executes a trade
func (e *BinanceExecutor) ExecuteTrade(ctx context.Context, symbol string, action TradeAction, amount float64, reason string) *TradeResult {
	result := &TradeResult{
		Success:   false,
		Action:    action,
		Symbol:    symbol,
		Amount:    amount,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Reason:    reason,
		TestMode:  e.testMode,
	}

	// Get current position
	currentPosition, _ := e.GetCurrentPosition(ctx, symbol)

	// Log trade execution
	e.logger.Header("交易执行", '=', 60)
	e.logger.Info(fmt.Sprintf("动作: %s", action))
	e.logger.Info(fmt.Sprintf("交易对: %s", symbol))
	e.logger.Info(fmt.Sprintf("数量: %.4f", amount))
	e.logger.Info(fmt.Sprintf("理由: %s", reason))
	if currentPosition != nil {
		e.logger.Info(fmt.Sprintf("当前持仓: %s %.4f @ $%.2f",
			currentPosition.Side, currentPosition.Size, currentPosition.EntryPrice))
	} else {
		e.logger.Info("当前持仓: 无")
	}

	if e.testMode {
		e.logger.Warning("测试模式 - 仅模拟交易，不实际下单")
		result.Success = true
		result.Message = "测试模式：模拟交易成功"
		return result
	}

	// Detect position mode
	e.DetectPositionMode(ctx)

	// Execute trade based on action
	var err error
	switch action {
	case ActionBuy:
		err = e.executeBuy(ctx, symbol, currentPosition, amount, result)
	case ActionSell:
		err = e.executeSell(ctx, symbol, currentPosition, amount, result)
	case ActionCloseLong:
		err = e.executeCloseLong(ctx, symbol, currentPosition, result)
	case ActionCloseShort:
		err = e.executeCloseShort(ctx, symbol, currentPosition, result)
	case ActionHold:
		e.logger.Info("💤 建议观望，不执行交易")
		result.Success = true
		result.Message = "观望，不执行交易"
		return result
	default:
		result.Message = fmt.Sprintf("未知的交易动作: %s", action)
		e.logger.Error(result.Message)
		return result
	}

	if err != nil {
		result.Message = fmt.Sprintf("订单执行失败: %v", err)
		e.logger.Error(result.Message)
		return result
	}

	// Get updated position
	time.Sleep(2 * time.Second)
	newPosition, _ := e.GetCurrentPosition(ctx, symbol)
	result.NewPosition = newPosition

	// Record to history
	e.tradeHistory = append(e.tradeHistory, *result)

	return result
}

func (e *BinanceExecutor) executeBuy(ctx context.Context, symbol string, currentPosition *Position, amount float64, result *TradeResult) error {
	binanceSymbol := e.config.GetBinanceSymbolFor(symbol)

	// Close short position if exists
	if currentPosition != nil && currentPosition.Side == "short" {
		e.logger.Info("📤 平空仓...")
		positionSide := futures.PositionSideTypeShort
		if e.positionMode == PositionModeOneWay {
			positionSide = futures.PositionSideTypeBoth
		}

		_, err := e.client.NewCreateOrderService().
			Symbol(binanceSymbol).
			Side(futures.SideTypeBuy).
			PositionSide(positionSide).
			Type(futures.OrderTypeMarket).
			Quantity(fmt.Sprintf("%.4f", currentPosition.Size)).
			Do(ctx)

		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}

	// Open long position if not already long
	if currentPosition == nil || currentPosition.Side != "long" {
		e.logger.Info("📈 开多仓...")
		positionSide := futures.PositionSideTypeLong
		if e.positionMode == PositionModeOneWay {
			positionSide = futures.PositionSideTypeBoth
		}

		order, err := e.client.NewCreateOrderService().
			Symbol(binanceSymbol).
			Side(futures.SideTypeBuy).
			PositionSide(positionSide).
			Type(futures.OrderTypeMarket).
			Quantity(fmt.Sprintf("%.4f", amount)).
			Do(ctx)

		if err != nil {
			return err
		}

		result.Success = true
		result.OrderID = fmt.Sprintf("%d", order.OrderID)
		result.Message = "订单执行成功"
		e.logger.Success(fmt.Sprintf("✅ 订单执行成功，订单ID: %d", order.OrderID))
	} else {
		result.Message = "已有多仓，不重复开仓（系统保护：防止意外加仓）"
		e.logger.Warning("⚠️ 已有多仓，不重复开仓")
	}

	return nil
}

func (e *BinanceExecutor) executeSell(ctx context.Context, symbol string, currentPosition *Position, amount float64, result *TradeResult) error {
	binanceSymbol := e.config.GetBinanceSymbolFor(symbol)

	// Close long position if exists
	if currentPosition != nil && currentPosition.Side == "long" {
		e.logger.Info("📤 平多仓...")
		positionSide := futures.PositionSideTypeLong
		if e.positionMode == PositionModeOneWay {
			positionSide = futures.PositionSideTypeBoth
		}

		_, err := e.client.NewCreateOrderService().
			Symbol(binanceSymbol).
			Side(futures.SideTypeSell).
			PositionSide(positionSide).
			Type(futures.OrderTypeMarket).
			Quantity(fmt.Sprintf("%.4f", currentPosition.Size)).
			Do(ctx)

		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}

	// Open short position if not already short
	if currentPosition == nil || currentPosition.Side != "short" {
		e.logger.Info("📉 开空仓...")
		positionSide := futures.PositionSideTypeShort
		if e.positionMode == PositionModeOneWay {
			positionSide = futures.PositionSideTypeBoth
		}

		order, err := e.client.NewCreateOrderService().
			Symbol(binanceSymbol).
			Side(futures.SideTypeSell).
			PositionSide(positionSide).
			Type(futures.OrderTypeMarket).
			Quantity(fmt.Sprintf("%.4f", amount)).
			Do(ctx)

		if err != nil {
			return err
		}

		result.Success = true
		result.OrderID = fmt.Sprintf("%d", order.OrderID)
		result.Message = "订单执行成功"
		e.logger.Success(fmt.Sprintf("✅ 订单执行成功，订单ID: %d", order.OrderID))
	} else {
		result.Message = "已有空仓，不重复开仓（系统保护：防止意外加仓）"
		e.logger.Warning("⚠️ 已有空仓，不重复开仓")
	}

	return nil
}

func (e *BinanceExecutor) executeCloseLong(ctx context.Context, symbol string, currentPosition *Position, result *TradeResult) error {
	if currentPosition == nil || currentPosition.Side != "long" {
		result.Message = "没有多仓可平"
		e.logger.Warning("⚠️ 没有多仓可平")
		return nil
	}

	e.logger.Info("📤 平多仓...")
	binanceSymbol := e.config.GetBinanceSymbolFor(symbol)
	positionSide := futures.PositionSideTypeLong
	if e.positionMode == PositionModeOneWay {
		positionSide = futures.PositionSideTypeBoth
	}

	order, err := e.client.NewCreateOrderService().
		Symbol(binanceSymbol).
		Side(futures.SideTypeSell).
		PositionSide(positionSide).
		Type(futures.OrderTypeMarket).
		Quantity(fmt.Sprintf("%.4f", currentPosition.Size)).
		ReduceOnly(true).
		Do(ctx)

	if err != nil {
		return err
	}

	result.Success = true
	result.OrderID = fmt.Sprintf("%d", order.OrderID)
	result.Message = "订单执行成功"
	e.logger.Success(fmt.Sprintf("✅ 订单执行成功，订单ID: %d", order.OrderID))
	return nil
}

func (e *BinanceExecutor) executeCloseShort(ctx context.Context, symbol string, currentPosition *Position, result *TradeResult) error {
	if currentPosition == nil || currentPosition.Side != "short" {
		result.Message = "没有空仓可平"
		e.logger.Warning("⚠️ 没有空仓可平")
		return nil
	}

	e.logger.Info("📤 平空仓...")
	binanceSymbol := e.config.GetBinanceSymbolFor(symbol)
	positionSide := futures.PositionSideTypeShort
	if e.positionMode == PositionModeOneWay {
		positionSide = futures.PositionSideTypeBoth
	}

	order, err := e.client.NewCreateOrderService().
		Symbol(binanceSymbol).
		Side(futures.SideTypeBuy).
		PositionSide(positionSide).
		Type(futures.OrderTypeMarket).
		Quantity(fmt.Sprintf("%.4f", currentPosition.Size)).
		ReduceOnly(true).
		Do(ctx)

	if err != nil {
		return err
	}

	result.Success = true
	result.OrderID = fmt.Sprintf("%d", order.OrderID)
	result.Message = "订单执行成功"
	e.logger.Success(fmt.Sprintf("✅ 订单执行成功，订单ID: %d", order.OrderID))
	return nil
}

// GetPositionSummary returns a formatted position summary
func (e *BinanceExecutor) GetPositionSummary(ctx context.Context, symbol string) string {
	var summary strings.Builder

	// Get account balance
	account, err := e.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return fmt.Sprintf("**获取账户信息失败**: %v", err)
	}

	var usdtFree, usdtTotal float64
	for _, asset := range account.Assets {
		if asset.Asset == "USDT" {
			usdtFree, _ = parseFloat(asset.AvailableBalance)
			usdtTotal, _ = parseFloat(asset.WalletBalance)
			break
		}
	}

	summary.WriteString("**账户信息**:\n")
	summary.WriteString(fmt.Sprintf("- 可用余额: %.2f USDT\n", usdtFree))
	summary.WriteString(fmt.Sprintf("- 总余额: %.2f USDT\n", usdtTotal))
	summary.WriteString(fmt.Sprintf("- 已使用保证金: %.2f USDT\n\n", usdtTotal-usdtFree))

	// Get position
	position, _ := e.GetCurrentPosition(ctx, symbol)
	if position != nil && position.Side != "" {
		sideCN := "多头"
		if position.Side == "short" {
			sideCN = "空头"
		}

		// Get current price
		ticker, _ := e.client.NewListPriceChangeStatsService().Symbol(e.config.GetBinanceSymbolFor(symbol)).Do(ctx)
		currentPrice := position.EntryPrice
		if len(ticker) > 0 {
			currentPrice, _ = parseFloat(ticker[0].LastPrice)
		}

		// Calculate PnL percentage
		pnlPct := 0.0
		if position.EntryPrice > 0 {
			if position.Side == "long" {
				pnlPct = ((currentPrice - position.EntryPrice) / position.EntryPrice) * 100
			} else {
				pnlPct = ((position.EntryPrice - currentPrice) / position.EntryPrice) * 100
			}
		}

		summary.WriteString(fmt.Sprintf("**当前持仓 %s**:\n", symbol))
		summary.WriteString(fmt.Sprintf("- 方向: %s (%s)\n", sideCN, strings.ToUpper(position.Side)))
		summary.WriteString(fmt.Sprintf("- 数量: %.4f\n", position.Size))
		summary.WriteString(fmt.Sprintf("- 开仓价格: $%.2f\n", position.EntryPrice))
		summary.WriteString(fmt.Sprintf("- 当前价格: $%.2f\n", currentPrice))
		summary.WriteString(fmt.Sprintf("- 未实现盈亏: %+.2f USDT (%+.2f%%)\n", position.UnrealizedPnL, pnlPct))

		if position.LiquidationPrice > 0 {
			summary.WriteString(fmt.Sprintf("- 爆仓价格: $%.2f\n", position.LiquidationPrice))
		}

		// Provide suggestions
		if pnlPct < -5 {
			summary.WriteString(fmt.Sprintf("\n⚠️ **警告**: 当前浮亏 %.2f%%，已超过 -5%%，建议考虑止损\n", pnlPct))
		} else if pnlPct > 3 {
			summary.WriteString(fmt.Sprintf("\n✅ **盈利中**: 当前浮盈 %.2f%%，已超过 +3%%，可考虑止盈或继续持有\n", pnlPct))
		} else {
			summary.WriteString("\n📊 **状态正常**: 当前盈亏在合理范围内\n")
		}
	} else {
		summary.WriteString(fmt.Sprintf("**当前持仓 %s**: 无持仓\n", symbol))
		summary.WriteString("\n💡 **建议**: 可以根据市场分析开新仓位\n")
	}

	return summary.String()
}

// withRetry executes a function with exponential backoff retry
func (e *BinanceExecutor) withRetry(fn func() error) error {
	b := &backoff.Backoff{
		Min:    2 * time.Second,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	maxRetries := 3
	for i := 0; i <= maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		if i == maxRetries {
			return fmt.Errorf("max retries reached: %w", err)
		}

		duration := b.Duration()
		e.logger.Warning(fmt.Sprintf("操作失败 (尝试 %d/%d): %v，等待 %.1f 秒后重试...",
			i+1, maxRetries, err, duration.Seconds()))
		time.Sleep(duration)
	}

	return nil
}

// GetAccountInfo gets account information from Binance
// GetAccountInfo 从币安获取账户信息
func (e *BinanceExecutor) GetAccountInfo(ctx context.Context) (*futures.Account, error) {
	return e.client.NewGetAccountService().Do(ctx)
}

// Helper functions
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// Position helper methods
// Position 辅助方法

// GetUnrealizedPnL calculates unrealized profit/loss percentage
// GetUnrealizedPnL 计算未实现盈亏百分比
func (p *Position) GetUnrealizedPnL() float64 {
	if p.Side == "long" {
		return (p.CurrentPrice - p.EntryPrice) / p.EntryPrice
	}
	// For short positions
	// 空仓
	return (p.EntryPrice - p.CurrentPrice) / p.EntryPrice
}

// GetUnrealizedPnLUSDT calculates unrealized profit/loss in USDT
// GetUnrealizedPnLUSDT 计算 USDT 计价的未实现盈亏
func (p *Position) GetUnrealizedPnLUSDT() float64 {
	return p.GetUnrealizedPnL() * p.EntryPrice * p.Quantity
}

// GetHoldingDuration returns how long the position has been held
// GetHoldingDuration 返回持仓时间
func (p *Position) GetHoldingDuration() time.Duration {
	return time.Since(p.EntryTime)
}

// ShouldTriggerStopLoss checks if stop-loss should be triggered
// ShouldTriggerStopLoss 检查是否应该触发止损
func (p *Position) ShouldTriggerStopLoss() bool {
	if p.Side == "long" {
		return p.CurrentPrice <= p.CurrentStopLoss
	}
	// For short positions
	// 空仓
	return p.CurrentPrice >= p.CurrentStopLoss
}

// GetRiskRewardRatio calculates current risk/reward ratio
// GetRiskRewardRatio 计算当前盈亏比
func (p *Position) GetRiskRewardRatio() float64 {
	risk := p.EntryPrice - p.InitialStopLoss
	if risk <= 0 {
		return 0
	}

	reward := p.CurrentPrice - p.EntryPrice
	if p.Side == "short" {
		reward = p.EntryPrice - p.CurrentPrice
	}

	return reward / risk
}

// UpdatePrice updates current price and highest/lowest price
// UpdatePrice 更新当前价格和最高/最低价
func (p *Position) UpdatePrice(newPrice float64) {
	p.CurrentPrice = newPrice

	// Update highest price for long positions
	// 更新多仓的最高价
	if p.Side == "long" {
		if newPrice > p.HighestPrice {
			p.HighestPrice = newPrice
		}
	} else {
		// Update lowest price for short positions
		// 更新空仓的最低价
		if p.HighestPrice == 0 || newPrice < p.HighestPrice {
			p.HighestPrice = newPrice
		}
	}

	// Add to price history (limit to last 1000 points)
	// 添加到价格历史（限制最近 1000 个点）
	p.PriceHistory = append(p.PriceHistory, PricePoint{
		Time:  time.Now(),
		Price: newPrice,
	})
	if len(p.PriceHistory) > 1000 {
		p.PriceHistory = p.PriceHistory[1:]
	}
}

// AddStopLossEvent adds a stop-loss change event to history
// AddStopLossEvent 添加止损变更事件到历史记录
func (p *Position) AddStopLossEvent(oldStop, newStop float64, reason, trigger string) {
	event := StopLossEvent{
		Time:    time.Now(),
		OldStop: oldStop,
		NewStop: newStop,
		Reason:  reason,
		Trigger: trigger,
	}
	p.StopLossHistory = append(p.StopLossHistory, event)
}

// GetStopLossHistoryString returns formatted stop-loss history
// GetStopLossHistoryString 返回格式化的止损历史字符串
func (p *Position) GetStopLossHistoryString() string {
	if len(p.StopLossHistory) == 0 {
		return "无止损变更历史"
	}

	result := ""
	for i, event := range p.StopLossHistory {
		result += fmt.Sprintf("%d. %s: %.2f → %.2f (%s, 由%s触发)\n",
			i+1,
			event.Time.Format("15:04:05"),
			event.OldStop,
			event.NewStop,
			event.Reason,
			event.Trigger)
	}
	return result
}
