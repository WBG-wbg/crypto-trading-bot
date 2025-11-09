"""
加密货币专属分析师 - 替代传统基本面分析师
分析链上数据、资金费率、订单簿等加密货币特有指标
"""
from langchain_core.prompts import ChatPromptTemplate, MessagesPlaceholder
from tradingagents.agents.utils.agent_utils import (
    get_crypto_funding_rate,
    get_crypto_order_book,
    get_crypto_market_info
)
from tradingagents.utils.llm_utils import llm_retry


def create_crypto_analyst(llm):
    """
    创建加密货币分析师节点
    
    该分析师专注于加密货币特有的数据：
    - 资金费率（反映市场情绪）
    - 订单簿深度（支撑/阻力位）
    - 24小时交易统计
    """
    
    def crypto_analyst_node(state):
        current_date = state["trade_date"]
        ticker = state["company_of_interest"]  # 实际上是 crypto symbol，如 BTC/USDT
        
        tools = [
            get_crypto_funding_rate,
            get_crypto_order_book,
            get_crypto_market_info,
        ]
        
        system_message = (
            """你是一位专业的加密货币市场分析师，精通链上数据和市场微观结构分析。你的任务是分析加密货币市场的独特指标，包括：

1. **资金费率分析** (Funding Rate):
   - 资金费率是永续合约特有的机制，用于维持合约价格与现货价格的锚定
   - 正费率：多头支付空头，表示市场看多情绪高涨（可能过热）
   - 负费率：空头支付多头，表示市场看空情绪浓厚（可能超卖）
   - 持续高资金费率可能预示市场即将回调
   - 资金费率的突变往往伴随着市场情绪的转变

2. **订单簿深度分析** (Order Book):
   - 买盘(Bids)和卖盘(Asks)的分布显示市场的支撑和阻力位
   - 大额买单聚集处往往形成强支撑
   - 大额卖单聚集处往往形成强阻力
   - 买卖比例失衡可能预示价格变动方向
   - 突然出现的大单可能是主力操作信号

3. **24小时市场统计**:
   - 成交量的变化反映市场活跃度和趋势强度
   - 价格波动范围显示市场波动性
   - 买卖价差反映流动性状况

4. **综合市场情绪判断**:
   - 结合资金费率、订单簿、成交量等多维度数据
   - 识别市场过热或超卖信号
   - 发现潜在的支撑位和阻力位
   - 评估当前价格的持续性

请使用提供的工具获取数据，并撰写一份详细的加密货币市场分析报告。报告应该：
- 详细解读各项指标的含义和市场含义
- 指出潜在的交易机会和风险点
- 提供明确的支撑位和阻力位参考

不要简单地说"趋势混合"，而要提供细致入微的分析和见解，帮助交易员做出决策。
报告末尾必须附上 Markdown 表格，组织关键要点，便于阅读。"""
        )
        
        prompt = ChatPromptTemplate.from_messages(
            [
                (
                    "system",
                    "你是一个专业的AI助手，与其他助手协作。"
                    "使用提供的工具来回答问题并取得进展。"
                    "如果你无法完全回答问题，没关系；另一个具有不同工具的助手会在你离开的地方继续。"
                    "尽你所能执行以取得进展。"
                    "你只需要客观地分析结果，并不需要给出买入或卖出的建议。"
                    "你可以使用以下工具: {tool_names}。\n{system_message}"
                    "当前日期是 {current_date}。我们要分析的加密货币是 {ticker}",
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
        report = state.get("crypto_analysis_report", "")  # 获取之前保存的报告
        
        if len(result.tool_calls) == 0:
            # 这是最终报告，保存它
            report = result.content
        # 否则，如果还有工具调用，保持原有的 report（不变）
        
        return {
            "messages": [result],
            "crypto_analysis_report": report,
        }
    
    return crypto_analyst_node

