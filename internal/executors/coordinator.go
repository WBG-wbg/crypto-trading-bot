package executors

import (
	"context"
	"fmt"
	"time"

	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
)

// TradeCoordinator coordinates the entire trading flow from decision to execution
// TradeCoordinator 协调从决策到执行的整个交易流程
type TradeCoordinator struct {
	config   *config.Config
	executor *BinanceExecutor
	logger   *logger.ColorLogger
}

// NewTradeCoordinator creates a new TradeCoordinator
// NewTradeCoordinator 创建新的交易协调器
func NewTradeCoordinator(cfg *config.Config, executor *BinanceExecutor, log *logger.ColorLogger) *TradeCoordinator {
	return &TradeCoordinator{
		config:   cfg,
		executor: executor,
		logger:   log,
	}
}

// ExecuteDecision executes a trading decision with full safety checks
// ExecuteDecision 执行交易决策并进行完整的安全检查
func (tc *TradeCoordinator) ExecuteDecision(ctx context.Context, symbol string, action TradeAction, reason string) (*TradeResult, error) {
	tc.logger.Header("交易执行协调器", '=', 80)
	tc.logger.Info(fmt.Sprintf("交易对: %s", symbol))
	tc.logger.Info(fmt.Sprintf("决策动作: %s", action))
	tc.logger.Info(fmt.Sprintf("决策理由: %s", reason))

	// Step 1: Pre-execution safety checks
	// 步骤 1: 执行前安全检查
	tc.logger.Info("\n[步骤 1/5] 执行前安全检查...")
	if err := tc.preExecutionChecks(ctx, symbol, action); err != nil {
		tc.logger.Error(fmt.Sprintf("❌ 安全检查失败: %v", err))
		return nil, fmt.Errorf("pre-execution check failed: %w", err)
	}
	tc.logger.Success("✅ 安全检查通过")

	// Step 2: Get current position
	// 步骤 2: 获取当前持仓
	tc.logger.Info("\n[步骤 2/5] 获取当前持仓...")
	currentPosition, err := tc.executor.GetCurrentPosition(ctx, symbol)
	if err != nil {
		tc.logger.Warning(fmt.Sprintf("⚠️  无法获取持仓: %v，假设无持仓", err))
		currentPosition = nil
	}

	if currentPosition != nil {
		tc.logger.Info(fmt.Sprintf("当前持仓: %s %.4f @ $%.2f (盈亏: %+.2f USDT)",
			currentPosition.Side, currentPosition.Size, currentPosition.EntryPrice, currentPosition.UnrealizedPnL))
	} else {
		tc.logger.Info("当前持仓: 无")
	}

	// Step 3: Validate action against current position
	// 步骤 3: 验证动作与当前持仓的一致性
	tc.logger.Info("\n[步骤 3/5] 验证交易动作...")
	if err := tc.validateAction(action, currentPosition); err != nil {
		tc.logger.Error(fmt.Sprintf("❌ 动作验证失败: %v", err))
		return nil, fmt.Errorf("action validation failed: %w", err)
	}
	tc.logger.Success("✅ 动作验证通过")

	// Step 4: Calculate position size
	// 步骤 4: 计算仓位大小
	tc.logger.Info("\n[步骤 4/5] 计算仓位大小...")
	positionSize, err := tc.calculatePositionSize(ctx, symbol, action, currentPosition)
	if err != nil {
		tc.logger.Error(fmt.Sprintf("❌ 仓位计算失败: %v", err))
		return nil, fmt.Errorf("position size calculation failed: %w", err)
	}
	tc.logger.Info(fmt.Sprintf("仓位大小: %.4f", positionSize))

	// Step 5: Execute the trade
	// 步骤 5: 执行交易
	tc.logger.Info("\n[步骤 5/5] 执行交易...")

	if action == ActionHold {
		tc.logger.Info("💤 观望决策，不执行交易")
		return &TradeResult{
			Success:   true,
			Action:    action,
			Symbol:    symbol,
			Amount:    0,
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			Reason:    reason,
			TestMode:  tc.config.BinanceTestMode,
			Message:   "观望，不执行交易",
		}, nil
	}

	result := tc.executor.ExecuteTrade(ctx, symbol, action, positionSize, reason)

	// Step 6: Post-execution verification
	// 步骤 6: 执行后验证
	tc.logger.Info("\n[步骤 6/6] 执行后验证...")
	if result.Success {
		if err := tc.postExecutionVerification(ctx, symbol, action, result); err != nil {
			tc.logger.Warning(fmt.Sprintf("⚠️  执行后验证发现问题: %v", err))
		} else {
			tc.logger.Success("✅ 执行后验证通过")
		}
	}

	return result, nil
}

// preExecutionChecks performs safety checks before executing a trade
// preExecutionChecks 在执行交易前进行安全检查
func (tc *TradeCoordinator) preExecutionChecks(ctx context.Context, symbol string, action TradeAction) error {
	// Check 1: Verify balance
	// 检查 1: 验证余额
	account, err := tc.executor.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return fmt.Errorf("无法获取账户信息: %w", err)
	}

	var availableBalance float64
	for _, asset := range account.Assets {
		if asset.Asset == "USDT" {
			fmt.Sscanf(asset.AvailableBalance, "%f", &availableBalance)
			break
		}
	}

	if availableBalance < 10.0 { // Minimum balance check
		return fmt.Errorf("可用余额不足: %.2f USDT < 10 USDT", availableBalance)
	}

	tc.logger.Info(fmt.Sprintf("  ✓ 账户余额: %.2f USDT", availableBalance))

	// Check 2: Verify symbol exists and is trading
	// 检查 2: 验证交易对存在且正在交易
	binanceSymbol := tc.config.GetBinanceSymbolFor(symbol)
	ticker, err := tc.executor.client.NewListPriceChangeStatsService().Symbol(binanceSymbol).Do(ctx)
	if err != nil {
		return fmt.Errorf("无法获取交易对价格: %w", err)
	}

	if len(ticker) == 0 {
		return fmt.Errorf("交易对 %s 不存在或未在交易", binanceSymbol)
	}

	tc.logger.Info(fmt.Sprintf("  ✓ 交易对状态: 正常交易"))

	return nil
}

// validateAction validates the action against current position
// validateAction 验证动作与当前持仓的一致性
func (tc *TradeCoordinator) validateAction(action TradeAction, currentPosition *Position) error {
	if currentPosition == nil {
		// No position, only BUY and SELL are valid
		// 无持仓，只有 BUY 和 SELL 有效
		if action != ActionBuy && action != ActionSell && action != ActionHold {
			return fmt.Errorf("无持仓时只能执行 BUY、SELL 或 HOLD 动作，当前: %s", action)
		}
		return nil
	}

	// Has position, validate close actions
	// 有持仓，验证平仓动作
	switch action {
	case ActionBuy:
		if currentPosition.Side == "long" {
			return fmt.Errorf("已有多仓，不能重复开多")
		}
	case ActionSell:
		if currentPosition.Side == "short" {
			return fmt.Errorf("已有空仓，不能重复开空")
		}
	case ActionCloseLong:
		if currentPosition.Side != "long" {
			return fmt.Errorf("当前无多仓，无法平多")
		}
	case ActionCloseShort:
		if currentPosition.Side != "short" {
			return fmt.Errorf("当前无空仓，无法平空")
		}
	}

	return nil
}

// calculatePositionSize calculates the position size for the trade
// calculatePositionSize 计算交易的仓位大小
func (tc *TradeCoordinator) calculatePositionSize(ctx context.Context, symbol string, action TradeAction, currentPosition *Position) (float64, error) {
	// For close actions, use the current position size
	// 平仓动作使用当前持仓大小
	if action == ActionCloseLong || action == ActionCloseShort {
		if currentPosition == nil {
			return 0, fmt.Errorf("无持仓可平")
		}
		return currentPosition.Size, nil
	}

	// For open actions, use configured position size
	// 开仓动作使用配置的仓位大小
	return tc.config.PositionSize, nil
}

// postExecutionVerification verifies the trade was executed correctly
// postExecutionVerification 验证交易是否正确执行
func (tc *TradeCoordinator) postExecutionVerification(ctx context.Context, symbol string, action TradeAction, result *TradeResult) error {
	// Wait a moment for the order to be processed
	// 等待订单处理
	time.Sleep(2 * time.Second)

	// Get updated position
	// 获取更新后的持仓
	newPosition, err := tc.executor.GetCurrentPosition(ctx, symbol)
	if err != nil {
		return fmt.Errorf("无法获取更新后的持仓: %w", err)
	}

	// Verify position state matches expected
	// 验证持仓状态是否符合预期
	switch action {
	case ActionBuy:
		if newPosition == nil || newPosition.Side != "long" {
			return fmt.Errorf("开多后应有多仓，但当前持仓状态不符")
		}
		tc.logger.Info(fmt.Sprintf("  ✓ 多仓已建立: %.4f @ $%.2f", newPosition.Size, newPosition.EntryPrice))

	case ActionSell:
		if newPosition == nil || newPosition.Side != "short" {
			return fmt.Errorf("开空后应有空仓，但当前持仓状态不符")
		}
		tc.logger.Info(fmt.Sprintf("  ✓ 空仓已建立: %.4f @ $%.2f", newPosition.Size, newPosition.EntryPrice))

	case ActionCloseLong, ActionCloseShort:
		if newPosition != nil && newPosition.Size > 0.0001 {
			return fmt.Errorf("平仓后应无持仓，但当前仍有持仓: %.4f", newPosition.Size)
		}
		tc.logger.Info("  ✓ 持仓已平仓")
	}

	return nil
}

// GetExecutionSummary returns a summary of the execution
// GetExecutionSummary 返回执行摘要
func (tc *TradeCoordinator) GetExecutionSummary(result *TradeResult) string {
	summary := "\n"
	summary += "================================================================================\n"
	summary += "                           交易执行摘要\n"
	summary += "================================================================================\n\n"

	if result.Success {
		summary += "✅ 执行状态: 成功\n"
	} else {
		summary += "❌ 执行状态: 失败\n"
	}

	summary += fmt.Sprintf("交易对: %s\n", result.Symbol)
	summary += fmt.Sprintf("动作: %s\n", result.Action)
	summary += fmt.Sprintf("数量: %.4f\n", result.Amount)
	summary += fmt.Sprintf("时间: %s\n", result.Timestamp)
	summary += fmt.Sprintf("理由: %s\n", result.Reason)

	if result.TestMode {
		summary += "\n⚠️  注意: 这是测试模式，未实际执行交易\n"
	}

	if result.OrderID != "" {
		summary += fmt.Sprintf("\n订单ID: %s\n", result.OrderID)
	}

	if result.NewPosition != nil {
		summary += "\n当前持仓:\n"
		summary += fmt.Sprintf("  方向: %s\n", result.NewPosition.Side)
		summary += fmt.Sprintf("  数量: %.4f\n", result.NewPosition.Size)
		summary += fmt.Sprintf("  入场价: $%.2f\n", result.NewPosition.EntryPrice)
		summary += fmt.Sprintf("  未实现盈亏: %+.2f USDT\n", result.NewPosition.UnrealizedPnL)
	}

	summary += "\n" + result.Message + "\n"
	summary += "================================================================================\n"

	return summary
}
