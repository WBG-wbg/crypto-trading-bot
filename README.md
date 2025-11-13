# 🤖 Crypto Trading Bot (Go Version)

**English** | [简体中文](README_CN.md)

---

An AI agent-based cryptocurrency automated trading system - **Go implementation**

Uses Large Language Models (LLM) to analyze market data, generate trading signals, and execute trades on Binance Futures. Built with **Cloudwego Eino Framework** for multi-agent parallel orchestration.

![Trading Bot Dashboard](assets/fig1.png)


> ⚠️ **Important Notice**: This project has been completely refactored from Python to Go for higher performance and better concurrency.

## ✨ Core Features

### 🎯 Intelligent Trading
- **Multi-Agent Parallel Analysis**: Market Analyst, Crypto Analyst, and Sentiment Analyst working in parallel
- **LLM-Driven Decisions**: Supports OpenAI API with configurable models
- **Dynamic Leverage**: Intelligently adjusts leverage (e.g., `10-20x`) based on confidence
- **External Prompt Management**: Adjust trading strategies without recompilation

### 🛡️ Risk Management
- **Phased Stop-Loss Strategy**: Fixed Stop → Breakeven Stop → Trailing Stop
- **Real-time Position Monitoring**: Check and adjust stop-loss every 10 seconds
- **Partial Take Profit**: Take partial profits at targets, trail remaining position
- **Configurable Risk Parameters**: Stop-loss triggers, trailing distance, tightening conditions

### 📊 Multi-Symbol Support
- **Parallel Analysis**: Analyze multiple pairs simultaneously (BTC/USDT, ETH/USDT, etc.)
- **Intelligent Selection**: LLM evaluates and selects optimal trading opportunities
- **Independent Position Management**: Each pair has independent stop-loss and risk control

### 🌐 Web Monitoring Dashboard
- **Real-time Balance Chart**: Auto-updates every 30 seconds with adaptive Y-axis
- **Position Visualization**: Display all active positions and P&L in real-time
- **Trade History**: View all analysis sessions and trading decisions
- **Next Trade Countdown**: Precise countdown timer to the next trade

### 💾 Data Persistence
- **SQLite Database**: Store trading sessions, position history, balance snapshots
- **Query Tool**: CLI tool for quick historical data queries
- **Balance History Tracking**: Auto-save balance snapshots every 5 minutes

## 🏗️ Tech Stack

- **Language**: Go 1.21+
- **Workflow Orchestration**: [Cloudwego Eino](https://github.com/cloudwego/eino)
- **Web Framework**: [Hertz](https://github.com/cloudwego/hertz)
- **Exchange API**: [go-binance](https://github.com/adshao/go-binance)
- **Configuration**: [Viper](https://github.com/spf13/viper)
- **Logging**: [zerolog](https://github.com/rs/zerolog)
- **Database**: SQLite3

## 🚀 Quick Start

### Prerequisites

- **Go 1.21 or higher**
- Binance Futures account (testnet supported)
- OpenAI API Key (optional, for LLM decisions)

### Installation

```bash
# Clone the repository
git clone https://github.com/Oakshen/crypto-trading-bot.git
cd crypto-trading-bot

# Install dependencies
make deps

# Build all components
make build-all
```

### Configuration

1. Copy the configuration template:
```bash
cp .env.example .env
```

2. Edit `.env` file with required parameters:

```env
# Binance API (testnet or production)
BINANCE_API_KEY=your_api_key
BINANCE_API_SECRET=your_api_secret
BINANCE_TEST_MODE=true  # ⚠️ Strongly recommended to use test mode first

# Trading pairs (single or multiple)
CRYPTO_SYMBOLS=BTC/USDT,ETH/USDT

# Timeframe
CRYPTO_TIMEFRAME=1h

# Leverage (fixed or dynamic)
BINANCE_LEVERAGE=10      # Fixed 10x
# BINANCE_LEVERAGE=10-20  # Dynamic 10-20x

# OpenAI API (optional)
OPENAI_API_KEY=your_openai_key

# Auto execution
AUTO_EXECUTE=false  # Set to true to enable auto trading
```

### Running

```bash
# Single execution mode (runs once and exits)
make run

# Web monitoring mode (continuous + web interface)
make run-web

# Query historical data
make query ARGS="stats"           # View statistics
make query ARGS="latest 10"       # Last 10 sessions
make query ARGS="symbol BTC/USDT 5"  # Specific symbol
```

Web interface default address: `http://localhost:8080` (configurable in `.env`)

## 📖 Usage

### 1. Test Mode (Recommended for Beginners)

```bash
# Set in .env
BINANCE_TEST_MODE=true
AUTO_EXECUTE=true

# Run web mode to observe
make run-web
```

### 2. Custom Trading Strategy

Edit `prompts/trader_system.txt` to modify trading strategy without recompilation:

```bash
# Use different prompt file
TRADER_PROMPT_PATH=prompts/trader_aggressive.txt
```

Available strategy templates:
- `trader_system.txt` - Trend trading, highly selective (recommended)
- `trader_aggressive.txt` - Scalping, actively captures opportunities

### 3. Multi-Symbol Configuration

```bash
# Monitor multiple pairs simultaneously
CRYPTO_SYMBOLS=BTC/USDT,ETH/USDT,BNB/USDT

# System analyzes in parallel and selects best opportunities
```

### 4. Real-time Data Access

```bash
# Web API endpoints
curl http://localhost:8080/api/balance/current    # Real-time balance
curl http://localhost:8080/api/balance/history    # Balance history
curl http://localhost:8080/api/positions          # Current positions
```

## 📁 Project Structure

```
crypto-trading-bot/
├── cmd/
│   ├── main.go           # Single execution mode entry
│   ├── web/main.go       # Web monitoring mode entry
│   └── query/main.go     # Data query tool
├── internal/
│   ├── agents/           # AI agents (Eino Graph workflow)
│   ├── dataflows/        # Market data and indicator calculation
│   ├── executors/        # Trade execution and stop-loss management
│   ├── portfolio/        # Portfolio management
│   ├── storage/          # SQLite database
│   ├── scheduler/        # Time scheduler
│   ├── web/              # Web server and templates
│   ├── config/           # Configuration loading
│   └── logger/           # Logging system
├── prompts/              # External prompt files
├── data/                 # SQLite database files
├── .env.example          # Configuration template
├── Makefile             # Build scripts
└── README.md
```

## 🏗️ Architecture

### Multi-Agent Workflow (Eino Graph)

The system uses Eino Graph to orchestrate multiple AI agents working in parallel:

```
START → [Market Analyst, Sentiment Analyst] (parallel)
           ↓
Market Analyst → Crypto Analyst → Position Info
           ↓                    ↓
    Sentiment Analyst ────→ Trader (Final Decision)
                              ↓
                            END
```

### Stop-Loss Management Phases

```
Open → Fixed Stop → Breakeven Stop → Trailing Stop → Close
      (Initial)   (Profit > 1R)     (Profit > 2R)
```

## ⚙️ Common Commands

```bash
# Development
make build        # Build main program
make build-all    # Build all components
make test         # Run tests
make test-cover   # Test coverage
make fmt          # Format code
make clean        # Clean build artifacts

# Running
make run          # Single execution
make run-web      # Web monitoring mode

# Query
make query ARGS="stats"              # Statistics
make query ARGS="latest 5"           # Last 5 sessions
make query ARGS="symbol BTC/USDT 3"  # Specific symbol
```

## ⚠️ Security Warnings

**Important Reminders**:

1. **Use Test Mode First**: `BINANCE_TEST_MODE=true` - Thoroughly test before live trading
2. **Start Small**: Begin with minimum `POSITION_SIZE`
3. **Set Stop-Loss**: Ensure stop-loss strategy is properly configured
4. **Monitor Operations**: Regularly check web interface and logs
5. **API Security**:
   - Use IP whitelist to restrict API access
   - Never share your API keys
   - Grant only necessary permissions (futures trading, not spot)

**Risk Disclaimer**: Cryptocurrency trading carries high risk and may result in capital loss. This software is for educational and research purposes only. Users assume all risks.

## 🐛 Troubleshooting

### Common Issues

1. **Balance Chart Not Showing**
   - Ensure the program has been running for at least 5-10 minutes
   - Check database: `sqlite3 data/trading.db "SELECT COUNT(*) FROM balance_history;"`

2. **Market Sentiment Fetch Failed**
   - Check network connection
   - Look for `⚠️ Market sentiment data fetch failed` in logs
   - Sentiment data failure doesn't affect trading decisions

3. **Position Display Issues**
   - Confirm `BINANCE_POSITION_MODE` is configured correctly
   - Check actual position mode in Binance account

4. **Compilation Errors**
   - Ensure Go version >= 1.21
   - Run `make deps` to update dependencies
   - Clean and rebuild: `make clean && make build-all`

## 📚 More Documentation

- [CLAUDE.md](CLAUDE.md) - Detailed project guide and architecture
- [prompts/README.md](prompts/README.md) - Prompt management and strategy configuration
- [.env.example](.env.example) - Complete configuration parameters

## 🔄 Migration from Python Version

This project was completely rewritten from Python to Go:

**Major Changes**:
- LangGraph → Eino Graph (Cloudwego)
- CCXT → go-binance (Official SDK)
- pandas → Native Go slice operations
- Flask → Hertz (Cloudwego)

**Advantages**:
- Higher performance and concurrency
- Lower resource consumption
- Faster startup time
- Better type safety

## 🤝 Contributing

Issues and Pull Requests are welcome!

## 📄 License

[MIT License](LICENSE)

---

**⚡ Powered by Go + Cloudwego Eino + AI**

> For questions or suggestions, please provide feedback in GitHub Issues.
