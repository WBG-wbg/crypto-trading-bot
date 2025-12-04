package executors

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/storage"
)

// TakeProfitLevel represents a single take-profit level
// TakeProfitLevel 表示单个止盈级别
type TakeProfitLevel struct {
	Level          int     // 级别（1, 2, 3...）/ Level number
	RiskRewardRatio float64 // 风险回报比（1R, 2R, 3R）/ Risk-reward ratio
	Percentage     float64 // 平仓比例（0.3 = 30%）/ Close percentage
	TargetPrice    float64 // 目标价格 / Target price
	Executed       bool    // 是否已执行 / Whether executed
	ExecutedTime   *time.Time // 执行时间 / Execution time
	ExecutedPrice  float64 // 实际执行价格 / Actual execution price
	NewStopLoss    float64 // 执行后新止损价 / New stop-loss after execution
}

// TakeProfitConfig represents the configuration for partial take-profit
// TakeProfitConfig 表示分批止盈的配置
type TakeProfitConfig struct {
	Enabled bool                // 是否启用 / Whether enabled
	Levels  []*TakeProfitLevel  // 止盈级别列表 / List of TP levels
}

// TakeProfitManager manages partial take-profit for positions
// TakeProfitManager 管理持仓的分批止盈
//
// Responsibilities:
// 职责：
//  1. Calculate take-profit levels based on initial stop-loss distance
//     根据初始止损距离计算止盈级别
//  2. Monitor price and trigger partial closes when targets are reached
//     监控价格并在达到目标时触发部分平仓
//  3. Adjust stop-loss after each take-profit execution
//     每次止盈执行后调整止损
//  4. Coordinate with trailing stop to ensure proper floor protection
//     与追踪止损协调以确保适当的底线保护
type TakeProfitManager struct {
	executor *BinanceExecutor
	config   *config.Config
	logger   *logger.ColorLogger
	storage  *storage.Storage
	mu       sync.RWMutex
}

// NewTakeProfitManager creates a new TakeProfitManager
// NewTakeProfitManager 创建新的分批止盈管理器
func NewTakeProfitManager(cfg *config.Config, executor *BinanceExecutor, log *logger.ColorLogger, db *storage.Storage) *TakeProfitManager {
	return &TakeProfitManager{
		executor: executor,
		config:   cfg,
		logger:   log,
		storage:  db,
	}
}

// InitializeTakeProfitLevels initializes take-profit levels for a new position
// InitializeTakeProfitLevels 为新持仓初始化止盈级别
//
// Parameters:
// 参数：
//   - pos: Position to initialize / 要初始化的持仓
//
// This method calculates take-profit target prices based on:
// 此方法基于以下内容计算止盈目标价：
//   - Initial stop-loss distance (risk)
//     初始止损距离（风险）
//   - Risk-reward ratios (1R, 2R, 3R)
//     风险回报比（1R, 2R, 3R）
//   - Position side (long/short)
//     持仓方向（多/空）
func (tm *TakeProfitManager) InitializeTakeProfitLevels(pos *Position) {
	// Calculate risk distance (distance from entry to initial stop-loss)
	// 计算风险距离（入场价到初始止损的距离）
	riskDistance := math.Abs(pos.EntryPrice - pos.InitialStopLoss)

	// Default configuration: 3 levels
	// 默认配置：3个级别
	// Level 1: 30% at 1R (risk-reward ratio 1:1)
	// Level 2: 30% at 2R (risk-reward ratio 1:2)
	// Level 3: 40% at 3R (risk-reward ratio 1:3)
	levels := []*TakeProfitLevel{
		{
			Level:           1,
			RiskRewardRatio: 1.0,
			Percentage:      0.30,
			Executed:        false,
		},
		{
			Level:           2,
			RiskRewardRatio: 2.0,
			Percentage:      0.30,
			Executed:        false,
		},
		{
			Level:           3,
			RiskRewardRatio: 3.0,
			Percentage:      0.40,
			Executed:        false,
		},
	}

	// Calculate target prices based on position side
	// 根据持仓方向计算目标价格
	for _, level := range levels {
		if pos.Side == "long" {
			// Long: target = entry + (risk × ratio)
			// 多仓：目标 = 入场价 + (风险 × 比率)
			level.TargetPrice = pos.EntryPrice + (riskDistance * level.RiskRewardRatio)

			// Calculate new stop-loss after this level executes
			// 计算此级别执行后的新止损价
			switch level.Level {
			case 1:
				// After level 1: move stop to breakeven
				// 第1级后：移动止损到保本
				level.NewStopLoss = pos.EntryPrice
			case 2:
				// After level 2: move stop to level 1 target
				// 第2级后：移动止损到第1级目标价
				level.NewStopLoss = levels[0].TargetPrice
			case 3:
				// After level 3: move stop to level 2 target
				// 第3级后：移动止损到第2级目标价
				level.NewStopLoss = levels[1].TargetPrice
			}
		} else {
			// Short: target = entry - (risk × ratio)
			// 空仓：目标 = 入场价 - (风险 × 比率)
			level.TargetPrice = pos.EntryPrice - (riskDistance * level.RiskRewardRatio)

			// Calculate new stop-loss after this level executes
			// 计算此级别执行后的新止损价
			switch level.Level {
			case 1:
				// After level 1: move stop to breakeven
				// 第1级后：移动止损到保本
				level.NewStopLoss = pos.EntryPrice
			case 2:
				// After level 2: move stop to level 1 target
				// 第2级后：移动止损到第1级目标价
				level.NewStopLoss = levels[0].TargetPrice
			case 3:
				// After level 3: move stop to level 2 target
				// 第3级后：移动止损到第2级目标价
				level.NewStopLoss = levels[1].TargetPrice
			}
		}
	}

	// Initialize position's take-profit configuration
	// 初始化持仓的止盈配置
	pos.TakeProfitConfig = &TakeProfitConfig{
		Enabled: true,
		Levels:  levels,
	}

	// Log initialization
	// 记录初始化信息
	tm.logger.Success(fmt.Sprintf("【%s】分批止盈已初始化 (风险距离: %.2f)", pos.Symbol, riskDistance))
	for _, level := range levels {
		tm.logger.Info(fmt.Sprintf("  级别 %d: %.0f%% @ $%.2f (%.1fR) → 止损移至 $%.2f",
			level.Level, level.Percentage*100, level.TargetPrice, level.RiskRewardRatio, level.NewStopLoss))
	}
}

// MonitorAndExecute monitors price and executes take-profit when targets are reached
// MonitorAndExecute 监控价格并在达到目标时执行止盈
//
// Returns:
// 返回：
//   - executedLevels: Number of levels executed in this call
//     本次调用执行的级别数量
//   - error: Error if execution fails
//     执行失败时的错误
func (tm *TakeProfitManager) MonitorAndExecute(ctx context.Context, pos *Position, currentPrice float64) (int, error) {
	// Check if take-profit is enabled
	// 检查是否启用分批止盈
	if pos.TakeProfitConfig == nil || !pos.TakeProfitConfig.Enabled {
		return 0, nil
	}

	executedCount := 0

	// Check each level in order
	// 按顺序检查每个级别
	for _, level := range pos.TakeProfitConfig.Levels {
		// Skip if already executed
		// 跳过已执行的级别
		if level.Executed {
			continue
		}

		// Check if target price is reached
		// 检查是否达到目标价格
		targetReached := false
		if pos.Side == "long" {
			targetReached = currentPrice >= level.TargetPrice
		} else {
			targetReached = currentPrice <= level.TargetPrice
		}

		if !targetReached {
			// Target not reached, skip to next level
			// 目标未达到，跳过到下一级
			continue
		}

		// Execute partial close
		// 执行部分平仓
		tm.logger.Info(fmt.Sprintf("【%s】🎯 触发止盈级别 %d: 当前价 $%.2f >= 目标价 $%.2f",
			pos.Symbol, level.Level, currentPrice, level.TargetPrice))

		// Calculate close quantity
		// 计算平仓数量
		closeQuantity := pos.Quantity * level.Percentage

		// Execute close order
		// 执行平仓订单
		action := ActionCloseLong
		if pos.Side == "short" {
			action = ActionCloseShort
		}

		result := tm.executor.ExecuteTrade(ctx, pos.Symbol, action, closeQuantity,
			fmt.Sprintf("分批止盈级别%d (%.1fR)", level.Level, level.RiskRewardRatio))

		if !result.Success {
			tm.logger.Error(fmt.Sprintf("❌ 执行止盈失败: %s", result.Message))
			return executedCount, fmt.Errorf("执行止盈失败: %s", result.Message)
		}

		// Mark level as executed
		// 标记级别为已执行
		now := time.Now()
		level.Executed = true
		level.ExecutedTime = &now
		level.ExecutedPrice = result.Price

		// Update position quantity
		// 更新持仓数量
		pos.Quantity -= closeQuantity

		executedCount++

		// Calculate realized PnL for this partial close
		// 计算此次部分平仓的已实现盈亏
		var partialPnL float64
		if pos.Side == "long" {
			partialPnL = (result.Price - pos.EntryPrice) * closeQuantity
		} else {
			partialPnL = (pos.EntryPrice - result.Price) * closeQuantity
		}

		tm.logger.Success(fmt.Sprintf("✅【%s】止盈级别 %d 已执行: 平仓 %.4f (%.0f%%) @ $%.2f, 盈亏: %+.2f USDT",
			pos.Symbol, level.Level, closeQuantity, level.Percentage*100, result.Price, partialPnL))

		// Check if this was the last level (close entire position)
		// 检查是否是最后一个级别（关闭整个持仓）
		allExecuted := true
		for _, l := range pos.TakeProfitConfig.Levels {
			if !l.Executed {
				allExecuted = false
				break
			}
		}

		if allExecuted {
			tm.logger.Success(fmt.Sprintf("【%s】🎊 所有止盈级别已完成，持仓已完全平仓", pos.Symbol))
			// Position will be cleaned up by caller
			// 持仓将由调用者清理
			return executedCount, nil
		}

		// Update stop-loss to new level (this is the key coordination point)
		// 更新止损到新级别（这是关键协调点）
		tm.logger.Info(fmt.Sprintf("【%s】📌 准备更新止损: %.2f → %.2f (级别 %d 后)",
			pos.Symbol, pos.CurrentStopLoss, level.NewStopLoss, level.Level))

		// The new stop-loss will serve as the minimum floor for trailing stop
		// 新的止损价将作为追踪止损的最低底线
		// This ensures trailing stop cannot move below this level
		// 这确保追踪止损不会低于此级别

		tm.logger.Success(fmt.Sprintf("【%s】✅ 止盈级别 %d 完成，剩余仓位: %.4f (%.0f%%)",
			pos.Symbol, level.Level, pos.Quantity, (pos.Quantity/(pos.Quantity+closeQuantity))*100))

		// Only execute one level per call to avoid rapid successive executions
		// 每次调用只执行一个级别，避免快速连续执行
		break
	}

	return executedCount, nil
}

// GetMinimumStopLoss returns the minimum stop-loss based on executed TP levels
// GetMinimumStopLoss 根据已执行的止盈级别返回最低止损价
//
// This is used by trailing stop to ensure it doesn't move below the TP floor
// 这被追踪止损使用，以确保不会低于止盈底线
//
// Returns:
// 返回：
//   - minStopLoss: Minimum stop-loss price (0 if no TP executed)
//     最低止损价（如果没有执行止盈则为0）
//   - hasFloor: Whether a floor exists
//     是否存在底线
func (tm *TakeProfitManager) GetMinimumStopLoss(pos *Position) (float64, bool) {
	if pos.TakeProfitConfig == nil || !pos.TakeProfitConfig.Enabled {
		return 0, false
	}

	// Find the highest executed level
	// 找到已执行的最高级别
	var lastExecutedLevel *TakeProfitLevel
	for _, level := range pos.TakeProfitConfig.Levels {
		if level.Executed {
			lastExecutedLevel = level
		} else {
			// Levels are in order, so we can break
			// 级别是按顺序的，所以可以中断
			break
		}
	}

	if lastExecutedLevel == nil {
		// No TP executed yet, no floor
		// 还没有执行止盈，没有底线
		return 0, false
	}

	// Return the new stop-loss from the last executed level
	// 返回最后执行级别的新止损价
	return lastExecutedLevel.NewStopLoss, true
}

// GetStatus returns a human-readable status of take-profit levels
// GetStatus 返回止盈级别的可读状态
func (tm *TakeProfitManager) GetStatus(pos *Position) string {
	if pos.TakeProfitConfig == nil || !pos.TakeProfitConfig.Enabled {
		return "未启用"
	}

	status := ""
	for i, level := range pos.TakeProfitConfig.Levels {
		if i > 0 {
			status += ", "
		}
		if level.Executed {
			status += fmt.Sprintf("L%d✅", level.Level)
		} else {
			status += fmt.Sprintf("L%d⏳", level.Level)
		}
	}
	return status
}

// ShouldDisableTrailingStop checks if trailing stop should be disabled
// ShouldDisableTrailingStop 检查是否应该禁用追踪止损
//
// Trailing stop is disabled when:
// 追踪止损在以下情况下禁用：
//   - First TP level is not reached yet (use trailing to protect)
//     第一个止盈级别尚未达到（使用追踪保护）
//   - After first TP level, coordinate with TP floor
//     第一个止盈级别之后，与止盈底线协调
//
// Returns: false (we always want trailing stop enabled for coordination)
// 返回：false（我们总是希望追踪止损启用以便协调）
func (tm *TakeProfitManager) ShouldDisableTrailingStop(pos *Position) bool {
	// In hybrid mode, we always keep trailing stop enabled
	// 在混合模式下，我们始终保持追踪止损启用
	// It will respect the TP floor via GetMinimumStopLoss()
	// 它将通过 GetMinimumStopLoss() 尊重止盈底线
	return false
}
