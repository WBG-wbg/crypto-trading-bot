package dataflows

import (
	"context"
	"fmt"
	"time"
)

// ExampleGetSentimentIndicators demonstrates how to use GetSentimentIndicators
// ExampleGetSentimentIndicators 演示如何使用 GetSentimentIndicators
func ExampleGetSentimentIndicators() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Fetch sentiment data for BTC
	// 获取 BTC 的情绪数据
	sentiment := GetSentimentIndicators(ctx, "BTC")

	if sentiment.Success {
		fmt.Printf("Symbol: %s\n", sentiment.Symbol)
		fmt.Printf("Positive Ratio: %.2f%%\n", sentiment.PositiveRatio*100)
		fmt.Printf("Negative Ratio: %.2f%%\n", sentiment.NegativeRatio*100)
		fmt.Printf("Net Sentiment: %+.4f\n", sentiment.NetSentiment)
		fmt.Printf("Sentiment Level: %s\n", sentiment.SentimentLevel)
	} else {
		fmt.Printf("Failed to fetch sentiment: %s\n", sentiment.Error)
	}
}

// ExampleFormatSentimentReport demonstrates how to format sentiment report
// ExampleFormatSentimentReport 演示如何格式化情绪报告
func ExampleFormatSentimentReport() {
	sentiment := &SentimentData{
		Success:          true,
		PositiveRatio:    0.65,
		NegativeRatio:    0.35,
		NetSentiment:     0.30,
		SentimentLevel:   "偏向乐观 ✅",
		DataTime:         "2025-11-11 22:00:00",
		DataDelayMinutes: 45,
		Symbol:           "BTC",
	}

	report := FormatSentimentReport(sentiment)
	fmt.Println(report)
}

// ExampleFormatSentimentReport_error demonstrates error report formatting
// ExampleFormatSentimentReport_error 演示错误报告格式化
func ExampleFormatSentimentReport_error() {
	sentiment := &SentimentData{
		Success: false,
		Error:   "API request timeout",
		Symbol:  "ETH",
	}

	report := FormatSentimentReport(sentiment)
	fmt.Println(report)
}

// ExampleGetSentimentIndicators_interpretation demonstrates sentiment interpretation
// ExampleGetSentimentIndicators_interpretation 演示情绪解释
func ExampleGetSentimentIndicators_interpretation() {
	// This example shows how sentiment values map to sentiment levels
	// 此示例展示情绪值如何映射到情绪等级

	testCases := []struct {
		value float64
		level string
	}{
		{0.75, "极度乐观 🔥"},
		{0.5, "强烈乐观 📈"},
		{0.2, "轻度乐观 ↗️"},
		{0.0, "中性 ➖"},
		{-0.2, "轻度悲观 ↘️"},
		{-0.5, "偏向悲观 ❌"},
		{-0.75, "强烈悲观 📉"},
	}

	fmt.Println("情绪值到情绪等级的映射：")
	for _, tc := range testCases {
		fmt.Printf("%.2f → %s\n", tc.value, tc.level)
	}
	// Output example:
	// 情绪值到情绪等级的映射：
	// 0.75 → 极度乐观 🔥
	// 0.50 → 强烈乐观 📈
	// 0.20 → 轻度乐观 ↗️
	// 0.00 → 中性 ➖
	// -0.20 → 轻度悲观 ↘️
	// -0.50 → 偏向悲观 ❌
	// -0.75 → 强烈悲观 📉
}
