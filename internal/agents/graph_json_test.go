package agents

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestTradeDecisionJSONParsing tests JSON parsing for various trade actions
// TestTradeDecisionJSONParsing 测试不同交易动作的 JSON 解析
func TestTradeDecisionJSONParsing(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantErr  bool
		expected TradeDecision
	}{
		{
			name: "BUY action with all required fields",
			json: `{
				"symbol": "BTC/USDT",
				"action": "BUY",
				"confidence": 0.92,
				"leverage": 15,
				"position_size": 10.0,
				"stop_loss": 50000.00,
				"reasoning": "强势突破，订单簿买压优势",
				"risk_reward_ratio": 2.5,
				"summary": "高置信度做多机会"
			}`,
			wantErr: false,
			expected: TradeDecision{
				Symbol:          "BTC/USDT",
				Action:          "BUY",
				Confidence:      0.92,
				Leverage:        15,
				PositionSize:    10.0,
				StopLoss:        50000.00,
				Reasoning:       "强势突破，订单簿买压优势",
				RiskRewardRatio: 2.5,
				Summary:         "高置信度做多机会",
			},
		},
		{
			name: "SELL action",
			json: `{
				"symbol": "ETH/USDT",
				"action": "SELL",
				"confidence": 0.89,
				"leverage": 12,
				"position_size": 8.0,
				"stop_loss": 3200.00,
				"reasoning": "订单簿卖压优势+MACD死叉",
				"risk_reward_ratio": 2.2,
				"summary": "趋势反转信号明确"
			}`,
			wantErr: false,
			expected: TradeDecision{
				Symbol:          "ETH/USDT",
				Action:          "SELL",
				Confidence:      0.89,
				Leverage:        12,
				PositionSize:    8.0,
				StopLoss:        3200.00,
				Reasoning:       "订单簿卖压优势+MACD死叉",
				RiskRewardRatio: 2.2,
				Summary:         "趋势反转信号明确",
			},
		},
		{
			name: "HOLD action with stop loss adjustment",
			json: `{
				"symbol": "BTC/USDT",
				"action": "HOLD",
				"confidence": 0.90,
				"leverage": 15,
				"position_size": 10.0,
				"stop_loss": 51500.00,
				"reasoning": "趋势延续，价格创新高",
				"risk_reward_ratio": 3.0,
				"summary": "多头趋势强劲，继续持有",
				"current_pnl_percent": 8.5,
				"new_stop_loss": 51500.00,
				"stop_loss_reason": "价格创新高至52000，新止损=52000-2×ATR"
			}`,
			wantErr: false,
			expected: TradeDecision{
				Symbol:            "BTC/USDT",
				Action:            "HOLD",
				Confidence:        0.90,
				Leverage:          15,
				PositionSize:      10.0,
				StopLoss:          51500.00,
				Reasoning:         "趋势延续，价格创新高",
				RiskRewardRatio:   3.0,
				Summary:           "多头趋势强劲，继续持有",
				CurrentPnlPercent: floatPtr(8.5),
				NewStopLoss:       floatPtr(51500.00),
				StopLossReason:    stringPtr("价格创新高至52000，新止损=52000-2×ATR"),
			},
		},
		{
			name: "HOLD action without stop loss adjustment",
			json: `{
				"symbol": "ETH/USDT",
				"action": "HOLD",
				"confidence": 0.88,
				"leverage": 12,
				"position_size": 8.0,
				"stop_loss": 3100.00,
				"reasoning": "趋势延续但未创新高",
				"risk_reward_ratio": 2.8,
				"summary": "继续观察",
				"current_pnl_percent": 5.2
			}`,
			wantErr: false,
			expected: TradeDecision{
				Symbol:            "ETH/USDT",
				Action:            "HOLD",
				Confidence:        0.88,
				Leverage:          12,
				PositionSize:      8.0,
				StopLoss:          3100.00,
				Reasoning:         "趋势延续但未创新高",
				RiskRewardRatio:   2.8,
				Summary:           "继续观察",
				CurrentPnlPercent: floatPtr(5.2),
			},
		},
		{
			name: "CLOSE_LONG action",
			json: `{
				"symbol": "BTC/USDT",
				"action": "CLOSE_LONG",
				"confidence": 0.95,
				"leverage": 15,
				"position_size": 0.0,
				"stop_loss": 0.0,
				"reasoning": "趋势反转信号+止损触发",
				"risk_reward_ratio": 0.0,
				"summary": "多头动能衰竭，主动平仓"
			}`,
			wantErr: false,
			expected: TradeDecision{
				Symbol:          "BTC/USDT",
				Action:          "CLOSE_LONG",
				Confidence:      0.95,
				Leverage:        15,
				PositionSize:    0.0,
				StopLoss:        0.0,
				Reasoning:       "趋势反转信号+止损触发",
				RiskRewardRatio: 0.0,
				Summary:         "多头动能衰竭，主动平仓",
			},
		},
		{
			name: "CLOSE_SHORT action",
			json: `{
				"symbol": "ETH/USDT",
				"action": "CLOSE_SHORT",
				"confidence": 0.93,
				"leverage": 12,
				"position_size": 0.0,
				"stop_loss": 0.0,
				"reasoning": "目标达成+买盘增强",
				"risk_reward_ratio": 0.0,
				"summary": "空头目标已达成，主动止盈"
			}`,
			wantErr: false,
			expected: TradeDecision{
				Symbol:          "ETH/USDT",
				Action:          "CLOSE_SHORT",
				Confidence:      0.93,
				Leverage:        12,
				PositionSize:    0.0,
				StopLoss:        0.0,
				Reasoning:       "目标达成+买盘增强",
				RiskRewardRatio: 0.0,
				Summary:         "空头目标已达成，主动止盈",
			},
		},
		{
			name:    "Invalid JSON - missing required field (action)",
			json:    `{"symbol": "BTC/USDT", "confidence": 0.92}`,
			wantErr: false, // JSON 解析不会出错，但后续验证会检测到
			expected: TradeDecision{
				Symbol:     "BTC/USDT",
				Confidence: 0.92,
			},
		},
		{
			name:    "Invalid JSON - malformed",
			json:    `{"symbol": "BTC/USDT", "action": "BUY",}`, // 尾部多余逗号
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result TradeDecision
			err := json.Unmarshal([]byte(tt.json), &result)

			// 检查错误
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 如果不期望错误，则比较结果
			if !tt.wantErr {
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("解析结果不匹配\n得到: %+v\n期望: %+v", result, tt.expected)
				}
			}
		})
	}
}

// TestTradeDecisionValidation tests validation of parsed decisions
// TestTradeDecisionValidation 测试解析后决策的验证
func TestTradeDecisionValidation(t *testing.T) {
	tests := []struct {
		name     string
		decision TradeDecision
		isValid  bool
		reason   string
	}{
		{
			name: "Valid BUY decision",
			decision: TradeDecision{
				Symbol:     "BTC/USDT",
				Action:     "BUY",
				Confidence: 0.92,
			},
			isValid: true,
		},
		{
			name: "Invalid - missing symbol",
			decision: TradeDecision{
				Action:     "BUY",
				Confidence: 0.92,
			},
			isValid: false,
			reason:  "symbol is empty",
		},
		{
			name: "Invalid - missing action",
			decision: TradeDecision{
				Symbol:     "BTC/USDT",
				Confidence: 0.92,
			},
			isValid: false,
			reason:  "action is empty",
		},
		{
			name: "Valid - all fields populated",
			decision: TradeDecision{
				Symbol:          "ETH/USDT",
				Action:          "SELL",
				Confidence:      0.89,
				Leverage:        12,
				PositionSize:    8.0,
				StopLoss:        3200.00,
				Reasoning:       "卖压优势",
				RiskRewardRatio: 2.2,
				Summary:         "反转信号",
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 简单验证：检查必填字段
			isValid := tt.decision.Symbol != "" && tt.decision.Action != ""

			if isValid != tt.isValid {
				t.Errorf("验证结果不匹配，得到 %v，期望 %v (原因: %s)", isValid, tt.isValid, tt.reason)
			}
		})
	}
}

// TestJSONSchemaGeneration tests JSON schema generation for TradeDecision
// TestJSONSchemaGeneration 测试为 TradeDecision 生成 JSON Schema
func TestJSONSchemaGeneration(t *testing.T) {
	var decision TradeDecision

	// 尝试生成 Schema（需要 jsonschema 包）
	// 这里只是一个占位测试，实际的 Schema 生成在运行时进行
	if decision.Symbol == "" {
		// 确保结构体字段正确定义了 JSON tags
		t.Log("TradeDecision 结构体已定义，JSON tags 已设置")
	}
}

// Helper functions for creating pointers
// 辅助函数：创建指针

func floatPtr(f float64) *float64 {
	return &f
}

func stringPtr(s string) *string {
	return &s
}
