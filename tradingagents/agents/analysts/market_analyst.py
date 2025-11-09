from langchain_core.prompts import ChatPromptTemplate, MessagesPlaceholder
import time
import json
from tradingagents.agents.utils.agent_utils import get_crypto_data, get_crypto_indicators
from tradingagents.dataflows.config import get_config
from tradingagents.utils.llm_utils import llm_retry


def create_market_analyst(llm):

    def market_analyst_node(state):
        current_date = state["trade_date"]
        ticker = state["company_of_interest"]
        company_name = state["company_of_interest"]

        tools = [
            get_crypto_data,
            get_crypto_indicators,
        ]

        system_message = (
            """You are a trading assistant tasked with analyzing cryptocurrency markets. Your role is to select the **most relevant indicators** (up to 5) for a given market condition or trading strategy from the following list. Choose indicators that provide complementary insights without redundancy. Categories and each category's indicators are:

Moving Averages:
- close_50_sma: 50 SMA: A medium-term trend indicator. Usage: Identify trend direction and serve as dynamic support/resistance. Tips: It lags price; combine with faster indicators for timely signals.
- close_200_sma: 200 SMA: A long-term trend benchmark. Usage: Confirm overall market trend and identify golden/death cross setups. Tips: It reacts slowly; best for strategic trend confirmation rather than frequent trading entries.
- close_10_ema: 10 EMA: A responsive short-term average. Usage: Capture quick shifts in momentum and potential entry points. Tips: Prone to noise in choppy markets; use alongside longer averages for filtering false signals.

MACD Related:
- macd: MACD: Computes momentum via differences of EMAs. Usage: Look for crossovers and divergence as signals of trend changes. Tips: Confirm with other indicators in low-volatility or sideways markets.
- macds: MACD Signal: An EMA smoothing of the MACD line. Usage: Use crossovers with the MACD line to trigger trades. Tips: Should be part of a broader strategy to avoid false positives.
- macdh: MACD Histogram: Shows the gap between the MACD line and its signal. Usage: Visualize momentum strength and spot divergence early. Tips: Can be volatile; complement with additional filters in fast-moving markets.

Momentum Indicators:
- rsi: RSI: Measures momentum to flag overbought/oversold conditions. Usage: Apply 70/30 thresholds and watch for divergence to signal reversals. Tips: In strong trends, RSI may remain extreme; always cross-check with trend analysis.

Volatility Indicators:
- boll: Bollinger Middle: A 20 SMA serving as the basis for Bollinger Bands. Usage: Acts as a dynamic benchmark for price movement. Tips: Combine with the upper and lower bands to effectively spot breakouts or reversals.
- boll_ub: Bollinger Upper Band: Typically 2 standard deviations above the middle line. Usage: Signals potential overbought conditions and breakout zones. Tips: Confirm signals with other tools; prices may ride the band in strong trends.
- boll_lb: Bollinger Lower Band: Typically 2 standard deviations below the middle line. Usage: Indicates potential oversold conditions. Tips: Use additional analysis to avoid false reversal signals.
- atr: ATR: Averages true range to measure volatility. Usage: Set stop-loss levels and adjust position sizes based on current market volatility. Tips: It's a reactive measure, so use it as part of a broader risk management strategy.

Volume-Based Indicators:
- vwma: VWMA: A moving average weighted by volume. Usage: Confirm trends by integrating price action with volume data. Tips: Watch for skewed results from volume spikes; use in combination with other volume analyses.

- Select up to 5 indicators that provide diverse and complementary information. Avoid redundancy (e.g., do not select both rsi and stochrsi). When you tool call, please use the exact name of the indicators provided above as they are defined parameters, otherwise your call will fail. 

For crypto markets, prioritize:
1. RSI for momentum
2. One moving average (SMA or EMA) for trend
3. MACD for momentum divergence
4. ATR for volatility

First call get_crypto_data to retrieve price data, then call get_crypto_indicators for each selected indicator (one at a time). Write a concise but insightful report of the trends you observe."""
            + """ Make sure to append a Markdown table at the end of the report to organize key points in the report, organized and easy to read."""
        )

        prompt = ChatPromptTemplate.from_messages(
            [
                (
                    "system",
                    "You are a helpful AI assistant, collaborating with other assistants."
                    " Use the provided tools to progress towards answering the question."
                    " If you are unable to fully answer, that's OK; another assistant with different tools"
                    " will help where you left off. Execute what you can to make progress."
                    " You only need to analyze the results objectively; you don't need to give advice on whether to buy or sell."
                    " You have access to the following tools: {tool_names}.\n{system_message}"
                    "For your reference, the current date is {current_date}. The company we want to look at is {ticker}"
                    "请使用中文回答",
                ),
                MessagesPlaceholder(variable_name="messages"),
            ]
        )

        prompt = prompt.partial(system_message=system_message)
        prompt = prompt.partial(tool_names=", ".join([tool.name for tool in tools]))
        prompt = prompt.partial(current_date=current_date)
        prompt = prompt.partial(ticker=ticker)

        chain = prompt | llm.bind_tools(tools)

        # 添加重试机制防止 404 等临时错误
        @llm_retry(max_retries=3, base_delay=10.0, backoff_factor=2.0)
        def invoke_with_retry():
            return chain.invoke(state["messages"])
        
        result = invoke_with_retry()

        # 保存报告逻辑：
        # 1. 如果没有工具调用，说明这是最终报告
        # 2. 如果有工具调用，保持原有的报告（不覆盖）
        report = state.get("market_report", "")  # 获取之前保存的报告
        
        if len(result.tool_calls) == 0:
            # 这是最终报告，保存它
            report = result.content
        # 否则，如果还有工具调用，保持原有的 report（不变）
       
        return {
            "messages": [result],
            "market_report": report,
        }

    return market_analyst_node
