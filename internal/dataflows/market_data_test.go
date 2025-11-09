package dataflows

import (
	"math"
	"testing"
	"time"
)

func TestCalculateSMA(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	period := 3

	result := calculateSMA(data, period)

	// 前两个值应该是 NaN（因为周期是 3）
	if !math.IsNaN(result[0]) || !math.IsNaN(result[1]) {
		t.Errorf("First two values should be NaN")
	}

	// 第三个值应该是 (1+2+3)/3 = 2
	expected := 2.0
	if math.Abs(result[2]-expected) > 0.0001 {
		t.Errorf("SMA[2]: expected %f, got %f", expected, result[2])
	}

	// 最后一个值应该是 (8+9+10)/3 = 9
	expected = 9.0
	if math.Abs(result[9]-expected) > 0.0001 {
		t.Errorf("SMA[9]: expected %f, got %f", expected, result[9])
	}
}

func TestCalculateEMA(t *testing.T) {
	data := []float64{22.27, 22.19, 22.08, 22.17, 22.18, 22.13, 22.23, 22.43, 22.24, 22.29}
	period := 5

	result := calculateEMA(data, period)

	// 检查结果长度
	if len(result) != len(data) {
		t.Errorf("EMA length: expected %d, got %d", len(data), len(result))
	}

	// 前几个值应该是 NaN
	if !math.IsNaN(result[0]) {
		t.Errorf("First value should be NaN")
	}

	// EMA 应该是递增的（在这个数据集中）
	if result[len(result)-1] < 22.0 || result[len(result)-1] > 23.0 {
		t.Errorf("EMA last value seems incorrect: %f", result[len(result)-1])
	}
}

func TestCalculateRSI(t *testing.T) {
	// 使用已知的测试数据
	data := []float64{
		44.34, 44.09, 43.61, 44.33, 44.83,
		45.10, 45.42, 45.84, 46.08, 45.89,
		46.03, 45.61, 46.28, 46.28, 46.00,
		46.03, 46.41, 46.22, 45.64,
	}
	period := 14

	result := calculateRSI(data, period)

	// 检查结果长度
	if len(result) != len(data) {
		t.Errorf("RSI length: expected %d, got %d", len(data), len(result))
	}

	// RSI 应该在 0-100 之间
	for i, val := range result {
		if !math.IsNaN(val) {
			if val < 0 || val > 100 {
				t.Errorf("RSI[%d] out of range [0,100]: %f", i, val)
			}
		}
	}

	// 最后一个 RSI 值应该在合理范围内（约 70 左右，因为是上升趋势）
	lastRSI := result[len(result)-1]
	if lastRSI < 50 || lastRSI > 100 {
		t.Errorf("RSI last value seems incorrect: %f", lastRSI)
	}
}

func TestCalculateMACD(t *testing.T) {
	// 生成测试数据
	data := make([]float64, 100)
	for i := range data {
		data[i] = 100.0 + float64(i)*0.5 // 上升趋势
	}

	macd, signal := calculateMACD(data)

	// 检查结果长度
	if len(macd) != len(data) {
		t.Errorf("MACD length: expected %d, got %d", len(data), len(macd))
	}
	if len(signal) != len(data) {
		t.Errorf("Signal length: expected %d, got %d", len(data), len(signal))
	}

	// 前面的值应该是 NaN
	if !math.IsNaN(macd[0]) {
		t.Errorf("First MACD value should be NaN")
	}

	// 在上升趋势中，MACD 应该是正值
	lastMACD := macd[len(macd)-1]
	if math.IsNaN(lastMACD) || lastMACD <= 0 {
		t.Errorf("MACD in uptrend should be positive, got: %f", lastMACD)
	}
}

func TestCalculateBollingerBands(t *testing.T) {
	data := []float64{
		22.27, 22.19, 22.08, 22.17, 22.18,
		22.13, 22.23, 22.43, 22.24, 22.29,
		22.15, 22.39, 22.38, 22.61, 23.36,
		24.05, 23.75, 23.83, 23.95, 23.63,
	}
	period := 20
	stdDev := 2.0

	upper, middle, lower := calculateBollingerBands(data, period, stdDev)

	// 检查长度
	if len(upper) != len(data) || len(middle) != len(data) || len(lower) != len(data) {
		t.Errorf("Bollinger Bands length mismatch")
	}

	// 检查最后一个值的关系：upper > middle > lower
	lastIdx := len(data) - 1
	if !math.IsNaN(upper[lastIdx]) {
		if !(upper[lastIdx] > middle[lastIdx] && middle[lastIdx] > lower[lastIdx]) {
			t.Errorf("Bollinger Bands relationship incorrect: upper=%f, middle=%f, lower=%f",
				upper[lastIdx], middle[lastIdx], lower[lastIdx])
		}
	}
}

func TestCalculateATR(t *testing.T) {
	highs := []float64{48.70, 48.72, 48.90, 48.87, 48.82, 49.05, 49.20, 49.35, 49.92, 50.19}
	lows := []float64{47.79, 48.14, 48.39, 48.37, 48.24, 48.64, 48.94, 48.86, 49.50, 49.87}
	closes := []float64{48.16, 48.61, 48.75, 48.63, 48.74, 49.03, 49.07, 49.32, 49.91, 50.13}
	period := 14

	result := calculateATR(highs, lows, closes, period)

	// 检查长度
	if len(result) != len(highs) {
		t.Errorf("ATR length: expected %d, got %d", len(highs), len(result))
	}

	// ATR 应该是正值
	for i, val := range result {
		if !math.IsNaN(val) {
			if val <= 0 {
				t.Errorf("ATR[%d] should be positive: %f", i, val)
			}
		}
	}
}

func TestTechnicalIndicatorsStructure(t *testing.T) {
	// 创建测试用的 OHLCV 数据
	ohlcvData := make([]OHLCV, 50)
	baseTime := time.Now()
	for i := range ohlcvData {
		price := 100.0 + float64(i)*0.5
		ohlcvData[i] = OHLCV{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Open:      price - 0.2,
			High:      price + 0.3,
			Low:       price - 0.5,
			Close:     price,
			Volume:    1000.0 + float64(i)*10,
		}
	}

	// 手动计算一些指标来验证
	closes := make([]float64, len(ohlcvData))
	highs := make([]float64, len(ohlcvData))
	lows := make([]float64, len(ohlcvData))
	for i, k := range ohlcvData {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	rsi := calculateRSI(closes, 14)
	macd, signal := calculateMACD(closes)
	upper, middle, lower := calculateBollingerBands(closes, 20, 2.0)
	atr := calculateATR(highs, lows, closes, 14)

	// 检查所有指标的长度
	if len(rsi) != len(ohlcvData) {
		t.Errorf("RSI length mismatch: expected %d, got %d", len(ohlcvData), len(rsi))
	}
	if len(macd) != len(ohlcvData) {
		t.Errorf("MACD length mismatch: expected %d, got %d", len(ohlcvData), len(macd))
	}
	if len(signal) != len(ohlcvData) {
		t.Errorf("Signal length mismatch: expected %d, got %d", len(ohlcvData), len(signal))
	}
	if len(upper) != len(ohlcvData) {
		t.Errorf("BollingerUpper length mismatch: expected %d, got %d", len(ohlcvData), len(upper))
	}
	if len(middle) != len(ohlcvData) {
		t.Errorf("BollingerMiddle length mismatch: expected %d, got %d", len(ohlcvData), len(middle))
	}
	if len(lower) != len(ohlcvData) {
		t.Errorf("BollingerLower length mismatch: expected %d, got %d", len(ohlcvData), len(lower))
	}
	if len(atr) != len(ohlcvData) {
		t.Errorf("ATR length mismatch: expected %d, got %d", len(ohlcvData), len(atr))
	}

	// 检查最新值是否有效（非 NaN）
	lastIdx := len(ohlcvData) - 1
	if math.IsNaN(rsi[lastIdx]) {
		t.Errorf("Latest RSI should not be NaN")
	}
}
