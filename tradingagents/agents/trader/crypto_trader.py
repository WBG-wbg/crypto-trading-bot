"""
加密货币交易员 - 基于多智能体分析生成交易决策
"""
from langchain_core.prompts import ChatPromptTemplate, MessagesPlaceholder


def create_crypto_trader(llm, memory, config=None):
    """
    创建加密货币交易员节点
    
    交易员的职责：
    1. 综合所有分析师的报告
    2. 生成具体的交易决策（BUY/SELL/HOLD）
    3. 确定交易数量、止损、止盈价位
    4. 考虑杠杆倍数和风险控制
    """
    
    def crypto_trader_node(state):
        company_of_interest = state["company_of_interest"]
        trade_date = state["trade_date"]
        
        # 获取各个分析师的报告
        market_report = state.get("market_report", "")
        crypto_analysis_report = state.get("crypto_analysis_report", "")
        sentiment_report = state.get("sentiment_report", "")
        news_report = state.get("news_report", "")
        
        # 获取研究团队的辩论结果
        judge_decision = state.get("investment_debate_state", {}).get("judge_decision", "")
        
        # 从记忆中获取过往经验（如果启用）
        use_memory = config.get("use_memory", False) if config else False
        if use_memory:
            curr_situation = f"{market_report}\n{crypto_analysis_report}"
            past_memories = memory.get_memories(curr_situation, n_matches=3)
            memory_context = "\n".join([
                f"- {mem['recommendation']}"
                for mem in past_memories
            ]) if past_memories else "No relevant past trading experiences found."
        else:
            memory_context = "Memory function is disabled."
        
        system_message = f"""你是一位急需赚钱的经验丰富加密货币交易员，负责将多个分析师的报告整合成具体的交易决策。

你需要考虑以下方面：

**1. 市场技术分析** (来自市场分析师):
{market_report if market_report else "未提供市场技术分析"}

**2. 加密货币特有分析** (来自加密货币分析师):
{crypto_analysis_report if crypto_analysis_report else "未提供加密货币专属分析"}

**3. 社交媒体情绪** (来自社交媒体分析师):
{sentiment_report if sentiment_report else "未提供社交媒体情绪分析"}

**4. 新闻和宏观面** (来自新闻分析师):
{news_report if news_report else "未提供新闻分析"}

**5. 研究团队辩论结论**:
{judge_decision if judge_decision else "研究团队尚未给出结论"}

**6. 过往交易经验**:
{memory_context}

---

**交易决策要求**：

作为一位急需赚钱的加密货币交易员，你需要生成一份详细的交易计划，包括：

1. **交易方向**: BUY (做多) / SELL (做空) / HOLD (观望)
   - 综合所有分析维度，给出明确的交易方向
   - 如果信号矛盾或不确定性高，应选择 HOLD

2. **进场理由**:
   - 列出支持该决策的3-5个关键因素
   - 说明各个维度的信号是否一致

3. **仓位大小建议**:
   - 轻仓（10-30%）：信号弱或不确定性高
   - 中仓（30-60%）：信号较强且一致
   - 重仓（60-90%）：信号极强且多维度确认
   - 建议使用的杠杆倍数（1x-10x）

4. **风险管理**:
   - **止损价位**: 基于技术分析（支撑位/阻力位）和资金费率
   - **止盈价位**: 设置合理的盈利目标
   - **风险收益比**: 至少应该达到 1:2

5. **关键监控指标**:
   - 需要持续关注的价格水平
   - 可能改变决策的市场信号

6. **退出策略**:
   - 什么情况下应该提前离场
   - 什么情况下可以加仓

**特别注意**：
- 加密货币市场波动性极高，风险控制优先
- 资金费率异常可能是反转信号
- 订单簿大单可能是主力操纵
- 社交媒体情绪极端时要警惕反向操作

请生成一份结构清晰、逻辑严谨的交易计划，帮助风险管理团队评估。

报告末尾必须包含一个 Markdown 表格，总结关键交易参数。
"""
        
        prompt = ChatPromptTemplate.from_messages([
            (
                "system",
                "你是一位急需赚钱的加密货币交易员，并且加密货币只是你整体理财的一小部分，负责制定交易计划。"
                "综合所有分析师的报告和研究团队的建议，给出具体的交易决策。"
                "当前日期: {trade_date}, 交易对: {ticker}"
            ),
            MessagesPlaceholder(variable_name="messages"),
            ("human", system_message)
        ])
        
        prompt = prompt.partial(trade_date=trade_date)
        prompt = prompt.partial(ticker=company_of_interest)
        
        chain = prompt | llm
        result = chain.invoke(state["messages"])
        
        # 存储交易计划到记忆
        memory.add_memory(
            company_of_interest,
            f"Trading plan generated on {trade_date}: {result.content[:500]}...",
            metadata={"date": trade_date, "type": "trading_plan"}
        )
        
        return {
            "messages": [result],
            "trader_investment_plan": result.content
        }
    
    return crypto_trader_node

