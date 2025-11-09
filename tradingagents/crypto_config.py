"""
加密货币交易配置 - 所有配置从 .env 文件读取
统一配置管理，避免配置分散
"""
import os


def get_crypto_config():
    """
    从环境变量读取加密货币交易配置
    所有配置统一在 .env 文件中设置
    """
    
    # 项目目录（不需要配置）
    project_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "."))
    
    config = {
        # ==================== 项目路径 ====================
        "project_dir": project_dir,
        "results_dir": os.getenv("RESULTS_DIR", "./crypto_results"),
        "data_cache_dir": os.path.join(project_dir, "dataflows/data_cache"),
        
        # ==================== LLM 配置 ====================
        "llm_provider": os.getenv("LLM_PROVIDER", "openai"),
        "deep_think_llm": os.getenv("DEEP_THINK_LLM", "gpt-4o"),
        "quick_think_llm": os.getenv("QUICK_THINK_LLM", "gpt-4o-mini"),
        "backend_url": os.getenv("LLM_BACKEND_URL", "https://api.openai.com/v1"),
        
        # ==================== 智能体行为配置 ====================
        "max_debate_rounds": int(os.getenv("MAX_DEBATE_ROUNDS", "2")),
        "max_risk_discuss_rounds": int(os.getenv("MAX_RISK_DISCUSS_ROUNDS", "2")),
        "max_recur_limit": int(os.getenv("MAX_RECUR_LIMIT", "100")),
        
        # ==================== 数据源配置 ====================
        "data_vendors": {
            "core_stock_apis": os.getenv("DATA_VENDOR_STOCK", "ccxt"),
            "technical_indicators": os.getenv("DATA_VENDOR_INDICATORS", "ccxt"),
            "news_data": os.getenv("DATA_VENDOR_NEWS", "alpha_vantage"),
            "crypto_data": os.getenv("DATA_VENDOR_CRYPTO", "ccxt"),
        },
        "tool_vendors": {},
        
        # ==================== 币安交易配置 ====================
        "binance_proxy": os.getenv("BINANCE_PROXY", None),
        "binance_leverage": int(os.getenv("BINANCE_LEVERAGE", "10")),
        "binance_test_mode": os.getenv("BINANCE_TEST_MODE", "true").lower() == "true",
        "binance_position_mode": os.getenv("BINANCE_POSITION_MODE", "auto").lower(),  # 'auto', 'oneway', 'hedge'
        
        # ==================== 交易参数 ====================
        "crypto_symbol": os.getenv("CRYPTO_SYMBOL", "BTC/USDT"),
        "crypto_timeframe": os.getenv("CRYPTO_TIMEFRAME", "1h"),
        "crypto_lookback_days": int(os.getenv("CRYPTO_LOOKBACK_DAYS")) if os.getenv("CRYPTO_LOOKBACK_DAYS") else None,
        "position_size": float(os.getenv("POSITION_SIZE", "0.001")),
        "max_position_size": float(os.getenv("MAX_POSITION_SIZE", "0.01")),
        
        # ==================== 风险管理参数 ====================
        "max_drawdown": float(os.getenv("MAX_DRAWDOWN", "0.15")),
        "risk_per_trade": float(os.getenv("RISK_PER_TRADE", "0.02")),
        "volatility_multiplier": float(os.getenv("VOLATILITY_MULTIPLIER", "1.5")),
        
        # ==================== 记忆系统 ====================
        "use_memory": os.getenv("USE_MEMORY", "true").lower() == "true",
        "memory_top_k": int(os.getenv("MEMORY_TOP_K", "3")),
        
        # ==================== 调试选项 ====================
        "debug_mode": os.getenv("DEBUG_MODE", "false").lower() == "true",
        "selected_analysts": os.getenv("SELECTED_ANALYSTS", "market,crypto,social,news").split(","),
        "auto_execute": os.getenv("AUTO_EXECUTE", "false").lower() == "true",
    }
    
    return config

