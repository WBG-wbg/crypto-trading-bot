# SQLite 存储系统使用说明

## 概述

Go 版本的加密货币交易机器人使用 SQLite 数据库来存储所有交易分析会话。每次运行工作流时，系统会自动将分析结果保存到数据库中。

## 数据库位置

默认数据库路径：`./data/trading.db`

可以通过 `.env` 文件配置：
```bash
DATABASE_PATH=./data/trading.db
```

## 存储内容

每个交易会话包含以下信息：
- **基本信息**：交易对、时间周期、创建时间
- **分析报告**：
  - 市场技术分析报告
  - 加密货币专属分析报告
  - 市场情绪分析报告
  - 持仓信息
- **交易决策**：最终决策建议
- **执行状态**：是否执行、执行结果

## 查询工具

项目提供了 `query` 命令行工具来查询历史记录。

### 编译查询工具

```bash
# 编译所有工具（包括主程序和查询工具）
make build-all

# 或单独编译查询工具
go build -o bin/query cmd/query/main.go
```

### 使用方法

#### 1. 查看统计信息

```bash
./bin/query stats
```

输出示例：
```
=== Trading Sessions Statistics ===
Symbol:           BTC/USDT
Total Sessions:   25
Executed Trades:  5
Execution Rate:   20.0%
First Session:    2025-11-09 10:00:00
Last Session:     2025-11-09 18:30:00
```

#### 2. 查看最近的会话

```bash
# 查看最近 10 个会话（默认）
./bin/query latest

# 查看最近 5 个会话
./bin/query latest 5
```

输出示例：
```
=== Latest 5 Trading Sessions ===

[1] Session ID: 25
    Symbol:      BTC/USDT
    Timeframe:   1h
    Created:     2025-11-09 18:30:00
    Executed:    false
    Decision:    === 交易决策分析 ===

    技术面分析:
    - RSI(14): 62.45 (中性区域)
    - MACD: 125.34, Signal: 118...

[2] Session ID: 24
    ...
```

#### 3. 按交易对查询

```bash
# 查看 BTC/USDT 最近 10 个会话
./bin/query symbol BTC/USDT

# 查看 ETH/USDT 最近 20 个会话
./bin/query symbol ETH/USDT 20
```

### 使用 Makefile 快捷命令

```bash
# 查看统计信息
make query ARGS="stats"

# 查看最近 5 个会话
make query ARGS="latest 5"

# 按交易对查询
make query ARGS="symbol BTC/USDT 10"
```

## 程序化访问

在 Go 代码中使用存储系统：

```go
import "github.com/oak/crypto-trading-bot/internal/storage"

// 打开数据库
db, err := storage.NewStorage("./data/trading.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// 保存会话
session := &storage.TradingSession{
    Symbol:          "BTC/USDT",
    Timeframe:       "1h",
    CreatedAt:       time.Now(),
    MarketReport:    "...",
    CryptoReport:    "...",
    SentimentReport: "...",
    PositionInfo:    "...",
    Decision:        "...",
    Executed:        false,
}

sessionID, err := db.SaveSession(session)
if err != nil {
    log.Fatal(err)
}

// 查询最近的会话
sessions, err := db.GetLatestSessions(10)
if err != nil {
    log.Fatal(err)
}

// 获取统计信息
stats, err := db.GetSessionStats("BTC/USDT")
if err != nil {
    log.Fatal(err)
}
```

## 数据库 Schema

```sql
CREATE TABLE trading_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    timeframe TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    market_report TEXT,
    crypto_report TEXT,
    sentiment_report TEXT,
    position_info TEXT,
    decision TEXT,
    executed BOOLEAN DEFAULT 0,
    execution_result TEXT
);

-- 索引优化
CREATE INDEX idx_symbol_created_at ON trading_sessions(symbol, created_at DESC);
CREATE INDEX idx_created_at ON trading_sessions(created_at DESC);
```

## 维护

### 备份数据库

```bash
# 简单备份
cp data/trading.db data/trading_backup_$(date +%Y%m%d).db

# 使用 sqlite3 命令
sqlite3 data/trading.db ".backup data/trading_backup.db"
```

### 清理旧数据

```sql
-- 删除 30 天前的记录
DELETE FROM trading_sessions
WHERE created_at < datetime('now', '-30 days');

-- 优化数据库
VACUUM;
```

### 查看数据库大小

```bash
ls -lh data/trading.db
```

## 性能特点

- **纯 Go 实现**：使用 `modernc.org/sqlite`，无需 CGO，跨平台兼容
- **自动索引**：按时间和交易对优化查询
- **并发安全**：支持多个进程同时读取
- **轻量级**：单个会话约 5-10 KB，可存储数万条记录
- **快速查询**：索引优化后，查询速度 < 1ms

## 注意事项

1. **数据库文件**：已在 `.gitignore` 中配置，不会提交到版本控制
2. **定期备份**：建议每周备份一次数据库
3. **存储空间**：1000 个会话约占用 5-10 MB
4. **并发写入**：主程序每次执行只写入一次，不存在并发写入问题

## 未来增强

可能的功能扩展：
- 交易执行结果自动更新
- 策略回测分析
- 性能指标统计
- Web 界面查询和可视化
- 导出为 CSV/JSON 格式
