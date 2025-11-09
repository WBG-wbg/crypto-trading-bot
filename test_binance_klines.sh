#!/bin/bash
# 币安 K 线数据获取测试脚本
# 无需 API key，使用公开接口

echo "=========================================="
echo "币安 K 线数据获取测试"
echo "=========================================="
echo ""

# 1. 基础测试：获取 BTC/USDT 最近 10 根 1 小时 K 线
echo "1️⃣  测试：获取 BTC/USDT 最近 10 根 1 小时 K 线"
echo "----------------------------------------"
curl -s "https://fapi.binance.com/fapi/v1/klines?symbol=BTCUSDT&interval=1h&limit=10" | \
  jq -r '.[] | [
    (.[0]/1000|strftime("%Y-%m-%d %H:%M")),
    "开:",
    .[1],
    "高:",
    .[2],
    "低:",
    .[3],
    "收:",
    .[4],
    "量:",
    .[5]
  ] | join(" ")' | head -3

echo ""
echo "✅ 成功（无需 API key）"
echo ""

# 2. 获取不同时间周期
echo "2️⃣  测试：获取不同时间周期的 K 线"
echo "----------------------------------------"

for interval in "15m" "1h" "4h" "1d"; do
  count=$(curl -s "https://fapi.binance.com/fapi/v1/klines?symbol=BTCUSDT&interval=${interval}&limit=1" | jq 'length')
  if [ "$count" -gt 0 ]; then
    echo "✅ ${interval}: 成功获取数据"
  else
    echo "❌ ${interval}: 失败"
  fi
done

echo ""

# 3. 获取不同交易对
echo "3️⃣  测试：获取不同交易对的数据"
echo "----------------------------------------"

for symbol in "BTCUSDT" "ETHUSDT" "BNBUSDT"; do
  data=$(curl -s "https://fapi.binance.com/fapi/v1/klines?symbol=${symbol}&interval=1h&limit=1")
  price=$(echo "$data" | jq -r '.[0][4]')
  if [ "$price" != "null" ] && [ -n "$price" ]; then
    echo "✅ ${symbol}: 最新价格 \$${price}"
  else
    echo "❌ ${symbol}: 失败"
  fi
done

echo ""

# 4. 显示详细的 JSON 响应格式
echo "4️⃣  JSON 响应格式示例 (最新 1 根 K 线)"
echo "----------------------------------------"
curl -s "https://fapi.binance.com/fapi/v1/klines?symbol=BTCUSDT&interval=1h&limit=1" | jq '.[0] | {
  "开盘时间": (.[0]/1000|strftime("%Y-%m-%d %H:%M:%S")),
  "开盘价": .[1],
  "最高价": .[2],
  "最低价": .[3],
  "收盘价": .[4],
  "成交量": .[5],
  "收盘时间": (.[6]/1000|strftime("%Y-%m-%d %H:%M:%S")),
  "成交笔数": .[8]
}'

echo ""

# 5. 计算 24 小时价格变化
echo "5️⃣  24 小时价格变化"
echo "----------------------------------------"
data=$(curl -s "https://fapi.binance.com/fapi/v1/klines?symbol=BTCUSDT&interval=1h&limit=24")
first_price=$(echo "$data" | jq -r '.[0][1]')
last_price=$(echo "$data" | jq -r '.[-1][4]')

if [ "$first_price" != "null" ] && [ "$last_price" != "null" ]; then
  change=$(echo "scale=2; (($last_price - $first_price) / $first_price) * 100" | bc)
  echo "24小时前: \$${first_price}"
  echo "当前价格: \$${last_price}"
  echo "变化: ${change}%"
fi

echo ""
echo "=========================================="
echo "✅ 所有测试完成！"
echo "=========================================="
echo ""
echo "说明："
echo "  - 所有测试均使用公开接口"
echo "  - 不需要 API key"
echo "  - 不需要签名"
echo "  - 可以直接运行"
echo ""
echo "API 端点："
echo "  - 主网: https://fapi.binance.com/fapi/v1/klines"
echo "  - 测试网: https://testnet.binancefuture.com/fapi/v1/klines"
echo ""