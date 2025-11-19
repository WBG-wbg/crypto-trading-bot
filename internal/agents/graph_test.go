package agents

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
)

// roundTripperFunc is a helper type to implement http.RoundTripper
// roundTripperFunc 是一个帮助类型，用于实现 http.RoundTripper 接口
type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface
// RoundTrip 实现 RoundTripper 接口
func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestMakeLLMDecision_FallbackToSimpleDecision verifies that when LLM call fails,
// makeLLMDecision falls back to makeSimpleDecision.
// TestMakeLLMDecision_FallbackToSimpleDecision 验证当 LLM 调用失败时，
// makeLLMDecision 会回退到 makeSimpleDecision。
func TestMakeLLMDecision_FallbackToSimpleDecision(t *testing.T) {
	// Stub global default transport to avoid real network calls
	// 替换全局默认 Transport，避免真实网络调用
	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("stub error")),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	// Prepare minimal config and logger
	// 构造最小化配置和日志器
	cfg := &config.Config{
		APIKey:        "test-key",
		BackendURL:    "https://api.openai.com/v1",
		QuickThinkLLM: "gpt-4.1-mini",

		CryptoSymbols:   []string{"BTC/USDT"},
		CryptoTimeframe: "1h",
		TradingInterval: "1h",
	}
	log := logger.NewColorLogger(false)

	graph := &SimpleTradingGraph{
		config: cfg,
		logger: log,
		state:  NewAgentState(cfg.CryptoSymbols, cfg.CryptoTimeframe),
	}

	// Expected fallback decision from simple rule-based logic
	// 期望结果为基于规则的简单决策
	expected := graph.makeSimpleDecision()

	decision, err := graph.makeLLMDecision(context.Background())
	if err != nil {
		t.Fatalf("makeLLMDecision returned unexpected error: %v", err)
	}

	if decision != expected {
		t.Fatalf("expected fallback decision from makeSimpleDecision,\nwant:\n%s\n\ngot:\n%s", expected, decision)
	}
}
