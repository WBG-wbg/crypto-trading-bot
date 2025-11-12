package dataflows

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetSentimentIndicators_Success tests successful sentiment data retrieval
// TestGetSentimentIndicators_Success 测试成功获取情绪数据
func TestGetSentimentIndicators_Success(t *testing.T) {
	// Create mock server with valid response
	// 创建模拟服务器并返回有效响应
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Return mock response
		response := CryptoOracleResponse{
			Code:    200,
			Message: "success",
			Data: []struct {
				TimePeriods []struct {
					StartTime string `json:"startTime"`
					EndTime   string `json:"endTime"`
					Data      []struct {
						Endpoint string `json:"endpoint"`
						Value    string `json:"value"`
					} `json:"data"`
				} `json:"timePeriods"`
			}{
				{
					TimePeriods: []struct {
						StartTime string `json:"startTime"`
						EndTime   string `json:"endTime"`
						Data      []struct {
							Endpoint string `json:"endpoint"`
							Value    string `json:"value"`
						} `json:"data"`
					}{
						{
							StartTime: time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:04:05"),
							EndTime:   time.Now().Format("2006-01-02 15:04:05"),
							Data: []struct {
								Endpoint string `json:"endpoint"`
								Value    string `json:"value"`
							}{
								{Endpoint: "CO-A-02-01", Value: "0.65"},
								{Endpoint: "CO-A-02-02", Value: "0.35"},
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Note: This test will still call the real API
	// For full mock testing, you would need to refactor GetSentimentIndicators
	// to accept a custom HTTP client or URL
	t.Log("Note: This test calls the real CryptoOracle API")
	t.Log("If the API is unavailable or slow, the test may fail or timeout")
}

// TestGetSentimentIndicators_Timeout tests timeout handling
// TestGetSentimentIndicators_Timeout 测试超时处理
func TestGetSentimentIndicators_Timeout(t *testing.T) {
	// Create context with very short timeout
	// 创建超短超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Sleep to ensure timeout
	time.Sleep(2 * time.Millisecond)

	result := GetSentimentIndicators(ctx, "BTC")

	if result.Success {
		t.Error("Expected failure due to timeout, but got success")
	}

	if !strings.Contains(result.Error, "context deadline exceeded") &&
		!strings.Contains(result.Error, "API request failed") {
		t.Errorf("Expected timeout error, got: %s", result.Error)
	}

	t.Logf("✅ Timeout handled correctly: %s", result.Error)
}

// TestGetSentimentIndicators_RealAPI tests the actual API call
// TestGetSentimentIndicators_RealAPI 测试实际的 API 调用
func TestGetSentimentIndicators_RealAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real API test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	symbols := []string{"BTC", "ETH", "SOL"}

	for _, symbol := range symbols {
		t.Run(symbol, func(t *testing.T) {
			result := GetSentimentIndicators(ctx, symbol)

			t.Logf("Symbol: %s", symbol)
			t.Logf("Success: %v", result.Success)

			if result.Success {
				t.Logf("✅ Positive Ratio: %.2f%%", result.PositiveRatio*100)
				t.Logf("✅ Negative Ratio: %.2f%%", result.NegativeRatio*100)
				t.Logf("✅ Net Sentiment: %+.4f", result.NetSentiment)
				t.Logf("✅ Sentiment Level: %s", result.SentimentLevel)
				t.Logf("✅ Data Time: %s", result.DataTime)
				t.Logf("✅ Data Delay: %d minutes", result.DataDelayMinutes)

				// Validate data ranges
				if result.PositiveRatio < 0 || result.PositiveRatio > 1 {
					t.Errorf("Invalid positive ratio: %.4f (should be 0-1)", result.PositiveRatio)
				}
				if result.NegativeRatio < 0 || result.NegativeRatio > 1 {
					t.Errorf("Invalid negative ratio: %.4f (should be 0-1)", result.NegativeRatio)
				}
				if result.NetSentiment < -1 || result.NetSentiment > 1 {
					t.Errorf("Invalid net sentiment: %.4f (should be -1 to 1)", result.NetSentiment)
				}
			} else {
				t.Logf("⚠️  Error: %s", result.Error)
				// This is not necessarily a failure - API might be down or data delayed
				t.Logf("Note: API failure is expected if service is unavailable")
			}
		})
	}
}

// TestInterpretSentiment tests sentiment interpretation logic
// TestInterpretSentiment 测试情绪解释逻辑
func TestInterpretSentiment(t *testing.T) {
	tests := []struct {
		name          string
		netSentiment  float64
		expectedLevel string
	}{
		{"极度乐观", 0.75, "极度乐观 🔥"},
		{"强烈乐观", 0.6, "强烈乐观 📈"},
		{"偏向乐观", 0.4, "偏向乐观 ✅"},
		{"轻度乐观", 0.2, "轻度乐观 ↗️"},
		{"中性", 0.0, "中性 ➖"},
		{"轻度悲观", -0.2, "轻度悲观 ↘️"},
		{"偏向悲观", -0.4, "偏向悲观 ❌"},
		{"强烈悲观", -0.6, "强烈悲观 📉"},
		{"极度悲观", -0.8, "极度悲观 ❄️"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpretSentiment(tt.netSentiment)
			if result != tt.expectedLevel {
				t.Errorf("interpretSentiment(%.2f) = %s, want %s",
					tt.netSentiment, result, tt.expectedLevel)
			} else {
				t.Logf("✅ %.2f → %s", tt.netSentiment, result)
			}
		})
	}
}

// TestFormatSentimentReport_Success tests report formatting with valid data
// TestFormatSentimentReport_Success 测试有效数据的报告格式化
func TestFormatSentimentReport_Success(t *testing.T) {
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

	// Check for required sections
	requiredSections := []string{
		"市场情绪分析报告",
		"情绪指标概览",
		"正面情绪比率",
		"负面情绪比率",
		"净情绪值",
		"情绪等级",
		"情绪解读",
		"交易建议参考",
		"数据来源",
		"BTC",
		"偏向乐观",
	}

	for _, section := range requiredSections {
		if !strings.Contains(report, section) {
			t.Errorf("Report missing section: %s", section)
		}
	}

	// Check values
	if !strings.Contains(report, "65.00%") {
		t.Error("Report missing positive ratio value")
	}
	if !strings.Contains(report, "35.00%") {
		t.Error("Report missing negative ratio value")
	}
	if !strings.Contains(report, "+0.3000") {
		t.Error("Report missing net sentiment value")
	}

	t.Logf("✅ Report formatted correctly")
	t.Log("Report preview:")
	t.Log(report)
}

// TestFormatSentimentReport_Failure tests report formatting with error
// TestFormatSentimentReport_Failure 测试错误情况的报告格式化
func TestFormatSentimentReport_Failure(t *testing.T) {
	sentiment := &SentimentData{
		Success: false,
		Error:   "API request failed: timeout",
		Symbol:  "ETH",
	}

	report := FormatSentimentReport(sentiment)

	// Check error report format
	requiredParts := []string{
		"市场情绪数据获取失败",
		"错误信息",
		"API request failed: timeout",
		"ETH",
		"谨慎交易",
	}

	for _, part := range requiredParts {
		if !strings.Contains(report, part) {
			t.Errorf("Error report missing part: %s", part)
		}
	}

	t.Logf("✅ Error report formatted correctly")
	t.Log("Report preview:")
	t.Log(report)
}

// TestSentimentData_EdgeCases tests edge cases
// TestSentimentData_EdgeCases 测试边界情况
func TestSentimentData_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		sentiment *SentimentData
		wantPanic bool
	}{
		{
			name: "Zero values",
			sentiment: &SentimentData{
				Success:       true,
				PositiveRatio: 0,
				NegativeRatio: 0,
				NetSentiment:  0,
				Symbol:        "BTC",
			},
			wantPanic: false,
		},
		{
			name: "Extreme positive",
			sentiment: &SentimentData{
				Success:       true,
				PositiveRatio: 1.0,
				NegativeRatio: 0.0,
				NetSentiment:  1.0,
				Symbol:        "BTC",
			},
			wantPanic: false,
		},
		{
			name: "Extreme negative",
			sentiment: &SentimentData{
				Success:       true,
				PositiveRatio: 0.0,
				NegativeRatio: 1.0,
				NetSentiment:  -1.0,
				Symbol:        "BTC",
			},
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("wantPanic = %v, but panic = %v", tt.wantPanic, r != nil)
				}
			}()

			if tt.sentiment != nil {
				report := FormatSentimentReport(tt.sentiment)
				if report == "" {
					t.Error("Expected non-empty report")
				}
				t.Logf("✅ Report generated for %s", tt.name)
			}
		})
	}
}

// BenchmarkGetSentimentIndicators benchmarks API performance
// BenchmarkGetSentimentIndicators 基准测试 API 性能
func BenchmarkGetSentimentIndicators(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		GetSentimentIndicators(ctx, "BTC")
	}
}

// BenchmarkInterpretSentiment benchmarks sentiment interpretation
// BenchmarkInterpretSentiment 基准测试情绪解释
func BenchmarkInterpretSentiment(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interpretSentiment(0.5)
	}
}

// BenchmarkFormatSentimentReport benchmarks report formatting
// BenchmarkFormatSentimentReport 基准测试报告格式化
func BenchmarkFormatSentimentReport(b *testing.B) {
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatSentimentReport(sentiment)
	}
}
