package executors

import (
	"math"
	"testing"
)

func TestCalculateInitialStop(t *testing.T) {
	calc := NewTrailingStopCalculator(nil)

	tests := []struct {
		name       string
		symbol     string
		entryPrice float64
		atr        float64
		side       string
		expected   float64
	}{
		{
			name:       "Long position initial stop",
			symbol:     "BTCUSDT",
			entryPrice: 50000,
			atr:        500,
			side:       "long",
			expected:   50000 - 2.5*500, // 48750
		},
		{
			name:       "Short position initial stop",
			symbol:     "BTCUSDT",
			entryPrice: 50000,
			atr:        500,
			side:       "short",
			expected:   50000 + 2.5*500, // 51250
		},
		{
			name:       "ETH long position initial stop",
			symbol:     "ETHUSDT",
			entryPrice: 3000,
			atr:        50,
			side:       "long",
			expected:   3000 - 2.5*50, // 2875
		},
		{
			name:       "SOL short position with higher volatility",
			symbol:     "SOLUSDT",
			entryPrice: 100,
			atr:        5,
			side:       "short",
			expected:   100 + 2.5*5, // 112.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.CalculateInitialStop(tt.symbol, tt.entryPrice, tt.atr, tt.side)
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("CalculateInitialStop() = %.2f, expected %.2f", result, tt.expected)
			}
		})
	}
}

func TestCalculateTrailingStop(t *testing.T) {
	calc := NewTrailingStopCalculator(nil)

	tests := []struct {
		name         string
		symbol       string
		highestPrice float64
		atr          float64
		side         string
		expected     float64
	}{
		{
			name:         "Long position trailing stop",
			symbol:       "BTCUSDT",
			highestPrice: 52000,
			atr:          500,
			side:         "long",
			expected:     52000 - 2.5*500, // 50750
		},
		{
			name:         "Short position trailing stop",
			symbol:       "BTCUSDT",
			highestPrice: 48000, // This is actually lowest price for short
			atr:          500,
			side:         "short",
			expected:     48000 + 2.5*500, // 49250
		},
		{
			name:         "ETH long with small ATR",
			symbol:       "ETHUSDT",
			highestPrice: 3200,
			atr:          40,
			side:         "long",
			expected:     3200 - 2.5*40, // 3100
		},
		{
			name:         "SOL short with same multiplier",
			symbol:       "SOLUSDT",
			highestPrice: 95, // lowest price
			atr:          5,
			side:         "short",
			expected:     95 + 2.5*5, // 107.5 (SOL uses 2.5x multiplier)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.CalculateTrailingStop(tt.symbol, tt.highestPrice, tt.atr, tt.side)
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("CalculateTrailingStop() = %.2f, expected %.2f", result, tt.expected)
			}
		})
	}
}

func TestIsValidUpdate(t *testing.T) {
	calc := NewTrailingStopCalculator(nil)

	tests := []struct {
		name        string
		side        string
		oldStopLoss float64
		newStopLoss float64
		expected    bool
	}{
		{
			name:        "Long position - stop moves up (valid)",
			side:        "long",
			oldStopLoss: 48000,
			newStopLoss: 49000,
			expected:    true,
		},
		{
			name:        "Long position - stop moves down (invalid)",
			side:        "long",
			oldStopLoss: 49000,
			newStopLoss: 48000,
			expected:    false,
		},
		{
			name:        "Short position - stop moves down (valid)",
			side:        "short",
			oldStopLoss: 52000,
			newStopLoss: 51000,
			expected:    true,
		},
		{
			name:        "Short position - stop moves up (invalid)",
			side:        "short",
			oldStopLoss: 51000,
			newStopLoss: 52000,
			expected:    false,
		},
		{
			name:        "Long position - no change",
			side:        "long",
			oldStopLoss: 50000,
			newStopLoss: 50000,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.IsValidUpdate(tt.side, tt.oldStopLoss, tt.newStopLoss)
			if result != tt.expected {
				t.Errorf("IsValidUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestShouldUpdate(t *testing.T) {
	calc := NewTrailingStopCalculator(nil)

	tests := []struct {
		name        string
		symbol      string
		oldStopLoss float64
		newStopLoss float64
		expected    bool
	}{
		{
			name:        "BTC - change exceeds threshold",
			symbol:      "BTCUSDT",
			oldStopLoss: 50000,
			newStopLoss: 50150, // 0.3% change (exceeds 0.2% threshold)
			expected:    true,
		},
		{
			name:        "BTC - change below threshold",
			symbol:      "BTCUSDT",
			oldStopLoss: 50000,
			newStopLoss: 50050, // 0.1% change (below 0.2% threshold)
			expected:    false,
		},
		{
			name:        "SOL - change exceeds threshold",
			symbol:      "SOLUSDT",
			oldStopLoss: 100,
			newStopLoss: 100.3, // 0.3% change (exceeds 0.2% threshold)
			expected:    true,
		},
		{
			name:        "SOL - change below threshold",
			symbol:      "SOLUSDT",
			oldStopLoss: 100,
			newStopLoss: 100.1, // 0.1% change (below 0.2% threshold)
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.ShouldUpdate(tt.symbol, tt.oldStopLoss, tt.newStopLoss)
			if result != tt.expected {
				t.Errorf("ShouldUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestValidateStopDistance(t *testing.T) {
	calc := NewTrailingStopCalculator(nil)

	tests := []struct {
		name       string
		symbol     string
		entryPrice float64
		stopPrice  float64
		side       string
		expected   bool
	}{
		{
			name:       "BTC long - valid distance (2%)",
			symbol:     "BTCUSDT",
			entryPrice: 50000,
			stopPrice:  49000, // 2% below
			side:       "long",
			expected:   true,
		},
		{
			name:       "BTC long - too tight (0.3%)",
			symbol:     "BTCUSDT",
			entryPrice: 50000,
			stopPrice:  49850, // 0.3% below (below 0.5% min)
			side:       "long",
			expected:   false,
		},
		{
			name:       "BTC long - too wide (7%)",
			symbol:     "BTCUSDT",
			entryPrice: 50000,
			stopPrice:  46500, // 7% below (above 6% max for BTC)
			side:       "long",
			expected:   false,
		},
		{
			name:       "ETH short - valid distance (3%)",
			symbol:     "ETHUSDT",
			entryPrice: 3000,
			stopPrice:  3090, // 3% above
			side:       "short",
			expected:   true,
		},
		{
			name:       "SOL long - valid wider range (5%)",
			symbol:     "SOLUSDT",
			entryPrice: 100,
			stopPrice:  95, // 5% below (within 0.5%-8% for SOL)
			side:       "long",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.ValidateStopDistance(tt.symbol, tt.entryPrice, tt.stopPrice, tt.side)
			if result != tt.expected {
				t.Errorf("ValidateStopDistance() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	calc := NewTrailingStopCalculator(nil)

	tests := []struct {
		name                       string
		symbol                     string
		expectedTrailingMultiplier float64
	}{
		{
			name:                       "BTC config",
			symbol:                     "BTCUSDT",
			expectedTrailingMultiplier: 2.5,
		},
		{
			name:                       "ETH config",
			symbol:                     "ETHUSDT",
			expectedTrailingMultiplier: 2.5,
		},
		{
			name:                       "SOL config - same multiplier",
			symbol:                     "SOLUSDT",
			expectedTrailingMultiplier: 2.5,
		},
		{
			name:                       "Unknown symbol - uses default",
			symbol:                     "XYZUSDT",
			expectedTrailingMultiplier: 2.5, // DEFAULT config
		},
		{
			name:                       "Symbol with slash",
			symbol:                     "BTC/USDT",
			expectedTrailingMultiplier: 2.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := calc.GetConfig(tt.symbol)
			if config.TrailingATRMultiplier != tt.expectedTrailingMultiplier {
				t.Errorf("GetConfig().TrailingATRMultiplier = %.1f, expected %.1f",
					config.TrailingATRMultiplier, tt.expectedTrailingMultiplier)
			}
		})
	}
}

func TestTrailingStopScenario(t *testing.T) {
	// Integration test: simulate a complete trailing stop scenario
	// 集成测试：模拟一个完整的追踪止损场景
	calc := NewTrailingStopCalculator(nil)

	// Scenario: BTC long position
	// 场景：BTC 多仓
	symbol := "BTCUSDT"
	entryPrice := 50000.0
	atr := 500.0

	// 1. Calculate initial stop
	// 1. 计算初始止损
	initialStop := calc.CalculateInitialStop(symbol, entryPrice, atr, "long")
	expectedInitialStop := 48750.0 // 50000 - 2.5*500
	if math.Abs(initialStop-expectedInitialStop) > 0.01 {
		t.Errorf("Initial stop = %.2f, expected %.2f", initialStop, expectedInitialStop)
	}

	// 2. Price rises to 52000, calculate trailing stop
	// 2. 价格上涨到 52000，计算追踪止损
	highestPrice := 52000.0
	trailingStop1 := calc.CalculateTrailingStop(symbol, highestPrice, atr, "long")
	expectedTrailing1 := 50750.0 // 52000 - 2.5*500
	if math.Abs(trailingStop1-expectedTrailing1) > 0.01 {
		t.Errorf("Trailing stop 1 = %.2f, expected %.2f", trailingStop1, expectedTrailing1)
	}

	// 3. Validate update (should move up)
	// 3. 验证更新（应该向上移动）
	if !calc.IsValidUpdate("long", initialStop, trailingStop1) {
		t.Error("Trailing stop should be valid (moving up)")
	}

	// 4. Check if update threshold is met
	// 4. 检查是否超过更新阈值
	if !calc.ShouldUpdate(symbol, initialStop, trailingStop1) {
		t.Error("Change should exceed threshold")
	}

	// 5. Price rises to 53000, calculate new trailing stop
	// 5. 价格上涨到 53000，计算新的追踪止损
	highestPrice = 53000.0
	trailingStop2 := calc.CalculateTrailingStop(symbol, highestPrice, atr, "long")
	expectedTrailing2 := 51750.0 // 53000 - 2.5*500
	if math.Abs(trailingStop2-expectedTrailing2) > 0.01 {
		t.Errorf("Trailing stop 2 = %.2f, expected %.2f", trailingStop2, expectedTrailing2)
	}

	// 6. Validate update (should move up from previous trailing stop)
	// 6. 验证更新（应该从之前的追踪止损向上移动）
	if !calc.IsValidUpdate("long", trailingStop1, trailingStop2) {
		t.Error("New trailing stop should be higher than previous")
	}

	// 7. Try to move stop down (should be invalid)
	// 7. 尝试向下移动止损（应该无效）
	if calc.IsValidUpdate("long", trailingStop2, trailingStop1) {
		t.Error("Moving stop down should be invalid for long position")
	}

	t.Logf("Scenario test passed:")
	t.Logf("  Entry: $%.2f", entryPrice)
	t.Logf("  Initial stop: $%.2f", initialStop)
	t.Logf("  Trailing stop 1 (@ $52000): $%.2f", trailingStop1)
	t.Logf("  Trailing stop 2 (@ $53000): $%.2f", trailingStop2)
}
