package dataflows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	cryptoOracleAPIURL = "https://service.cryptoracle.network/openapi/v2/endpoint"
	cryptoOracleAPIKey = "7ad48a56-8730-4238-a714-eebc30834e3e"
)

// SentimentData holds market sentiment information
type SentimentData struct {
	Success            bool
	PositiveRatio      float64
	NegativeRatio      float64
	NetSentiment       float64
	SentimentLevel     string
	DataTime           string
	DataDelayMinutes   int
	Symbol             string
	Error              string
}

// CryptoOracleRequest represents the API request structure
type CryptoOracleRequest struct {
	APIKey    string   `json:"apiKey"`
	Endpoints []string `json:"endpoints"`
	StartTime string   `json:"startTime"`
	EndTime   string   `json:"endTime"`
	TimeType  string   `json:"timeType"`
	Token     []string `json:"token"`
}

// CryptoOracleResponse represents the API response structure
type CryptoOracleResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Data    []struct {
		TimePeriods []struct {
			StartTime string `json:"startTime"`
			EndTime   string `json:"endTime"`
			Data      []struct {
				Endpoint string `json:"endpoint"`
				Value    string `json:"value"`
			} `json:"data"`
		} `json:"timePeriods"`
	} `json:"data"`
}

// GetSentimentIndicators fetches market sentiment indicators
func GetSentimentIndicators(ctx context.Context, symbol string) *SentimentData {
	// Get time range (account for ~40 min delay)
	endTime := time.Now().Add(-40 * time.Minute)
	startTime := endTime.Add(-4 * time.Hour)

	requestBody := CryptoOracleRequest{
		APIKey:    cryptoOracleAPIKey,
		Endpoints: []string{"CO-A-02-01", "CO-A-02-02"}, // Positive/Negative sentiment
		StartTime: startTime.Format("2006-01-02 15:04:05"),
		EndTime:   endTime.Format("2006-01-02 15:04:05"),
		TimeType:  "15m",
		Token:     []string{symbol},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return &SentimentData{
			Success: false,
			Error:   fmt.Sprintf("Failed to marshal request: %v", err),
			Symbol:  symbol,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cryptoOracleAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return &SentimentData{
			Success: false,
			Error:   fmt.Sprintf("Failed to create request: %v", err),
			Symbol:  symbol,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", cryptoOracleAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &SentimentData{
			Success: false,
			Error:   fmt.Sprintf("API request failed: %v", err),
			Symbol:  symbol,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &SentimentData{
			Success: false,
			Error:   fmt.Sprintf("HTTP request failed: status_code=%d", resp.StatusCode),
			Symbol:  symbol,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &SentimentData{
			Success: false,
			Error:   fmt.Sprintf("Failed to read response: %v", err),
			Symbol:  symbol,
		}
	}

	var apiResp CryptoOracleResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return &SentimentData{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse response: %v", err),
			Symbol:  symbol,
		}
	}

	if apiResp.Code != 200 || len(apiResp.Data) == 0 {
		return &SentimentData{
			Success: false,
			Error:   fmt.Sprintf("API returned error: code=%d, msg=%s", apiResp.Code, apiResp.Message),
			Symbol:  symbol,
		}
	}

	// Find first valid data period
	for _, period := range apiResp.Data[0].TimePeriods {
		sentiment := make(map[string]float64)
		validDataFound := false

		for _, item := range period.Data {
			if strings.TrimSpace(item.Value) != "" {
				value, err := strconv.ParseFloat(item.Value, 64)
				if err == nil && (item.Endpoint == "CO-A-02-01" || item.Endpoint == "CO-A-02-02") {
					sentiment[item.Endpoint] = value
					validDataFound = true
				}
			}
		}

		// Check if we have both positive and negative sentiment
		if validDataFound {
			positive, hasPositive := sentiment["CO-A-02-01"]
			negative, hasNegative := sentiment["CO-A-02-02"]

			if hasPositive && hasNegative {
				netSentiment := positive - negative

				// Calculate data delay
				dataTime, _ := time.Parse("2006-01-02 15:04:05", period.StartTime)
				dataDelay := int(time.Since(dataTime).Minutes())

				return &SentimentData{
					Success:          true,
					PositiveRatio:    positive,
					NegativeRatio:    negative,
					NetSentiment:     netSentiment,
					SentimentLevel:   interpretSentiment(netSentiment),
					DataTime:         period.StartTime,
					DataDelayMinutes: dataDelay,
					Symbol:           symbol,
				}
			}
		}
	}

	return &SentimentData{
		Success: false,
		Error:   "所有时间段数据都为空（可能数据延迟超过预期）",
		Symbol:  symbol,
	}
}

// interpretSentiment interprets the net sentiment value
func interpretSentiment(netSentiment float64) string {
	switch {
	case netSentiment >= 0.7:
		return "极度乐观 🔥"
	case netSentiment >= 0.5:
		return "强烈乐观 📈"
	case netSentiment >= 0.3:
		return "偏向乐观 ✅"
	case netSentiment >= 0.1:
		return "轻度乐观 ↗️"
	case netSentiment >= -0.1:
		return "中性 ➖"
	case netSentiment >= -0.3:
		return "轻度悲观 ↘️"
	case netSentiment >= -0.5:
		return "偏向悲观 ❌"
	case netSentiment >= -0.7:
		return "强烈悲观 📉"
	default:
		return "极度悲观 ❄️"
	}
}

// FormatSentimentReport formats sentiment data as a readable report
func FormatSentimentReport(sentiment *SentimentData) string {
	if !sentiment.Success {
		return fmt.Sprintf(`
# 市场情绪数据获取失败

⚠️ 错误信息: %s
⚠️ 交易对: %s

说明: 本次分析无法获取市场情绪数据，建议谨慎交易。
`, sentiment.Error, sentiment.Symbol)
	}

	// Generate sentiment trend description
	var trendDesc string
	net := sentiment.NetSentiment

	switch {
	case net >= 0.5:
		trendDesc = "市场情绪极度乐观，可能存在过度买入风险，需警惕回调。"
	case net >= 0.3:
		trendDesc = "市场情绪偏向乐观，多头占据优势，适合顺势做多。"
	case net >= 0.1:
		trendDesc = "市场情绪轻度乐观，多头略占优势，可考虑轻仓做多。"
	case net >= -0.1:
		trendDesc = "市场情绪相对中性，多空分歧较大，建议观望或轻仓操作。"
	case net >= -0.3:
		trendDesc = "市场情绪轻度悲观，空头略占优势，可考虑轻仓做空。"
	case net >= -0.5:
		trendDesc = "市场情绪偏向悲观，空头占据优势，适合顺势做空。"
	default:
		trendDesc = "市场情绪极度悲观，可能存在恐慌性抛售，需警惕反弹或寻找抄底机会。"
	}

	return fmt.Sprintf(`
# 市场情绪分析报告（%s）

## 情绪指标概览
- **数据时间**: %s（延迟 %d 分钟）
- **正面情绪比率**: %.2f%%
- **负面情绪比率**: %.2f%%
- **净情绪值**: %+.4f
- **情绪等级**: %s

## 情绪解读
%s

## 交易建议参考
- **净情绪 > 0.3**: 市场偏多，可考虑做多策略
- **净情绪 < -0.3**: 市场偏空，可考虑做空策略
- **|净情绪| < 0.3**: 市场中性，建议观望或轻仓操作
- **|净情绪| > 0.6**: 极端情绪，警惕反转风险

## 数据来源
- API: CryptoOracle Sentiment Indicators
- 指标: CO-A-02-01 (正面情绪), CO-A-02-02 (负面情绪)
- 时间粒度: 15分钟
`, sentiment.Symbol, sentiment.DataTime, sentiment.DataDelayMinutes,
		sentiment.PositiveRatio*100, sentiment.NegativeRatio*100,
		sentiment.NetSentiment, sentiment.SentimentLevel, trendDesc)
}