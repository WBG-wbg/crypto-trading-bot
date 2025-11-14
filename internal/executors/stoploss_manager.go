package executors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/storage"
)

// StopLossManager manages stop-loss for all active positions
// StopLossManager 管理所有活跃持仓的止损
//
// Architecture: Server-side stop-loss strategy (Fixed stop-loss)
// 架构：服务器端止损策略（固定止损）
//
// Responsibilities:
// 职责：
//  1. Position lifecycle management (register, remove, query)
//     持仓生命周期管理（注册、移除、查询）
//  2. Binance stop-loss order placement and cancellation
//     币安止损单下单和取消
//  3. Position data storage and retrieval
//     持仓数据存储和检索
//
// Note: Local price monitoring is DISABLED. Stop-loss execution relies entirely on
// Binance server-side STOP_MARKET orders, which provide:
// 注意：本地价格监控已禁用。止损执行完全依赖币安服务器端 STOP_MARKET 订单，优势：
//   - 24/7 server-side monitoring (no local uptime dependency)
//     24/7 服务器端监控（不依赖本地程序运行）
//   - Millisecond-level trigger speed (vs 10s polling)
//     毫秒级触发速度（相比 10 秒轮询）
//   - Resilience to local program crashes/network issues
//     对本地程序崩溃/网络问题有弹性
//   - No duplicate execution risk
//     无重复执行风险
type StopLossManager struct {
	positions map[string]*Position // symbol -> Position
	executor  *BinanceExecutor     // 执行器 / Executor
	config    *config.Config       // 配置 / Config
	logger    *logger.ColorLogger  // 日志 / Logger
	storage   *storage.Storage     // 数据库 / Database
	mu        sync.RWMutex         // 读写锁 / RW mutex
	ctx       context.Context      // 上下文 / Context
	cancel    context.CancelFunc   // 取消函数 / Cancel function
}

// NewStopLossManager creates a new StopLossManager
// NewStopLossManager 创建新的止损管理器
func NewStopLossManager(cfg *config.Config, executor *BinanceExecutor, log *logger.ColorLogger, db *storage.Storage) *StopLossManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &StopLossManager{
		positions: make(map[string]*Position),
		executor:  executor,
		config:    cfg,
		logger:    log,
		storage:   db,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// RegisterPosition registers a new position for stop-loss management
// RegisterPosition 注册新持仓进行止损管理
func (sm *StopLossManager) RegisterPosition(pos *Position) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	pos.HighestPrice = pos.EntryPrice // 初始化最高价/最低价 / Initialize highest/lowest
	pos.CurrentPrice = pos.EntryPrice
	pos.StopLossType = "fixed" // LLM 驱动的固定止损 / LLM-driven fixed stop

	sm.positions[pos.Symbol] = pos
	sm.logger.Success(fmt.Sprintf("【%s】持仓已注册，入场价: %.2f, 初始止损: %.2f",
		pos.Symbol, pos.EntryPrice, pos.InitialStopLoss))
}

// RemovePosition removes a position from management
// RemovePosition 从管理中移除持仓
func (sm *StopLossManager) RemovePosition(symbol string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.positions, symbol)
	sm.logger.Info(fmt.Sprintf("【%s】持仓已移除", symbol))
}

// ClosePosition closes a position completely: cancels stop-loss order, removes from memory, and updates database
// ClosePosition 完整关闭持仓：取消止损单、从内存移除、更新数据库
func (sm *StopLossManager) ClosePosition(ctx context.Context, symbol string, closePrice float64, closeReason string, realizedPnL float64) error {
	sm.mu.Lock()
	pos, exists := sm.positions[symbol]
	sm.mu.Unlock()

	if !exists {
		sm.logger.Warning(fmt.Sprintf("⚠️  %s 持仓不存在，无需关闭", symbol))
		return nil
	}

	sm.logger.Info(fmt.Sprintf("【%s】正在关闭持仓...", symbol))

	// Step 1: Cancel Binance stop-loss order
	// 步骤 1：取消币安止损单
	if pos.StopLossOrderID != "" {
		if err := sm.cancelStopLossOrder(ctx, pos); err != nil {
			sm.logger.Warning(fmt.Sprintf("⚠️  取消 %s 止损单失败: %v（继续关闭流程）", symbol, err))
		} else {
			sm.logger.Success(fmt.Sprintf("✅ %s 止损单已取消", symbol))
		}
	}

	// Step 2: Remove from memory
	// 步骤 2：从内存移除
	sm.mu.Lock()
	delete(sm.positions, symbol)
	sm.mu.Unlock()
	sm.logger.Info(fmt.Sprintf("✅ %s 已从止损管理器移除", symbol))

	// Step 3: Update database status
	// 步骤 3：更新数据库状态
	if sm.storage != nil {
		// Get position record from database
		// 从数据库获取持仓记录
		posRecord, err := sm.storage.GetPositionByID(pos.ID)
		if err != nil {
			sm.logger.Warning(fmt.Sprintf("⚠️  获取 %s 持仓记录失败: %v（跳过数据库更新）", symbol, err))
		} else if posRecord != nil {
			// Update position record
			// 更新持仓记录
			now := time.Now()
			posRecord.Closed = true
			posRecord.CloseTime = &now
			posRecord.ClosePrice = closePrice
			posRecord.CloseReason = closeReason
			posRecord.RealizedPnL = realizedPnL

			if err := sm.storage.UpdatePosition(posRecord); err != nil {
				sm.logger.Warning(fmt.Sprintf("⚠️  更新 %s 数据库状态失败: %v", symbol, err))
			} else {
				sm.logger.Success(fmt.Sprintf("✅ %s 数据库状态已更新为已关闭", symbol))
			}
		}
	}

	sm.logger.Success(fmt.Sprintf("✅【%s】持仓完全关闭（止损单已取消，内存已清理，数据库已更新）", symbol))
	return nil
}

// PlaceInitialStopLoss places initial stop-loss order for a position
// PlaceInitialStopLoss 为持仓下初始止损单
func (sm *StopLossManager) PlaceInitialStopLoss(ctx context.Context, pos *Position) error {
	return sm.placeStopLossOrder(ctx, pos, pos.InitialStopLoss)
}

// GetPosition gets a position by symbol
// GetPosition 根据交易对获取持仓
func (sm *StopLossManager) GetPosition(symbol string) *Position {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.positions[symbol]
}

// UpdateStopLoss updates stop-loss price for a position (called by LLM every 15 minutes)
// UpdateStopLoss 更新持仓的止损价格（每 15 分钟由 LLM 调用）
func (sm *StopLossManager) UpdateStopLoss(ctx context.Context, symbol string, newStopLoss float64, reason string) error {
	sm.mu.Lock()
	pos, exists := sm.positions[symbol]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("持仓 %s 不存在", symbol)
	}
	sm.mu.Unlock()

	oldStop := pos.CurrentStopLoss

	// Validate stop-loss movement (only allow favorable direction)
	// 验证止损移动（只允许朝有利方向移动）
	if pos.Side == "long" && newStopLoss < oldStop {
		sm.logger.Warning(fmt.Sprintf("【%s】⚠️ LLM 建议降低多仓止损 (%.2f → %.2f)，拒绝（止损只能向上移动）",
			pos.Symbol, oldStop, newStopLoss))
		return fmt.Errorf("多仓止损只能向上移动")
	}
	if pos.Side == "short" && newStopLoss > oldStop {
		sm.logger.Warning(fmt.Sprintf("【%s】⚠️ LLM 建议提高空仓止损 (%.2f → %.2f)，拒绝（止损只能向下移动）",
			pos.Symbol, oldStop, newStopLoss))
		return fmt.Errorf("空仓止损只能向下移动")
	}

	// Record history
	// 记录历史
	pos.AddStopLossEvent(oldStop, newStopLoss, reason, "llm")

	// Cancel old stop-loss order if exists
	// 取消旧的止损单（如果存在）
	if pos.StopLossOrderID != "" {
		if err := sm.cancelStopLossOrder(ctx, pos); err != nil {
			sm.logger.Warning(fmt.Sprintf("取消旧止损单失败: %v", err))
			// Continue anyway / 继续执行
		}
	}

	// Place new stop-loss order
	// 下新的止损单
	if err := sm.placeStopLossOrder(ctx, pos, newStopLoss); err != nil {
		return fmt.Errorf("下止损单失败: %w", err)
	}

	pos.CurrentStopLoss = newStopLoss
	sm.logger.Success(fmt.Sprintf("【%s】✅ LLM 止损已更新: %.2f → %.2f (%s)",
		pos.Symbol, oldStop, newStopLoss, reason))

	return nil
}

// UpdatePosition updates position price and checks if stop-loss should trigger
// UpdatePosition 更新持仓价格并检查是否应触发止损
//
// DEPRECATED: This method is part of the deprecated local monitoring system.
// 已弃用：此方法是已弃用的本地监控系统的一部分。
// Use Binance server-side STOP_MARKET orders instead.
// 请使用币安服务器端 STOP_MARKET 订单。
func (sm *StopLossManager) UpdatePosition(ctx context.Context, symbol string, currentPrice float64) error {
	sm.mu.Lock()
	pos, exists := sm.positions[symbol]
	if !exists {
		sm.mu.Unlock()
		return nil // 无持仓 / No position
	}
	sm.mu.Unlock()

	// Update price
	// 更新价格
	pos.UpdatePrice(currentPrice)

	// Check if stop-loss should be triggered (simple fixed stop-loss check)
	// 检查是否应该触发止损（简单的固定止损检查）
	if pos.ShouldTriggerStopLoss() {
		sm.logger.Warning(fmt.Sprintf("【%s】触发止损！当前价: %.2f, 止损价: %.2f",
			pos.Symbol, pos.CurrentPrice, pos.CurrentStopLoss))
		return sm.executeStopLoss(ctx, pos)
	}

	return nil
}

// placeStopLossOrder places a stop-loss order on Binance
// placeStopLossOrder 在币安下止损单
func (sm *StopLossManager) placeStopLossOrder(ctx context.Context, pos *Position, stopPrice float64) error {
	// Get current market price for validation
	// 获取当前市场价格用于验证
	currentPrice, err := sm.getCurrentPrice(ctx, pos.Symbol)
	if err != nil {
		return fmt.Errorf("获取当前价格失败: %w", err)
	}

	// Validate stop-loss price to prevent immediate trigger
	// 验证止损价格以防止立即触发
	if pos.Side == "short" {
		// 空仓止损买入：止损价格必须高于当前市场价
		if stopPrice <= currentPrice {
			sm.logger.Warning(fmt.Sprintf("【%s】❌ 空仓止损价格设置错误: %.2f <= 当前价 %.2f (会立即触发)",
				pos.Symbol, stopPrice, currentPrice))
			return fmt.Errorf("空仓止损价格 %.2f 必须高于当前市场价 %.2f，否则会立即触发", stopPrice, currentPrice)
		}
	} else {
		// 多仓止损卖出：止损价格必须低于当前市场价
		if stopPrice >= currentPrice {
			sm.logger.Warning(fmt.Sprintf("【%s】❌ 多仓止损价格设置错误: %.2f >= 当前价 %.2f (会立即触发)",
				pos.Symbol, stopPrice, currentPrice))
			return fmt.Errorf("多仓止损价格 %.2f 必须低于当前市场价 %.2f，否则会立即触发", stopPrice, currentPrice)
		}
	}

	var orderSide futures.SideType
	if pos.Side == "short" {
		orderSide = futures.SideTypeBuy
	} else {
		orderSide = futures.SideTypeSell
	}

	binanceSymbol := sm.config.GetBinanceSymbolFor(pos.Symbol)

	// Create stop-loss order
	// 创建止损单
	order, err := sm.executor.client.NewCreateOrderService().
		Symbol(binanceSymbol).
		Side(orderSide).
		Type(futures.OrderTypeStopMarket).
		StopPrice(fmt.Sprintf("%.2f", stopPrice)).
		Quantity(fmt.Sprintf("%.4f", pos.Quantity)).
		ReduceOnly(true). // 只平仓不开仓 / Close only
		Do(ctx)

	if err != nil {
		return fmt.Errorf("下止损单失败: %w", err)
	}

	pos.StopLossOrderID = fmt.Sprintf("%d", order.OrderID)
	sm.logger.Success(fmt.Sprintf("【%s】止损单已下达: %.2f (订单ID: %s, 当前价: %.2f)",
		pos.Symbol, stopPrice, pos.StopLossOrderID, currentPrice))

	return nil
}

// cancelStopLossOrder cancels an existing stop-loss order
// cancelStopLossOrder 取消现有的止损单
func (sm *StopLossManager) cancelStopLossOrder(ctx context.Context, pos *Position) error {
	if pos.StopLossOrderID == "" {
		return nil
	}

	binanceSymbol := sm.config.GetBinanceSymbolFor(pos.Symbol)

	_, err := sm.executor.client.NewCancelOrderService().
		Symbol(binanceSymbol).
		OrderID(parseInt64(pos.StopLossOrderID)).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("取消止损单失败: %w", err)
	}

	sm.logger.Info(fmt.Sprintf("【%s】旧止损单已取消: %s", pos.Symbol, pos.StopLossOrderID))
	pos.StopLossOrderID = ""

	return nil
}

// executeStopLoss executes stop-loss (close position)
// executeStopLoss 执行止损（平仓）
//
// DEPRECATED: This method is part of the deprecated local monitoring system.
// 已弃用：此方法是已弃用的本地监控系统的一部分。
// Binance STOP_MARKET orders handle stop-loss execution automatically.
// 币安 STOP_MARKET 订单会自动处理止损执行。
func (sm *StopLossManager) executeStopLoss(ctx context.Context, pos *Position) error {
	sm.logger.Warning(fmt.Sprintf("【%s】🛑 执行止损平仓", pos.Symbol))

	// Close position via market order
	// 通过市价单平仓
	action := ActionCloseLong
	if pos.Side == "short" {
		action = ActionCloseShort
	}

	result := sm.executor.ExecuteTrade(ctx, pos.Symbol, action, pos.Quantity, "触发止损")

	if result.Success {
		sm.logger.Success(fmt.Sprintf("【%s】止损平仓成功，盈亏: %.2f%%",
			pos.Symbol, pos.GetUnrealizedPnL()*100))
		sm.RemovePosition(pos.Symbol)
	} else {
		sm.logger.Error(fmt.Sprintf("【%s】止损平仓失败: %s", pos.Symbol, result.Message))
		return fmt.Errorf("止损平仓失败: %s", result.Message)
	}

	return nil
}

// MonitorPositions monitors all positions in real-time (every 10 seconds)
// MonitorPositions 实时监控所有持仓（每 10 秒）
//
// DEPRECATED: This method is deprecated and should NOT be used with fixed stop-loss strategy.
// 已弃用：此方法已弃用，不应与固定止损策略一起使用。
//
// Reason: With Binance server-side STOP_MARKET orders, local monitoring is redundant and can cause issues:
// 原因：使用币安服务器端 STOP_MARKET 订单时，本地监控是多余的，可能导致问题：
//  1. Duplicate execution: Both Binance and local monitoring may try to close the position
//     重复执行：币安和本地监控可能都尝试平仓
//  2. API overhead: Polling price every 10 seconds for all positions
//     API 开销：每 10 秒为所有持仓轮询价格
//  3. Slower than Binance: 10s polling vs millisecond server-side trigger
//     比币安慢：10 秒轮询 vs 毫秒级服务器端触发
//  4. Reliability: Depends on local program uptime and network stability
//     可靠性：依赖本地程序运行和网络稳定性
//
// For fixed stop-loss strategy, rely entirely on Binance STOP_MARKET orders placed via PlaceInitialStopLoss().
// 对于固定止损策略，完全依赖通过 PlaceInitialStopLoss() 下达的币安 STOP_MARKET 订单。
func (sm *StopLossManager) MonitorPositions(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sm.logger.Info(fmt.Sprintf("🔍 启动持仓监控，间隔: %v", interval))

	for {
		select {
		case <-sm.ctx.Done():
			sm.logger.Info("持仓监控已停止")
			return

		case <-ticker.C:
			sm.mu.RLock()
			positions := make([]*Position, 0, len(sm.positions))
			for _, pos := range sm.positions {
				positions = append(positions, pos)
			}
			sm.mu.RUnlock()

			for _, pos := range positions {
				// Get latest price from Binance
				// 从币安获取最新价格
				currentPrice, err := sm.getCurrentPrice(sm.ctx, pos.Symbol)
				if err != nil {
					sm.logger.Warning(fmt.Sprintf("获取 %s 价格失败: %v", pos.Symbol, err))
					continue
				}

				// Update position and check stop-loss trigger
				// 更新持仓并检查止损触发
				if err := sm.UpdatePosition(sm.ctx, pos.Symbol, currentPrice); err != nil {
					sm.logger.Error(fmt.Sprintf("更新 %s 持仓失败: %v", pos.Symbol, err))
				}
			}
		}
	}
}

// getCurrentPrice gets current price from Binance
// getCurrentPrice 从币安获取当前价格
func (sm *StopLossManager) getCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	binanceSymbol := sm.config.GetBinanceSymbolFor(symbol)

	prices, err := sm.executor.client.NewListPricesService().
		Symbol(binanceSymbol).
		Do(ctx)

	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("未获取到价格数据")
	}

	price, err := parseFloat(prices[0].Price)
	if err != nil {
		return 0, fmt.Errorf("解析价格失败: %w", err)
	}

	return price, nil
}

// GetAllPositions returns all active positions
// GetAllPositions 返回所有活跃持仓
func (sm *StopLossManager) GetAllPositions() []*Position {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	positions := make([]*Position, 0, len(sm.positions))
	for _, pos := range sm.positions {
		positions = append(positions, pos)
	}
	return positions
}

// Stop stops the stop-loss manager
// Stop 停止止损管理器
func (sm *StopLossManager) Stop() {
	sm.cancel()
}

// Helper function to parse int64
// 辅助函数：解析 int64
func parseInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}
