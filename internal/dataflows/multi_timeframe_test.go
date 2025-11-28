package dataflows

import (
	"math"
	"strings"
	"testing"
)

// TestMultiTimeframeIndicatorStructure tests the MultiTimeframeIndicator structure
// TestMultiTimeframeIndicatorStructure 测试 MultiTimeframeIndicator 结构体
func TestMultiTimeframeIndicatorStructure(t *testing.T) {
	indicator := MultiTimeframeIndicator{
		Timeframe: "1h",
		EMA20:     86000.0,
		EMA50:     85500.0,
		MACD:      -50.0,
		RSI7:      48.5,
		RSI14:     52.0,
	}

	if indicator.Timeframe != "1h" {
		t.Errorf("Expected timeframe '1h', got '%s'", indicator.Timeframe)
	}
	if indicator.EMA20 != 86000.0 {
		t.Errorf("Expected EMA20 86000.0, got %f", indicator.EMA20)
	}
	if indicator.EMA50 != 85500.0 {
		t.Errorf("Expected EMA50 85500.0, got %f", indicator.EMA50)
	}
	if indicator.MACD != -50.0 {
		t.Errorf("Expected MACD -50.0, got %f", indicator.MACD)
	}
	if indicator.RSI7 != 48.5 {
		t.Errorf("Expected RSI7 48.5, got %f", indicator.RSI7)
	}
	if indicator.RSI14 != 52.0 {
		t.Errorf("Expected RSI14 52.0, got %f", indicator.RSI14)
	}
}

// TestFormatMultiTimeframeReport tests the formatting of multi-timeframe report
// TestFormatMultiTimeframeReport 测试多时间框架报告的格式化
func TestFormatMultiTimeframeReport(t *testing.T) {
	tests := []struct {
		name         string
		indicators   []MultiTimeframeIndicator
		wantEmpty    bool
		wantContains []string
	}{
		{
			name:       "EmptyIndicators",
			indicators: []MultiTimeframeIndicator{},
			wantEmpty:  true,
		},
		{
			name: "SingleTimeframe",
			indicators: []MultiTimeframeIndicator{
				{
					Timeframe: "1h",
					EMA20:     86000.0,
					EMA50:     85500.0,
					MACD:      -50.0,
					RSI7:      48.5,
					RSI14:     52.0,
				},
			},
			wantEmpty: false,
			wantContains: []string{
				"多时间框架指标：",
				"1小时",
				"EMA20=86000.000",
				"EMA50=85500.000",
				"MACD=-50.000",
				"RSI7=48.50",
				"RSI14=52.00",
			},
		},
		{
			name: "MultipleTimeframes",
			indicators: []MultiTimeframeIndicator{
				{
					Timeframe: "3m",
					EMA20:     86014.071,
					EMA50:     86066.329,
					MACD:      -28.439,
					RSI7:      48.63,
					RSI14:     51.64,
				},
				{
					Timeframe: "5m",
					EMA20:     86034.846,
					EMA50:     86163.901,
					MACD:      -41.399,
					RSI7:      38.35,
					RSI14:     41.86,
				},
				{
					Timeframe: "15m",
					EMA20:     86217.472,
					EMA50:     86549.920,
					MACD:      -216.439,
					RSI7:      50.81,
					RSI14:     51.48,
				},
			},
			wantEmpty: false,
			wantContains: []string{
				"多时间框架指标：",
				"3分钟",
				"5分钟",
				"15分钟",
			},
		},
		{
			name: "WithNaNValues",
			indicators: []MultiTimeframeIndicator{
				{
					Timeframe: "1h",
					EMA20:     math.NaN(),
					EMA50:     85500.0,
					MACD:      math.NaN(),
					RSI7:      48.5,
					RSI14:     math.NaN(),
				},
			},
			wantEmpty: false,
			wantContains: []string{
				"多时间框架指标：",
				"1小时",
				"EMA20=N/A",
				"EMA50=85500.000",
				"MACD=N/A",
				"RSI7=48.50",
				"RSI14=N/A",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMultiTimeframeReport(tt.indicators)

			if tt.wantEmpty {
				if result != "" {
					t.Errorf("Expected empty result, got: %s", result)
				}
				return
			}

			// Check that result is not empty
			if result == "" {
				t.Error("Expected non-empty result, got empty string")
			}

			// Check for expected substrings
			for _, substr := range tt.wantContains {
				if !strings.Contains(result, substr) {
					t.Errorf("Expected result to contain '%s', but it doesn't.\nGot: %s", substr, result)
				}
			}
		})
	}
}

// TestFormatMultiTimeframeReportFormat tests the exact format of the output
// TestFormatMultiTimeframeReportFormat 测试输出的精确格式
func TestFormatMultiTimeframeReportFormat(t *testing.T) {
	indicators := []MultiTimeframeIndicator{
		{
			Timeframe: "3m",
			EMA20:     86014.071,
			EMA50:     86066.329,
			MACD:      -28.439,
			RSI7:      48.63,
			RSI14:     51.64,
		},
	}

	result := FormatMultiTimeframeReport(indicators)

	// Check header
	if !strings.HasPrefix(result, "多时间框架指标：\n") {
		t.Errorf("Expected result to start with '多时间框架指标：\\n', got: %s", result)
	}

	// Check that it contains exactly one line for the indicator (plus header)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 { // Header + 1 indicator line
		t.Errorf("Expected 2 lines (header + 1 indicator), got %d lines", len(lines))
	}

	// Check format of indicator line
	indicatorLine := lines[1]
	expectedParts := []string{"3分钟:", "EMA20=", "EMA50=", "MACD=", "RSI7=", "RSI14="}
	for _, part := range expectedParts {
		if !strings.Contains(indicatorLine, part) {
			t.Errorf("Expected indicator line to contain '%s', got: %s", part, indicatorLine)
		}
	}
}
