package agents

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oak/crypto-trading-bot/internal/executors"
)

// TradingDecision represents a parsed trading decision from LLM
// TradingDecision 表示从 LLM 解析出的交易决策
type TradingDecision struct {
	Action     executors.TradeAction // 交易动作 / Trading action
	Confidence float64               // 置信度 0-1 / Confidence level 0-1
	Leverage   int                   // 杠杆倍数 / Leverage multiplier
	Reason     string                // 决策理由 / Decision reason
	Symbol     string                // 交易对 / Trading pair
	StopLoss   float64               // 止损价格 / Stop-loss price
	Valid      bool                  // 决策是否有效 / Whether decision is valid
}

// ParseDecision parses LLM decision text and extracts trading action
// ParseDecision 解析 LLM 决策文本并提取交易动作
func ParseDecision(decisionText string, symbol string) *TradingDecision {
	decision := &TradingDecision{
		Symbol: symbol,
		Valid:  false,
	}

	// Convert to lowercase for case-insensitive matching
	// 转换为小写以进行不区分大小写的匹配
	text := strings.ToLower(decisionText)

	// Extract action using multiple patterns
	// 使用多种模式提取交易动作
	action := extractAction(text)
	if action == "" {
		decision.Reason = "无法从决策文本中识别明确的交易动作"
		return decision
	}

	// Map action string to TradeAction enum
	// 将动作字符串映射到 TradeAction 枚举
	decision.Action = mapToTradeAction(action)
	if decision.Action == "" {
		decision.Reason = fmt.Sprintf("未知的交易动作: %s", action)
		return decision
	}

	// Extract confidence (optional)
	// 提取置信度（可选）
	decision.Confidence = extractConfidence(text)

	// Extract leverage (optional)
	// 提取杠杆倍数（可选）
	decision.Leverage = extractLeverage(text)

	// Extract stop-loss price (NEW!)
	// 提取止损价格（新功能）
	decision.StopLoss = extractStopLoss(text)

	// Extract reason
	// 提取理由
	decision.Reason = extractReason(decisionText)

	// Mark as valid
	// 标记为有效
	decision.Valid = true

	return decision
}

// extractAction extracts trading action from text using regex patterns
// extractAction 使用正则表达式从文本中提取交易动作
func extractAction(text string) string {
	// First try to extract from decision markers (highest priority)
	// 首先尝试从决策标记中提取（最高优先级）
	decisionPatterns := []string{
		`(?:最终决策|决策方向|交易方向)[：:*\s]*\*?\*?([A-Z_]+)`,
		`(?:decision|action)[：:*\s]*\*?\*?([A-Z_]+)`,
	}

	for _, pattern := range decisionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			action := strings.ToLower(strings.TrimSpace(matches[1]))
			return action
		}
	}

	// Patterns for different actions
	// 不同动作的匹配模式
	patterns := map[string][]string{
		"buy": {
			`建议.*?做多`,
			`建议.*?买入`,
			`建议.*?开多`,
			`action.*?buy`,
			`recommend.*?buy`,
			`decision.*?buy`,
			`做多`,
			`开多仓`,
			`买入`,
		},
		"sell": {
			`建议.*?做空`,
			`建议.*?卖出`,
			`建议.*?开空`,
			`action.*?sell`,
			`recommend.*?sell`,
			`decision.*?sell`,
			`做空`,
			`开空仓`,
			`卖出`,
		},
		"close_long": {
			`建议.*?平多`,
			`建议.*?平掉多单`,
			`close.*?long`,
			`平多仓`,
			`平掉多头`,
		},
		"close_short": {
			`建议.*?平空`,
			`建议.*?平掉空单`,
			`close.*?short`,
			`平空仓`,
			`平掉空头`,
		},
		"hold": {
			`建议.*?观望`,
			`建议.*?持有`,
			`建议.*?等待`,
			`action.*?hold`,
			`recommend.*?hold`,
			`decision.*?hold`,
			`观望`,
			`持有`,
			`不建议操作`,
		},
	}

	// Try each pattern
	// 尝试每个模式
	for action, patternList := range patterns {
		for _, pattern := range patternList {
			matched, _ := regexp.MatchString(pattern, text)
			if matched {
				return action
			}
		}
	}

	return ""
}

// mapToTradeAction maps action string to TradeAction enum
// mapToTradeAction 将动作字符串映射到 TradeAction 枚举
func mapToTradeAction(action string) executors.TradeAction {
	switch action {
	case "buy":
		return executors.ActionBuy
	case "sell":
		return executors.ActionSell
	case "close_long":
		return executors.ActionCloseLong
	case "close_short":
		return executors.ActionCloseShort
	case "hold":
		return executors.ActionHold
	default:
		return ""
	}
}

// extractConfidence extracts confidence level from text
// extractConfidence 从文本中提取置信度
func extractConfidence(text string) float64 {
	// Look for confidence patterns like "置信度: 0.8" or "confidence: 80%"
	// 查找置信度模式，如 "置信度: 0.8" 或 "confidence: 80%"
	patterns := []string{
		`置信度[：:]\s*([0-9.]+)`,
		`confidence[：:]\s*([0-9.]+)`,
		`信心[：:]\s*([0-9]+)%`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			var conf float64
			fmt.Sscanf(matches[1], "%f", &conf)
			if conf > 1 {
				conf = conf / 100.0 // Convert percentage to decimal / 将百分比转换为小数
			}
			return conf
		}
	}

	// Default confidence
	// 默认置信度
	return 0.7
}

// extractLeverage extracts leverage multiplier from text
// extractLeverage 从文本中提取杠杆倍数
func extractLeverage(text string) int {
	// Look for leverage patterns like "杠杆倍数: 15" or "leverage: 15x"
	// 查找杠杆模式，如 "杠杆倍数: 15" 或 "leverage: 15x"
	patterns := []string{
		`杠杆倍数[：:]\s*([0-9]+)`,
		`杠杆[：:]\s*([0-9]+)`,
		`leverage[：:]\s*([0-9]+)`,
		`([0-9]+)倍杠杆`,
		`([0-9]+)x\s+leverage`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			var leverage int
			fmt.Sscanf(matches[1], "%d", &leverage)
			if leverage >= 1 && leverage <= 125 {
				return leverage
			}
		}
	}

	// If no leverage found, return 0 (will use config default)
	// 如果未找到杠杆，返回 0（将使用配置默认值）
	return 0
}

// extractStopLoss extracts stop-loss price from text
// extractStopLoss 从文本中提取止损价格
func extractStopLoss(text string) float64 {
	// Look for stop-loss patterns (supports Markdown formatting like **)
	// 查找止损模式（支持 Markdown 格式如 **）
	patterns := []string{
		`\*?\*?止损[价格价位]*\*?\*?[：:]\s*\$?([0-9,.]+)`,
		`\*?\*?初始止损\*?\*?[：:]\s*\$?([0-9,.]+)`,
		`\*?\*?stop[-\s]?loss\*?\*?[：:]\s*\$?([0-9,.]+)`,
		`\*?\*?止损价\*?\*?[：:]\s*\$?([0-9,.]+)`,
		`\*?\*?止损点\*?\*?[：:]\s*\$?([0-9,.]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			// Clean the matched string (remove commas and dollar signs)
			// 清理匹配的字符串（移除逗号和美元符号）
			priceStr := strings.ReplaceAll(matches[1], ",", "")
			priceStr = strings.ReplaceAll(priceStr, "$", "")
			var stopLoss float64
			if _, err := fmt.Sscanf(priceStr, "%f", &stopLoss); err == nil && stopLoss > 0 {
				return stopLoss
			}
		}
	}

	// If no explicit stop-loss found, return 0 (will be calculated programmatically)
	// 如果未找到明确的止损价格，返回 0（将由程序计算）
	return 0
}

// extractReason extracts the decision reason from text
// extractReason 从文本中提取决策理由
func extractReason(text string) string {
	// Look for reason patterns
	// 查找理由模式
	patterns := []string{
		`理由[：:]\s*(.+?)(?:\n|$)`,
		`原因[：:]\s*(.+?)(?:\n|$)`,
		`reason[：:]\s*(.+?)(?:\n|$)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	// If no specific reason pattern, try to extract first meaningful sentence
	// 如果没有特定的理由模式，尝试提取第一个有意义的句子
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 20 && !strings.HasPrefix(line, "#") {
			return line
		}
	}

	return "未提供明确理由"
}

// ValidateDecision performs safety checks on the decision
// ValidateDecision 对决策执行安全检查
func ValidateDecision(decision *TradingDecision, currentPosition *executors.Position) error {
	if !decision.Valid {
		return fmt.Errorf("无效的决策")
	}

	// Check for conflicting actions
	// 检查冲突的动作
	if currentPosition != nil {
		switch decision.Action {
		case executors.ActionBuy:
			if currentPosition.Side == "long" {
				return fmt.Errorf("已有多仓，不能重复开多")
			}
		case executors.ActionSell:
			if currentPosition.Side == "short" {
				return fmt.Errorf("已有空仓，不能重复开空")
			}
		case executors.ActionCloseLong:
			if currentPosition.Side != "long" {
				return fmt.Errorf("没有多仓可平")
			}
		case executors.ActionCloseShort:
			if currentPosition.Side != "short" {
				return fmt.Errorf("没有空仓可平")
			}
		}
	}

	return nil
}

// ParseMultiCurrencyDecision parses multi-currency decision text and extracts trading actions for each symbol
// ParseMultiCurrencyDecision 解析多币种决策文本并为每个交易对提取交易动作
func ParseMultiCurrencyDecision(decisionText string, symbols []string) map[string]*TradingDecision {
	decisions := make(map[string]*TradingDecision)

	// Convert to lowercase for matching
	// 转换为小写用于匹配
	text := strings.ToLower(decisionText)

	// Try to find decision blocks for each symbol
	// 尝试为每个交易对找到决策块
	for _, symbol := range symbols {
		// Create patterns for this symbol (e.g., "btc/usdt", "btc", "【btc/usdt】")
		// 为该交易对创建模式
		symbolLower := strings.ToLower(symbol)
		baseSymbol := strings.Split(symbolLower, "/")[0] // e.g., "btc" from "btc/usdt"

		// Find the decision block for this symbol
		// 查找该交易对的决策块
		patterns := []string{
			fmt.Sprintf(`【%s】(.{0,500}?)(?:【|$)`, symbolLower),
			fmt.Sprintf(`【%s】(.{0,500}?)(?:【|$)`, baseSymbol),
			fmt.Sprintf(`%s(.{0,500}?)(?:\n\n|$)`, symbolLower),
		}

		var blockText string
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(text)
			if len(matches) > 1 {
				blockText = matches[1]
				break
			}
		}

		// If we found a block, parse it
		// 如果找到了决策块，解析它
		if blockText != "" {
			decision := ParseDecision(blockText, symbol)
			decisions[symbol] = decision
		} else {
			// No specific block found, default to HOLD
			// 未找到特定决策块，默认为 HOLD
			decisions[symbol] = &TradingDecision{
				Symbol:     symbol,
				Action:     executors.ActionHold,
				Confidence: 0.5,
				Reason:     "未在决策中明确提及，默认观望",
				Valid:      true,
			}
		}
	}

	return decisions
}

// ValidateLeverage validates and returns the appropriate leverage to use
// ValidateLeverage 验证并返回应使用的杠杆倍数
func ValidateLeverage(llmLeverage int, minLeverage int, maxLeverage int, dynamic bool) int {
	// If dynamic leverage is disabled, use min leverage (which equals max in fixed mode)
	// 如果未启用动态杠杆，使用最小杠杆（固定模式下最小值等于最大值）
	if !dynamic {
		return minLeverage
	}

	// If LLM didn't specify leverage, use minimum for safety
	// 如果 LLM 未指定杠杆，为安全起见使用最小值
	if llmLeverage == 0 {
		return minLeverage
	}

	// Validate LLM leverage is within range
	// 验证 LLM 杠杆在范围内
	if llmLeverage < minLeverage {
		return minLeverage
	}
	if llmLeverage > maxLeverage {
		return maxLeverage
	}

	// Use LLM's choice
	// 使用 LLM 的选择
	return llmLeverage
}
