# 🤖 Crypto Trading Bot (Go Version)

[English](README.md) | **简体中文**

---

基于 AI 智能体的加密货币自动交易系统 - **Go 语言实现版本**

使用大语言模型（LLM）分析市场数据、生成交易信号并在币安期货上执行交易。采用 **Cloudwego Eino 框架**进行多智能体并行编排。

![Trading Bot Dashboard](assets/fig1.png)


> ⚠️ **重要提示**：此项目从 Python 完全重构为 Go 版本，性能更高、并发能力更强。

## ✨ 核心特性

### 🎯 智能交易
- **多智能体并行分析**：市场分析师、加密货币分析师、情绪分析师并行工作
- **LLM 驱动决策**：支持 OpenAI API，可配置不同模型
- **动态杠杆**：根据置信度智能调整杠杆倍数（如 `10-20x`）
- **外部 Prompt 管理**：无需重新编译即可调整交易策略

### 🛡️ 风险管理
- **阶段性止损策略**：固定止损 → 保本止损 → 追踪止损
- **实时持仓监控**：每 10 秒检查并自动调整止损位
- **分批止盈**：达到目标后部分获利了结，剩余仓位追踪止损
- **风险参数可配置**：止损触发点、追踪距离、收紧条件等

### 📊 多交易对支持
- **并行分析**：同时分析多个交易对（BTC/USDT、ETH/USDT 等）
- **智能选择**：LLM 综合评估后选择最优交易机会
- **独立持仓管理**：每个交易对独立止损和风险控制

### 🌐 Web 监控面板
- **实时余额曲线图**：每 30 秒自动更新，Y 轴自适应
- **持仓可视化**：实时显示所有活跃持仓和盈亏
- **交易历史**：查看所有分析会话和决策记录
- **下次交易倒计时**：精确到秒的实时倒计时

### 💾 数据持久化
- **SQLite 数据库**：存储交易会话、持仓历史、余额快照
- **查询工具**：命令行工具快速查询历史数据
- **余额历史追踪**：每 5 分钟自动保存余额快照

## 🏗️ 技术栈

- **语言**：Go 1.21+
- **工作流编排**：[Cloudwego Eino](https://github.com/cloudwego/eino)
- **Web 框架**：[Hertz](https://github.com/cloudwego/hertz)
- **交易所 API**：[go-binance](https://github.com/adshao/go-binance)
- **配置管理**：[Viper](https://github.com/spf13/viper)
- **日志**：[zerolog](https://github.com/rs/zerolog)
- **数据库**：SQLite3

## 🚀 快速开始

### 前置要求

- **Go 1.21 或更高版本**
- 币安期货账户（支持测试网）
- OpenAI API Key（可选，用于 LLM 决策）

### 安装

```bash
# 克隆项目
git clone https://github.com/Oakshen/crypto-trading-bot.git
cd crypto-trading-bot

# 安装依赖
make deps

# 编译所有组件
make build-all
```

### 配置

1. 复制配置文件模板：
```bash
cp .env.example .env
```

2. 编辑 `.env` 文件，配置必要参数：

```env
# 币安 API（测试网或实盘）
BINANCE_API_KEY=your_api_key
BINANCE_API_SECRET=your_api_secret
BINANCE_TEST_MODE=true  # ⚠️ 强烈建议先使用测试模式

# 交易对（支持单个或多个）
CRYPTO_SYMBOLS=BTC/USDT,ETH/USDT,SOL/USDT

# 时间周期
CRYPTO_TIMEFRAME=15m

# 杠杆（支持固定或动态）
#BINANCE_LEVERAGE=10      # 固定 10 倍
BINANCE_LEVERAGE=10-20  # 动态 10-20 倍(强烈建议)

# OpenAI API（可选）
OPENAI_API_KEY=your_openai_key

# 自动执行
AUTO_EXECUTE=true  # 设置为 true 启用自动交易(！！！先设置为 false，测试过再改为 true)
```

### 运行

```bash
# 单次执行模式（运行一次分析后退出）
make run

# Web 监控模式（持续运行 + Web 界面）
make run-web

# 查询历史数据
make query ARGS="stats"           # 查看统计信息
make query ARGS="latest 10"       # 最近 10 次会话
make query ARGS="symbol BTC/USDT 5"  # 特定交易对
```

Web 界面默认地址：`http://localhost:8080`（端口可在 `.env` 中配置）

## 📖 使用方法

### 1. 测试模式运行（推荐新手）

```bash
# .env 中设置
BINANCE_TEST_MODE=true
AUTO_EXECUTE=true

# 运行 Web 模式观察
make run-web
```

### 2. 自定义交易策略

编辑 `prompts/trader_system.txt` 修改交易策略，无需重新编译：

```bash
# 使用不同的 Prompt 文件
TRADER_PROMPT_PATH=prompts/trader_aggressive.txt
```

提供的策略模板：
- `trader_system.txt` - 趋势交易，极度选择性（推荐）
- `trader_aggressive.txt` - 短线交易，积极捕捉机会

### 3. 多交易对配置

```bash
# 同时监控多个交易对
CRYPTO_SYMBOLS=BTC/USDT,ETH/USDT,BNB/USDT

# 系统会并行分析，选择最优机会
```

### 4. 查看实时数据

```bash
# Web API 端点
curl http://localhost:8080/api/balance/current    # 实时余额
curl http://localhost:8080/api/balance/history    # 余额历史
curl http://localhost:8080/api/positions          # 当前持仓
```

## 📁 项目结构

```
crypto-trading-bot/
├── cmd/
│   ├── main.go           # 单次执行模式入口
│   ├── web/main.go       # Web 监控模式入口
│   └── query/main.go     # 数据查询工具
├── internal/
│   ├── agents/           # AI 智能体（Eino Graph 工作流）
│   ├── dataflows/        # 市场数据获取和指标计算
│   ├── executors/        # 交易执行和止损管理
│   ├── portfolio/        # 投资组合管理
│   ├── storage/          # SQLite 数据库
│   ├── scheduler/        # 时间调度器
│   ├── web/              # Web 服务器和模板
│   ├── config/           # 配置加载
│   └── logger/           # 日志系统
├── prompts/              # 外部 Prompt 文件
├── data/                 # SQLite 数据库文件
├── .env.example          # 配置文件模板
├── Makefile             # 构建脚本
└── README.md

```

## 🏗️ 架构说明

### 多智能体工作流（Eino Graph）

系统使用 Eino Graph 编排多个 AI 智能体并行工作：

```
START → [市场分析师, 情绪分析师]（并行）
           ↓
市场分析师 → 加密货币分析师 → 持仓信息
           ↓                    ↓
       情绪分析师 ──────→ 交易员（综合决策）
                              ↓
                            END
```

## ⚙️ 常用命令

```bash
# 开发
make build        # 编译主程序
make build-all    # 编译所有组件
make test         # 运行测试
make test-cover   # 测试覆盖率
make fmt          # 格式化代码
make clean        # 清理编译产物

# 运行
make run          # 单次执行
make run-web      # Web 监控模式

# 查询
make query ARGS="stats"              # 统计信息
make query ARGS="latest 5"           # 最近 5 次
make query ARGS="symbol BTC/USDT 3"  # 特定交易对
```

## ⚠️ 安全警告

**重要提示**：

1. **先使用测试模式**：`BINANCE_TEST_MODE=true` 充分测试后再考虑实盘
2. **小仓位开始**：从最小的 `POSITION_SIZE` 开始
3. **设置止损**：确保止损策略已正确配置
4. **监控运行**：定期查看 Web 界面和日志
5. **API 安全**：
   - 使用 IP 白名单限制 API 访问
   - 永远不要分享你的 API 密钥
   - 只授予必要的权限（期货交易，不要现货）

**风险声明**：加密货币交易存在高风险，可能导致资金损失。本软件仅供学习和研究使用，使用者需自行承担所有风险。

## 🐛 故障排除

### 常见问题

1. **余额曲线图不显示**
   - 确保程序已运行至少 5-10 分钟
   - 检查数据库：`sqlite3 data/trading.db "SELECT COUNT(*) FROM balance_history;"`

2. **市场情绪获取失败**
   - 检查网络连接
   - 查看日志中的 `⚠️ 市场情绪数据获取失败` 提示
   - 情绪数据失败不影响交易决策

3. **持仓显示异常**
   - 确认 `BINANCE_POSITION_MODE` 配置正确
   - 检查币安账户实际持仓模式

4. **编译错误**
   - 确保 Go 版本 >= 1.21
   - 运行 `make deps` 更新依赖
   - 清理后重新编译：`make clean && make build-all`

## 📚 更多文档

- [CLAUDE.md](CLAUDE.md) - 详细的项目指南和架构说明
- [prompts/README.md](prompts/README.md) - Prompt 管理和策略配置
- [.env.example](.env.example) - 完整的配置参数说明

## 🔄 从 Python 版本迁移

本项目是从 Python 完全重写为 Go 版本：

**主要变化**：
- LangGraph → Eino Graph（Cloudwego）
- CCXT → go-binance（官方 SDK）
- pandas → 原生 Go 切片操作
- Flask → Hertz（Cloudwego）

**优势**：
- 更高的性能和并发能力
- 更低的资源占用
- 更快的启动速度
- 更好的类型安全

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

[MIT License](LICENSE)

---

**⚡ Powered by Go + Cloudwego Eino + AI**

> 如有问题或建议，欢迎在 GitHub Issues 中反馈。