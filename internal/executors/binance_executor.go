package executors

import (
	"context"
	"fmt"
	"math"
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
	Side             string
	Size             float64
	EntryPrice       float64
	UnrealizedPnL    float64
	PositionAmt      float64
	Symbol           string
	Leverage         int
	LiquidationPrice float64
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
func NewBinanceExecutor(cfg *config.Config, log *logger.ColorLogger) *BinanceExecutor {
	futures.UseTestnet = cfg.BinanceTestMode

	client := futures.NewClient(cfg.BinanceAPIKey, cfg.BinanceAPISecret)

	// Set proxy if configured
	if cfg.BinanceProxy != "" {
		// Note: go-binance doesn't directly support proxy, need to configure HTTP client
		// This can be done via http.DefaultTransport if needed
	}

	executor := &BinanceExecutor{
		client:       client,
		config:       cfg,
		testMode:     cfg.BinanceTestMode,
		logger:       log,
		tradeHistory: make([]TradeResult, 0),
	}

	// Print mode
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
		e.logger.Success(fmt.Sprintf("使用配置的持仓模式：%s", modeName))
		return nil
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
			Symbol(e.config.GetBinanceSymbol()).
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
			Symbol(e.config.GetBinanceSymbol()).
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
	binanceSymbol := e.config.GetBinanceSymbol()

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
	binanceSymbol := e.config.GetBinanceSymbol()

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
	binanceSymbol := e.config.GetBinanceSymbol()
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
	binanceSymbol := e.config.GetBinanceSymbol()
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
		ticker, _ := e.client.NewListPriceChangeStatsService().Symbol(e.config.GetBinanceSymbol()).Do(ctx)
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