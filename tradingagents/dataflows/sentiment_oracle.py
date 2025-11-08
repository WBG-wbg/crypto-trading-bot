"""
市场情绪数据获取模块
通过 CryptoOracle API 获取BTC市场情绪指标
"""
import requests
from datetime import datetime, timedelta
from typing import Dict, Optional, Any


def get_sentiment_indicators(symbol: str = "BTC") -> Dict[str, Any]:
    """
    获取加密货币市场情绪指标
    
    Args:
        symbol: 交易对符号（目前仅支持 BTC）
    
    Returns:
        情绪数据字典，包含正面/负面比率、净情绪值等
    """
    try:
        API_URL = "https://service.cryptoracle.network/openapi/v2/endpoint"
        API_KEY = "7ad48a56-8730-4238-a714-eebc30834e3e"
        
        # 获取最近4小时数据（考虑到30分钟延迟，确保能获取到有效数据）
        end_time = datetime.now() - timedelta(minutes=40)  # 提前40分钟，避免获取到空数据
        start_time = end_time - timedelta(hours=4)
        
        request_body = {
            "apiKey": API_KEY,
            "endpoints": ["CO-A-02-01", "CO-A-02-02"],  # 正面/负面情绪指标
            "startTime": start_time.strftime("%Y-%m-%d %H:%M:%S"),
            "endTime": end_time.strftime("%Y-%m-%d %H:%M:%S"),
            "timeType": "15m",
            "token": [symbol]
        }
        
        headers = {
            "Content-Type": "application/json",
            "X-API-KEY": API_KEY
        }
        
        response = requests.post(API_URL, json=request_body, headers=headers, timeout=10)
        
        if response.status_code == 200:
            data = response.json()
            
            if data.get("code") == 200 and data.get("data"):
                time_periods = data["data"][0]["timePeriods"]
                
                # 从最近的时间段开始查找第一个有有效数据的时间段
                for period in time_periods:
                    period_data = period.get("data", [])
                    sentiment = {}
                    valid_data_found = False
                    
                    for item in period_data:
                        endpoint = item.get("endpoint")
                        value = item.get("value", "").strip()
                        
                        if value:  # 只处理非空值
                            try:
                                if endpoint in ["CO-A-02-01", "CO-A-02-02"]:
                                    sentiment[endpoint] = float(value)
                                    valid_data_found = True
                            except (ValueError, TypeError):
                                continue
                    
                    # 如果找到有效数据
                    if valid_data_found and "CO-A-02-01" in sentiment and "CO-A-02-02" in sentiment:
                        positive = sentiment['CO-A-02-01']
                        negative = sentiment['CO-A-02-02']
                        net_sentiment = positive - negative
                        
                        # 计算数据延迟
                        data_time = datetime.strptime(period['startTime'], '%Y-%m-%d %H:%M:%S')
                        data_delay = int((datetime.now() - data_time).total_seconds() // 60)
                        
                        return {
                            'success': True,
                            'positive_ratio': positive,
                            'negative_ratio': negative,
                            'net_sentiment': net_sentiment,
                            'sentiment_level': _interpret_sentiment(net_sentiment),
                            'data_time': period['startTime'],
                            'data_delay_minutes': data_delay,
                            'symbol': symbol
                        }
                
                # 所有时间段数据都为空
                return {
                    'success': False,
                    'error': '所有时间段数据都为空（可能数据延迟超过预期）',
                    'symbol': symbol
                }
            else:
                return {
                    'success': False,
                    'error': f'API返回异常: code={data.get("code")}, msg={data.get("msg")}',
                    'symbol': symbol
                }
        else:
            return {
                'success': False,
                'error': f'HTTP请求失败: status_code={response.status_code}',
                'symbol': symbol
            }
            
    except requests.exceptions.Timeout:
        return {
            'success': False,
            'error': 'API请求超时',
            'symbol': symbol
        }
    except Exception as e:
        return {
            'success': False,
            'error': f'获取情绪指标失败: {str(e)}',
            'symbol': symbol
        }


def _interpret_sentiment(net_sentiment: float) -> str:
    """
    解释净情绪值
    
    Args:
        net_sentiment: 净情绪值（正面 - 负面）
    
    Returns:
        情绪等级描述
    """
    if net_sentiment >= 0.7:
        return "极度乐观 🔥"
    elif net_sentiment >= 0.5:
        return "强烈乐观 📈"
    elif net_sentiment >= 0.3:
        return "偏向乐观 ✅"
    elif net_sentiment >= 0.1:
        return "轻度乐观 ↗️"
    elif net_sentiment >= -0.1:
        return "中性 ➖"
    elif net_sentiment >= -0.3:
        return "轻度悲观 ↘️"
    elif net_sentiment >= -0.5:
        return "偏向悲观 ❌"
    elif net_sentiment >= -0.7:
        return "强烈悲观 📉"
    else:
        return "极度悲观 ❄️"


def format_sentiment_report(sentiment_data: Dict[str, Any]) -> str:
    """
    格式化情绪数据为可读报告
    
    Args:
        sentiment_data: 情绪数据字典
    
    Returns:
        格式化的情绪报告文本
    """
    if not sentiment_data.get('success'):
        return f"""
# 市场情绪数据获取失败

⚠️ 错误信息: {sentiment_data.get('error', '未知错误')}
⚠️ 交易对: {sentiment_data.get('symbol', 'N/A')}

说明: 本次分析无法获取市场情绪数据，建议谨慎交易。
"""
    
    positive = sentiment_data['positive_ratio']
    negative = sentiment_data['negative_ratio']
    net = sentiment_data['net_sentiment']
    level = sentiment_data['sentiment_level']
    data_time = sentiment_data['data_time']
    delay = sentiment_data['data_delay_minutes']
    
    # 生成情绪趋势描述
    if net >= 0.5:
        trend_desc = "市场情绪极度乐观，可能存在过度买入风险，需警惕回调。"
    elif net >= 0.3:
        trend_desc = "市场情绪偏向乐观，多头占据优势，适合顺势做多。"
    elif net >= 0.1:
        trend_desc = "市场情绪轻度乐观，多头略占优势，可考虑轻仓做多。"
    elif net >= -0.1:
        trend_desc = "市场情绪相对中性，多空分歧较大，建议观望或轻仓操作。"
    elif net >= -0.3:
        trend_desc = "市场情绪轻度悲观，空头略占优势，可考虑轻仓做空。"
    elif net >= -0.5:
        trend_desc = "市场情绪偏向悲观，空头占据优势，适合顺势做空。"
    else:
        trend_desc = "市场情绪极度悲观，可能存在恐慌性抛售，需警惕反弹或寻找抄底机会。"
    
    report = f"""
# 市场情绪分析报告（{sentiment_data['symbol']}）

## 情绪指标概览
- **数据时间**: {data_time}（延迟 {delay} 分钟）
- **正面情绪比率**: {positive:.2%}
- **负面情绪比率**: {negative:.2%}
- **净情绪值**: {net:+.4f}
- **情绪等级**: {level}

## 情绪解读
{trend_desc}

## 交易建议参考
- **净情绪 > 0.3**: 市场偏多，可考虑做多策略
- **净情绪 < -0.3**: 市场偏空，可考虑做空策略
- **|净情绪| < 0.3**: 市场中性，建议观望或轻仓操作
- **|净情绪| > 0.6**: 极端情绪，警惕反转风险

## 数据来源
- API: CryptoOracle Sentiment Indicators
- 指标: CO-A-02-01 (正面情绪), CO-A-02-02 (负面情绪)
- 时间粒度: 15分钟
"""
    
    return report


if __name__ == "__main__":
    # 测试代码
    print("正在获取BTC市场情绪数据...\n")
    sentiment = get_sentiment_indicators("BTC")
    
    if sentiment['success']:
        print("✅ 数据获取成功！")
        print(format_sentiment_report(sentiment))
    else:
        print("❌ 数据获取失败")
        print(format_sentiment_report(sentiment))

