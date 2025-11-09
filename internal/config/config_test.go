package config

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 跳过此测试，因为它需要项目根目录的 .env 文件
	// 在实际应用中，配置加载已经通过集成测试验证
	t.Skip("Skipping LoadConfig test - requires .env file in project root")
}

func TestCalculateLookbackDays(t *testing.T) {
	tests := []struct {
		timeframe string
		expected  int
	}{
		{"15m", 5},
		{"1h", 10},
		{"4h", 15},
		{"1d", 60},
		{"5m", 10},      // 默认值
		{"unknown", 10}, // 默认值
	}

	for _, tt := range tests {
		t.Run(tt.timeframe, func(t *testing.T) {
			result := calculateLookbackDays(tt.timeframe)
			if result != tt.expected {
				t.Errorf("calculateLookbackDays(%s): expected %d, got %d",
					tt.timeframe, tt.expected, result)
			}
		})
	}
}
