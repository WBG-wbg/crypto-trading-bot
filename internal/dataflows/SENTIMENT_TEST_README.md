# 市场情绪报告测试文档

## 概述

本测试套件用于测试 CryptoOracle 市场情绪数据的获取、解析和报告生成功能。

## 测试文件

- `sentiment_test.go` - 主测试文件
- `sentiment_example_test.go` - 使用示例

## 测试覆盖

### 1. 功能测试

#### `TestGetSentimentIndicators_Success`
测试 API 调用的基本流程（注意：会调用真实 API）

#### `TestGetSentimentIndicators_Timeout`
测试超时处理机制
```bash
go test -v -run TestGetSentimentIndicators_Timeout ./internal/dataflows
```

#### `TestGetSentimentIndicators_RealAPI`
测试真实 API 调用（默认跳过，使用 `-short` 标志时跳过）
```bash
# 运行真实 API 测试（需要网络连接和有效的 API 密钥）
go test -v -run TestGetSentimentIndicators_RealAPI ./internal/dataflows
```

### 2. 单元测试

#### `TestInterpretSentiment`
测试情绪值到情绪等级的转换逻辑
```bash
go test -v -run TestInterpretSentiment ./internal/dataflows
```

**测试用例**：
- `0.75` → 极度乐观 🔥
- `0.60` → 强烈乐观 📈
- `0.40` → 偏向乐观 ✅
- `0.20` → 轻度乐观 ↗️
- `0.00` → 中性 ➖
- `-0.20` → 轻度悲观 ↘️
- `-0.40` → 偏向悲观 ❌
- `-0.60` → 强烈悲观 📉
- `-0.80` → 极度悲观 ❄️

#### `TestFormatSentimentReport_Success`
测试成功情况的报告格式化
```bash
go test -v -run TestFormatSentimentReport_Success ./internal/dataflows
```

#### `TestFormatSentimentReport_Failure`
测试错误情况的报告格式化
```bash
go test -v -run TestFormatSentimentReport_Failure ./internal/dataflows
```

### 3. 边界测试

#### `TestSentimentData_EdgeCases`
测试边界情况和极端值
```bash
go test -v -run TestSentimentData_EdgeCases ./internal/dataflows
```

**测试用例**：
- 零值数据
- 极端正向情绪（1.0）
- 极端负向情绪（-1.0）

## 快速运行

### 运行所有测试（跳过实际 API 调用）
```bash
go test -v -short ./internal/dataflows
```

### 运行所有测试（包括实际 API 调用）
```bash
go test -v ./internal/dataflows
```

### 运行特定测试
```bash
# 只测试情绪解释
go test -v -run TestInterpretSentiment ./internal/dataflows

# 只测试报告格式化
go test -v -run TestFormatSentimentReport ./internal/dataflows

# 只测试超时处理
go test -v -run TestGetSentimentIndicators_Timeout ./internal/dataflows
```

### 运行测试并生成覆盖率报告
```bash
go test -v -short -coverprofile=coverage.out ./internal/dataflows
go tool cover -html=coverage.out -o coverage.html
```

## 性能基准测试

### 运行所有基准测试
```bash
go test -bench=. -benchmem ./internal/dataflows
```

### 运行特定基准测试
```bash
# 基准测试 API 调用
go test -bench=BenchmarkGetSentimentIndicators -benchmem ./internal/dataflows

# 基准测试情绪解释
go test -bench=BenchmarkInterpretSentiment -benchmem ./internal/dataflows

# 基准测试报告格式化
go test -bench=BenchmarkFormatSentimentReport -benchmem ./internal/dataflows
```

## 测试输出示例

### 成功的情绪报告测试
```
=== RUN   TestFormatSentimentReport_Success
    sentiment_test.go:229: ✅ Report formatted correctly
    sentiment_test.go:230: Report preview:
    sentiment_test.go:231:
        # 市场情绪分析报告（BTC）

        ## 情绪指标概览
        - **数据时间**: 2025-11-11 22:00:00（延迟 45 分钟）
        - **正面情绪比率**: 65.00%
        - **负面情绪比率**: 35.00%
        - **净情绪值**: +0.3000
        - **情绪等级**: 偏向乐观 ✅

        ## 情绪解读
        市场情绪偏向乐观，多头占据优势，适合顺势做多。
        ...
--- PASS: TestFormatSentimentReport_Success (0.00s)
```

### 错误报告测试
```
=== RUN   TestFormatSentimentReport_Failure
    sentiment_test.go:260: ✅ Error report formatted correctly
    sentiment_test.go:261: Report preview:
    sentiment_test.go:262:
        # 市场情绪数据获取失败

        ⚠️ 错误信息: API request failed: timeout
        ⚠️ 交易对: ETH

        说明: 本次分析无法获取市场情绪数据，建议谨慎交易。
--- PASS: TestFormatSentimentReport_Failure (0.00s)
```

## 常见问题

### Q: 为什么测试显示 "context deadline exceeded"？
**A**: 这是预期行为。`TestGetSentimentIndicators_Timeout` 测试故意使用极短的超时时间来验证超时处理机制。

### Q: 如何跳过需要网络的测试？
**A**: 使用 `-short` 标志：
```bash
go test -v -short ./internal/dataflows
```

### Q: 真实 API 测试失败了怎么办？
**A**: 可能的原因：
1. 网络连接问题
2. CryptoOracle API 服务暂时不可用
3. API 数据延迟超过预期
4. API 密钥失效

这些都是正常情况，不影响代码功能。可以稍后重试或使用 `-short` 跳过。

### Q: 如何测试特定的交易对？
**A**: 修改 `TestGetSentimentIndicators_RealAPI` 中的 `symbols` 数组：
```go
symbols := []string{"BTC", "ETH", "SOL"}  // 添加或修改交易对
```

## 集成到 CI/CD

### GitHub Actions 示例
```yaml
- name: Run sentiment tests
  run: |
    go test -v -short -coverprofile=coverage.out ./internal/dataflows
    go tool cover -func=coverage.out
```

### Makefile 集成
```makefile
test-sentiment:
	go test -v -short -run TestInterpretSentiment ./internal/dataflows
	go test -v -short -run TestFormatSentimentReport ./internal/dataflows
	go test -v -short -run TestSentimentData_EdgeCases ./internal/dataflows

test-sentiment-full:
	go test -v ./internal/dataflows -timeout 30s
```

## 测试数据

测试使用的模拟数据示例：
```go
sentiment := &SentimentData{
    Success:          true,
    PositiveRatio:    0.65,  // 65% 正面情绪
    NegativeRatio:    0.35,  // 35% 负面情绪
    NetSentiment:     0.30,  // 净情绪 = 0.65 - 0.35
    SentimentLevel:   "偏向乐观 ✅",
    DataTime:         "2025-11-11 22:00:00",
    DataDelayMinutes: 45,    // 数据延迟 45 分钟
    Symbol:           "BTC",
}
```

## 维护建议

1. **定期运行真实 API 测试**：确保 API 集成仍然有效
2. **监控 API 响应时间**：使用基准测试跟踪性能变化
3. **更新测试数据**：如果 API 响应格式变化，及时更新测试用例
4. **添加新测试**：发现 bug 时添加相应的回归测试

## 相关文件

- `internal/dataflows/sentiment.go` - 主实现文件
- `internal/dataflows/sentiment_test.go` - 测试文件
- `internal/dataflows/sentiment_example_test.go` - 使用示例
- `internal/agents/graph.go` - 情绪分析在交易工作流中的使用

## 报告问题

如果发现测试问题，请提供：
1. 运行的完整命令
2. 完整的错误输出
3. Go 版本：`go version`
4. 操作系统和架构
5. 网络环境（是否使用代理等）
