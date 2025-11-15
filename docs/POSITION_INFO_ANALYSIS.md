# 持仓信息获取与提供给 LLM 的完整分析

本文档深入分析系统如何从币安 API 获取原始持仓数据，以及如何格式化后提供给 LLM 做出交易决策。

---

## 1. 系统架构流程

```
graph.go (position_info node)
    ↓
GetPositionSummary() 方法
    ↓
GetCurrentPosition() 方法
    ↓
币安 API (GetPositionRiskService)
    ↓
格式化的 position_info 字符串
    ↓
AgentState.SetPositionInfo()
    ↓
LLM 接收数据
```

---

## 2. 原始币安 API 数据结构

### 数据获取来源
**文件**: `/internal/executors/binance_executor.go`
**方法**: `GetCurrentPosition(ctx context.Context, symbol string) (*Position, error)`
**API 服务**: `e.client.NewGetPositionRiskService()`

### 币安原始响应字段
来自 `go-binance/v2/futures` 的 PositionRisk 结构：

```go
// 币安 API 返回的原始字段
type PositionRisk struct {
    EntryPrice      string   // 开仓价格
    Leverage        string   // 杠杆倍数
    MaxNotional     string   // 未使用
    LiquidationPrice string  // 强平价格
    PositionAmt     string   // 持仓数量（正数为多，负数为空）
    Symbol          string   // 交易对
    UnRealizedProfit string  // 未实现盈亏
    RealisedProfit  string   // 已实现盈亏
    MarginType      string   // 保证金类型（cross/isolated）
    // ...
}
```

### 币安 API 中缺失的字段
根据代码分析，币安期货 API 的 Position 数据结构 **不包含** 以下字段：
- `highest_price` / `lowest_price` - 持仓期间最高/最低价
- `hold_duration` / `hold_time` - 持仓时长
- `time_on_chart` - 持仓K线数量

这些字段在 Spot API 中可能可用，但在 Futures API 中不被 Binance 提供。

### 系统处理过程
代码位置: `/internal/executors/binance_executor.go`, 第 324-372 行

```go
// GetCurrentPosition 获取当前持仓
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
                // 提取字段 / Extract fields
                entryPrice, _ := parseFloat(pos.EntryPrice)
                unrealizedPnL, _ := parseFloat(pos.UnRealizedProfit)
                liquidationPrice, _ := parseFloat(pos.LiquidationPrice)
                leverage, _ := parseInt(pos.Leverage)

                // 判断方向 / Determine side
                side := "long"
                if posAmt < 0 {
                    side = "short"
                }

                // 构建 Position 对象 / Build Position object
                position = &Position{
                    Side:             side,
                    Size:             math.Abs(posAmt),
                    EntryPrice:       entryPrice,
                    UnrealizedPnL:    unrealizedPnL,
                    PositionAmt:      posAmt,
                    Symbol:           pos.Symbol,
                    Leverage:         leverage,
                    LiquidationPrice: liquidationPrice,
                    // 注意：HighestPrice 未从币安获取 / NOTE: HighestPrice not from Binance
                    // 注意：CurrentPrice 也未从这里获取 / NOTE: CurrentPrice not from here
                }
                break
            }
        }

        return nil
    })

    return position, nil
}
```

---

## 3. Position 数据结构定义

**文件**: `/internal/executors/binance_executor.go`, 第 47-85 行

```go
// Position 代表交易持仓
type Position struct {
    // 基础持仓信息 / Basic position info
    ID               string    // 持仓 ID / Position ID
    Symbol           string    // 交易对 / Trading pair
    Side             string    // long/short
    Size             float64   // 持仓大小 / Position size (same as Quantity)
    EntryPrice       float64   // 入场价格 / Entry price ✓ 来自币安
    EntryTime        time.Time // 入场时间 / Entry time ✗ 币安不提供
    CurrentPrice     float64   // 当前价格 / Current price ✗ 币安不提供
    HighestPrice     float64   // 最高价（多仓）或最低价（空仓）/ Highest/lowest price ✗ 币安不提供
    Quantity         float64   // 持仓数量 / Quantity (same as Size) ✓ 来自币安
    UnrealizedPnL    float64   // 未实现盈亏 / Unrealized PnL ✓ 来自币安
    PositionAmt      float64   // 仓位金额 / Position amount ✓ 来自币安
    Leverage         int       // 杠杆倍数 / Leverage ✓ 来自币安
    LiquidationPrice float64   // 强平价格 / Liquidation price ✓ 来自币安

    // 止损管理 / Stop-loss management
    InitialStopLoss   float64 // 初始止损价格 / Initial stop-loss
    CurrentStopLoss   float64 // 当前止损价格 / Current stop-loss ✓ 来自 StopLossManager
    StopLossType      string  // 止损类型：fixed, breakeven, trailing
    TrailingDistance  float64 // 追踪距离（百分比）/ Trailing distance
    PartialTPExecuted bool    // 是否已执行分批止盈 / Whether partial TP has been executed
    ATR               float64 // ATR 值用于动态追踪距离 / ATR value for dynamic trailing distance

    // 订单管理 / Order management
    StopLossOrderID string // 当前止损单 ID / Stop-loss order ID

    // 历史和上下文 / History and context
    StopLossHistory []StopLossEvent // 止损变更历史 / Stop-loss history
    PriceHistory    []PricePoint    // 价格历史 / Price history
    OpenReason      string          // 开仓理由 / Opening reason
    LastLLMReview   time.Time       // 上次 LLM 复查时间 / Last LLM review
    LLMSuggestions  []string        // LLM 建议 / LLM suggestions
}
```

### 标记说明
- ✓ = 来自币安 API GetPositionRisk
- ✗ = 币安 API 不提供（需要其他来源或计算）

---

## 4. 传递给 LLM 的 position_info 格式

### 调用链
1. **graph.go** (line 473)：`posInfo := g.executor.GetPositionSummary(ctx, sym, g.stopLossManager)`
2. **graph.go** (line 474)：`g.state.SetPositionInfo(sym, posInfo)`
3. **graph.go** (line 132)：包含在 `GetAllReports()` 中发送给 LLM

### 代码实现
**文件**: `/internal/executors/binance_executor.go`, 第 649-763 行

```go
func (e *BinanceExecutor) GetPositionSummary(ctx context.Context, symbol string, stopLossManager *StopLossManager) string {
    var summary strings.Builder

    // 第一部分：账户信息 / Part 1: Account Information
    account, err := e.client.NewGetAccountService().Do(ctx)

    var usdtFree, usdtTotal float64
    for _, asset := range account.Assets {
        if asset.Asset == "USDT" {
            usdtFree, _ = parseFloat(asset.AvailableBalance)
            usdtTotal, _ = parseFloat(asset.WalletBalance)
            break
        }
    }

    // 计算保证金使用率 / Calculate margin usage
    usedMargin := usdtTotal - usdtFree
    usageRate := (usedMargin / usdtTotal) * 100

    // 确定风险等级 / Determine risk level
    riskLevel := ""
    if usageRate < 30 {
        riskLevel = "✅ 安全"
    } else if usageRate < 50 {
        riskLevel = "⚠️ 谨慎"
    } else if usageRate < 70 {
        riskLevel = "🚨 警戒"
    } else {
        riskLevel = "❌ 危险"
    }

    summary.WriteString("**账户信息**:\n")
    summary.WriteString(fmt.Sprintf("- 总余额: %.2f USDT\n", usdtTotal))
    summary.WriteString(fmt.Sprintf("- 可用余额: %.2f USDT\n", usdtFree))
    summary.WriteString(fmt.Sprintf("- 已用保证金: %.2f USDT\n", usedMargin))
    summary.WriteString(fmt.Sprintf("- 资金使用率: %.1f%% %s\n", usageRate, riskLevel))

    // 第二部分：持仓信息 / Part 2: Position Information
    position, _ := e.GetCurrentPosition(ctx, symbol)
    if position != nil && position.Side != "" {
        sideCN := "多头"
        if position.Side == "short" {
            sideCN = "空头"
        }

        // 获取当前价格 / Get current price
        ticker, _ := e.client.NewListPriceChangeStatsService().Symbol(e.config.GetBinanceSymbolFor(symbol)).Do(ctx)
        currentPrice := position.EntryPrice
        if len(ticker) > 0 {
            currentPrice, _ = parseFloat(ticker[0].LastPrice)
        }

        // 计算 ROE / Calculate ROE
        pnlPct := 0.0
        if position.EntryPrice > 0 && position.Size > 0 && position.Leverage > 0 {
            initialMargin := (position.EntryPrice * position.Size) / float64(position.Leverage)
            if initialMargin > 0 {
                pnlPct = (position.UnrealizedPnL / initialMargin) * 100
            }
        }

        summary.WriteString(fmt.Sprintf("**当前持仓 %s**:\n", symbol))
        summary.WriteString(fmt.Sprintf("- 方向: %s (%s)\n", sideCN, strings.ToUpper(position.Side)))
        summary.WriteString(fmt.Sprintf("- 数量: %.4f\n", position.Size))
        summary.WriteString(fmt.Sprintf("- 开仓价格: $%.2f\n", position.EntryPrice))
        summary.WriteString(fmt.Sprintf("- 杠杆倍数: %dx\n", position.Leverage))
        summary.WriteString(fmt.Sprintf("- 当前价格: $%.2f\n", currentPrice))
        summary.WriteString(fmt.Sprintf("- 未实现盈亏: %+.2f USDT (%+.2f%%)\n", position.UnrealizedPnL, pnlPct))

        // 第三部分：止损信息 / Part 3: Stop-loss Information
        if stopLossManager != nil {
            managedPos := stopLossManager.GetPosition(symbol)
            if managedPos != nil && managedPos.CurrentStopLoss > 0 {
                summary.WriteString(fmt.Sprintf("- 当前止损: $%.2f", managedPos.CurrentStopLoss))

                // 计算止损距离 / Calculate stop-loss distance
                stopDistance := 0.0
                if position.Side == "long" {
                    stopDistance = ((currentPrice - managedPos.CurrentStopLoss) / currentPrice) * 100
                } else {
                    stopDistance = ((managedPos.CurrentStopLoss - currentPrice) / currentPrice) * 100
                }
                summary.WriteString(fmt.Sprintf(" (距离当前价 %.2f%%)\n", stopDistance))
            }
        }

        if position.LiquidationPrice > 0 {
            summary.WriteString(fmt.Sprintf("- 爆仓价格: $%.2f\n", position.LiquidationPrice))
        }

    } else {
        summary.WriteString(fmt.Sprintf("**当前持仓 %s**: 无持仓\n", symbol))
    }

    return summary.String()
}
```

### 传递给 LLM 的实际格式示例

#### 无持仓情况：
```
**账户信息**:
- 总余额: 1000.00 USDT
- 可用余额: 1000.00 USDT
- 已用保证金: 0.00 USDT
- 资金使用率: 0.0% ✅ 安全

**当前持仓 BTC/USDT**: 无持仓
```

#### 有多仓情况：
```
**账户信息**:
- 总余额: 1000.00 USDT
- 可用余额: 700.00 USDT
- 已用保证金: 300.00 USDT
- 资金使用率: 30.0% ✅ 安全

**当前持仓 BTC/USDT**:
- 方向: 多头 (LONG)
- 数量: 0.1000
- 开仓价格: $50000.00
- 杠杆倍数: 10x
- 当前价格: $51000.00
- 未实现盈亏: +100.00 USDT (+33.33%)
- 当前止损: $48000.00 (距离当前价 5.88%)
- 爆仓价格: $45000.00
```

#### 有空仓情况：
```
**账户信息**:
- 总余额: 1000.00 USDT
- 可用余额: 700.00 USDT
- 已用保证金: 300.00 USDT
- 资金使用率: 30.0% ✅ 安全

**当前持仓 ETH/USDT**:
- 方向: 空头 (SHORT)
- 数量: 1.5000
- 开仓价格: $3000.00
- 杠杆倍数: 5x
- 当前价格: $2950.00
- 未实现盈亏: +75.00 USDT (+5.00%)
- 当前止损: $3100.00 (距离当前价 5.08%)
- 爆仓价格: $3750.00
```

---

## 5. 数据库存储

### 存储位置
**文件**: `/internal/storage/storage.go`

### TradingSession 结构
```go
type TradingSession struct {
    ID              int64
    BatchID         string
    Symbol          string
    Timeframe       string
    CreatedAt       time.Time
    MarketReport    string
    CryptoReport    string
    SentimentReport string
    PositionInfo    string  // ← 存储的是格式化后的字符串（上面示例的格式）
    Decision        string  // 该交易对的专属决策
    FullDecision    string  // LLM 原始完整决策（包含所有交易对）
    Executed        bool
    ExecutionResult string
}
```

### 数据库 Schema
```sql
CREATE TABLE trading_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id TEXT,
    symbol TEXT NOT NULL,
    timeframe TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    market_report TEXT,
    crypto_report TEXT,
    sentiment_report TEXT,
    position_info TEXT,          -- 存储的是格式化字符串
    decision TEXT,
    full_decision TEXT,
    leverage INTEGER,
    executed BOOLEAN DEFAULT 0,
    execution_result TEXT
);
```

### 保存流程
**文件**: `/internal/storage/storage.go`, 第 209-245 行

```go
func (s *Storage) SaveSession(session *TradingSession) (int64, error) {
    query := `
    INSERT INTO trading_sessions (
        batch_id, symbol, timeframe, created_at,
        market_report, crypto_report, sentiment_report,
        position_info, decision, full_decision, executed, execution_result
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

    result, err := s.db.Exec(
        query,
        session.BatchID,
        session.Symbol,
        session.Timeframe,
        session.CreatedAt,
        session.MarketReport,
        session.CryptoReport,
        session.SentimentReport,
        session.PositionInfo,  // 格式化的字符串
        session.Decision,
        session.FullDecision,
        session.Executed,
        session.ExecutionResult,
    )

    // ...
}
```

---

## 6. StopLossManager 中的持仓信息

### 位置
**文件**: `/internal/executors/stoploss_manager.go`

### 持仓注册
```go
func (sm *StopLossManager) RegisterPosition(pos *Position) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    pos.HighestPrice = pos.EntryPrice  // 初始化最高价
    pos.CurrentPrice = pos.EntryPrice   // 初始化当前价
    pos.StopLossType = "fixed"          // 固定止损

    sm.positions[pos.Symbol] = pos
    sm.logger.Success(fmt.Sprintf("【%s】持仓已注册，入场价: %.2f, 初始止损: %.2f",
        pos.Symbol, pos.EntryPrice, pos.InitialStopLoss))
}
```

### StopLossManager 的职责
1. 管理止损单（下单、取消、更新）
2. 追踪最高价/最低价（在本地内存中）
3. 记录止损变更历史
4. 保持与数据库的同步

---

## 7. 信息流总结

```
┌─────────────────────────────────────┐
│  币安 Futures API - GetPositionRisk │
│  (PositionAmt, EntryPrice,          │
│   UnRealizedProfit, Leverage, etc)  │
└────────────────┬────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────┐
│  GetCurrentPosition()                │
│  (提取必要字段，转换为 Position)     │
│  - PositionAmt → Side + Size        │
│  - EntryPrice                       │
│  - UnrealizedPnL                    │
│  - Leverage                         │
│  - LiquidationPrice                 │
└────────────────┬─────────────────────┘
                 │
        ┌────────┴──────────┐
        │                   │
        ▼                   ▼
┌─────────────┐    ┌──────────────────┐
│ Position    │    │ StopLossManager  │
│ 对象        │    │ 提供:            │
│             │    │ - CurrentStopLoss│
└────────────┘    │ - HighestPrice   │
        │         │ - PriceHistory   │
        │         └──────┬───────────┘
        └────────────────┤
                         │
                         ▼
┌──────────────────────────────────────┐
│  GetPositionSummary()                │
│  格式化为人类可读的字符串             │
│  包括：                              │
│  - 账户信息（余额、使用率）          │
│  - 持仓信息（方向、数量、价格）      │
│  - 止损信息（当前止损、距离）        │
│  - 风险等级                         │
└────────────────┬─────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────┐
│  AgentState.SetPositionInfo()        │
│  保存到 SymbolReports                │
└────────────────┬─────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────┐
│  GetAllReports()                     │
│  组合所有报告（市场、加密、情绪、    │
│  持仓）成为完整的提示                 │
└────────────────┬─────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────┐
│  makeLLMDecision()                   │
│  发送给 LLM 的用户提示                │
│  LLM 基于此作出交易决策               │
└──────────────────────────────────────┘
```

---

## 8. 关键发现与限制

### 币安 API 限制
1. **不提供历史最高/最低价**
   - 币安期货 API 只提供当前持仓的基本数据
   - 最高价需要从本地 K 线数据或实时价格监控中获取

2. **不提供持仓时长**
   - EntryTime 需要在本地记录
   - StopLossManager 中有 EntryTime 字段用于追踪

3. **不提供已实现盈亏汇总**
   - 只返回未实现盈亏 (UnRealizedProfit)
   - 已实现盈亏 (RealisedProfit) 仅在持仓打开时可用

### 系统补偿机制
1. **StopLossManager**
   - 在内存中维护 Position 对象
   - 追踪 HighestPrice、PriceHistory
   - 记录 StopLossHistory

2. **数据库持久化**
   - PositionRecord 存储完整的持仓历史
   - 包括 highest_price, current_price 等计算字段

3. **多数据源融合**
   - 币安 API：基本持仓数据
   - 市场数据模块：OHLCV 数据和实时价格
   - StopLossManager：止损跟踪
   - 数据库：历史记录

---

## 9. 提供给 LLM 的完整上下文

在 `makeLLMDecision()` 中，LLM 接收：

```
系统提示 (System Prompt):
  └─ 交易哲学、决策原则、输出格式要求

用户提示 (User Prompt):
  ├─ 动态杠杆范围信息（如果启用）
  ├─ K 线数据间隔信息
  └─ AllReports（包含）:
     ├─ 市场分析报告（技术指标）
     ├─ 加密货币分析报告（资金费率、订单簿、OI、24h统计）
     ├─ 市场情绪分析报告
     └─ 持仓信息（GetPositionSummary 的输出）
```

所有这些信息的组合使 LLM 能够做出有根据的交易决策。

---

## 10. 查询和审计

### 查询最新持仓信息
**方法**: `/internal/storage/storage.go` - `GetLatestSessions(limit int)`

```go
sessions, err := storage.GetLatestSessions(10)
for _, session := range sessions {
    fmt.Println(session.PositionInfo)
}
```

### 从命令行查询
```bash
# 查看最新 10 个会话
make query ARGS="latest 10"

# 查看特定交易对的 5 个会话
make query ARGS="symbol BTC/USDT 5"

# 显示统计信息
make query ARGS="stats"
```

---

## 附录：数据类型转换

### 字符串到浮点数转换
**文件**: `/internal/executors/binance_executor.go`, 第 847-858 行

```go
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
```

币安 API 返回的所有数字字段都是字符串，需要进行转换。

---

## 总结

**持仓信息获取流程**：
1. 从币安 API 获取原始持仓数据（仅包含基本字段）
2. 从 StopLossManager 获取止损信息
3. 从实时行情获取当前价格
4. 从账户服务获取余额和保证金使用情况
5. 格式化为人类可读的摘要字符串
6. 保存到数据库为字符串
7. 发送给 LLM 作为决策上下文

**关键限制**：
- 币安 API 不提供历史最高/最低价格
- 币安 API 不提供持仓时长
- 这些信息由系统内部维护（StopLossManager、数据库）

**数据流的完整性**：虽然币安 API 限制了可用字段，但系统通过多个数据源的组合（API、数据库、实时计算）提供了 LLM 做出决策所需的完整信息。
