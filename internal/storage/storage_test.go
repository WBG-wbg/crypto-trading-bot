package storage

import (
	"os"
	"testing"
	"time"
)

func TestNewStorage(t *testing.T) {
	// 使用临时数据库文件
	tmpDB := "./test_trading.db"
	defer os.Remove(tmpDB)

	db, err := NewStorage(tmpDB)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("Storage instance should not be nil")
	}
}

func TestSaveAndGetSession(t *testing.T) {
	tmpDB := "./test_trading_sessions.db"
	defer os.Remove(tmpDB)

	db, err := NewStorage(tmpDB)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer db.Close()

	// 创建测试会话
	session := &TradingSession{
		Symbol:          "BTC/USDT",
		Timeframe:       "1h",
		CreatedAt:       time.Now(),
		MarketReport:    "Market is bullish",
		CryptoReport:    "Funding rate is positive",
		SentimentReport: "Sentiment is neutral",
		PositionInfo:    "No position",
		Decision:        "BUY at 50000",
		Executed:        false,
	}

	// 保存会话
	id, err := db.SaveSession(session)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("Session ID should be positive, got: %d", id)
	}

	// 获取最新会话
	sessions, err := db.GetLatestSessions(1)
	if err != nil {
		t.Fatalf("GetLatestSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got: %d", len(sessions))
	}

	retrieved := sessions[0]
	if retrieved.Symbol != session.Symbol {
		t.Errorf("Symbol mismatch: expected %s, got %s", session.Symbol, retrieved.Symbol)
	}
	if retrieved.Timeframe != session.Timeframe {
		t.Errorf("Timeframe mismatch: expected %s, got %s", session.Timeframe, retrieved.Timeframe)
	}
	if retrieved.Decision != session.Decision {
		t.Errorf("Decision mismatch: expected %s, got %s", session.Decision, retrieved.Decision)
	}
}

func TestGetSessionsBySymbol(t *testing.T) {
	tmpDB := "./test_trading_symbol.db"
	defer os.Remove(tmpDB)

	db, err := NewStorage(tmpDB)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer db.Close()

	// 保存多个不同交易对的会话
	symbols := []string{"BTC/USDT", "ETH/USDT", "BTC/USDT"}
	for _, symbol := range symbols {
		session := &TradingSession{
			Symbol:    symbol,
			Timeframe: "1h",
			CreatedAt: time.Now(),
			Decision:  "HOLD",
		}
		_, err := db.SaveSession(session)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}
	}

	// 获取 BTC/USDT 的会话
	sessions, err := db.GetSessionsBySymbol("BTC/USDT", 10)
	if err != nil {
		t.Fatalf("GetSessionsBySymbol failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("Expected 2 BTC/USDT sessions, got: %d", len(sessions))
	}

	// 验证所有返回的会话都是 BTC/USDT
	for _, s := range sessions {
		if s.Symbol != "BTC/USDT" {
			t.Errorf("Expected symbol BTC/USDT, got: %s", s.Symbol)
		}
	}
}

func TestGetSessionStats(t *testing.T) {
	tmpDB := "./test_trading_stats.db"
	defer os.Remove(tmpDB)

	db, err := NewStorage(tmpDB)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer db.Close()

	// 保存一些会话，部分已执行
	sessions := []*TradingSession{
		{Symbol: "BTC/USDT", Timeframe: "1h", CreatedAt: time.Now(), Decision: "BUY", Executed: true},
		{Symbol: "BTC/USDT", Timeframe: "1h", CreatedAt: time.Now(), Decision: "HOLD", Executed: false},
		{Symbol: "BTC/USDT", Timeframe: "1h", CreatedAt: time.Now(), Decision: "SELL", Executed: true},
		{Symbol: "ETH/USDT", Timeframe: "1h", CreatedAt: time.Now(), Decision: "BUY", Executed: false},
	}

	for _, s := range sessions {
		_, err := db.SaveSession(s)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}
	}

	// 获取 BTC/USDT 的统计
	stats, err := db.GetSessionStats("BTC/USDT")
	if err != nil {
		t.Fatalf("GetSessionStats failed: %v", err)
	}

	// 检查统计数据
	totalSessions, ok := stats["total_sessions"].(int)
	if !ok {
		t.Fatal("total_sessions should be int")
	}
	if totalSessions != 3 {
		t.Errorf("Expected 3 total sessions for BTC/USDT, got: %d", totalSessions)
	}

	executedCount, ok := stats["executed_count"].(int)
	if !ok {
		t.Fatal("executed_count should be int")
	}
	if executedCount != 2 {
		t.Errorf("Expected 2 executed sessions, got: %d", executedCount)
	}

	executionRate, ok := stats["execution_rate"].(float64)
	if !ok {
		t.Fatal("execution_rate should be float64")
	}
	expectedRate := 66.67
	if executionRate < expectedRate-1 || executionRate > expectedRate+1 {
		t.Errorf("Expected execution rate around %.2f%%, got: %.2f%%", expectedRate, executionRate)
	}
}

func TestUpdateExecutionResult(t *testing.T) {
	tmpDB := "./test_trading_update.db"
	defer os.Remove(tmpDB)

	db, err := NewStorage(tmpDB)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer db.Close()

	// 保存一个会话
	session := &TradingSession{
		Symbol:    "BTC/USDT",
		Timeframe: "1h",
		CreatedAt: time.Now(),
		Decision:  "BUY",
		Executed:  false,
	}

	id, err := db.SaveSession(session)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// 更新执行结果
	executionResult := "Order executed successfully at 50000"
	err = db.UpdateExecutionResult(id, true, executionResult)
	if err != nil {
		t.Fatalf("UpdateExecutionResult failed: %v", err)
	}

	// 获取更新后的会话
	sessions, err := db.GetLatestSessions(1)
	if err != nil {
		t.Fatalf("GetLatestSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got: %d", len(sessions))
	}

	updated := sessions[0]
	if !updated.Executed {
		t.Error("Session should be marked as executed")
	}
	if updated.ExecutionResult != executionResult {
		t.Errorf("ExecutionResult mismatch: expected %s, got %s",
			executionResult, updated.ExecutionResult)
	}
}
