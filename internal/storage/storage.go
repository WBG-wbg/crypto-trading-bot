package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// TradingSession represents a trading analysis session
type TradingSession struct {
	ID              int64
	Symbol          string
	Timeframe       string
	CreatedAt       time.Time
	MarketReport    string
	CryptoReport    string
	SentimentReport string
	PositionInfo    string
	Decision        string
	Executed        bool
	ExecutionResult string
}

// Storage handles SQLite database operations
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new storage instance
func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	storage := &Storage{db: db}

	// Initialize schema
	if err := storage.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return storage, nil
}

// initSchema creates database tables if they don't exist
func (s *Storage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS trading_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT NOT NULL,
		timeframe TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		market_report TEXT,
		crypto_report TEXT,
		sentiment_report TEXT,
		position_info TEXT,
		decision TEXT,
		executed BOOLEAN DEFAULT 0,
		execution_result TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_symbol_created_at ON trading_sessions(symbol, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_created_at ON trading_sessions(created_at DESC);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveSession saves a trading session to the database
func (s *Storage) SaveSession(session *TradingSession) (int64, error) {
	query := `
	INSERT INTO trading_sessions (
		symbol, timeframe, created_at,
		market_report, crypto_report, sentiment_report,
		position_info, decision, executed, execution_result
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(
		query,
		session.Symbol,
		session.Timeframe,
		session.CreatedAt,
		session.MarketReport,
		session.CryptoReport,
		session.SentimentReport,
		session.PositionInfo,
		session.Decision,
		session.Executed,
		session.ExecutionResult,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to save session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// GetLatestSessions retrieves the latest N sessions
func (s *Storage) GetLatestSessions(limit int) ([]*TradingSession, error) {
	query := `
	SELECT id, symbol, timeframe, created_at,
		   market_report, crypto_report, sentiment_report,
		   position_info, decision, executed, execution_result
	FROM trading_sessions
	ORDER BY created_at DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*TradingSession
	for rows.Next() {
		session := &TradingSession{}
		err := rows.Scan(
			&session.ID,
			&session.Symbol,
			&session.Timeframe,
			&session.CreatedAt,
			&session.MarketReport,
			&session.CryptoReport,
			&session.SentimentReport,
			&session.PositionInfo,
			&session.Decision,
			&session.Executed,
			&session.ExecutionResult,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// GetSessionsBySymbol retrieves sessions for a specific symbol
func (s *Storage) GetSessionsBySymbol(symbol string, limit int) ([]*TradingSession, error) {
	query := `
	SELECT id, symbol, timeframe, created_at,
		   market_report, crypto_report, sentiment_report,
		   position_info, decision, executed, execution_result
	FROM trading_sessions
	WHERE symbol = ?
	ORDER BY created_at DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*TradingSession
	for rows.Next() {
		session := &TradingSession{}
		err := rows.Scan(
			&session.ID,
			&session.Symbol,
			&session.Timeframe,
			&session.CreatedAt,
			&session.MarketReport,
			&session.CryptoReport,
			&session.SentimentReport,
			&session.PositionInfo,
			&session.Decision,
			&session.Executed,
			&session.ExecutionResult,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// GetSessionStats returns statistics about trading sessions
func (s *Storage) GetSessionStats(symbol string) (map[string]interface{}, error) {
	query := `
	SELECT
		COUNT(*) as total_sessions,
		SUM(CASE WHEN executed = 1 THEN 1 ELSE 0 END) as executed_count,
		MIN(created_at) as first_session,
		MAX(created_at) as last_session
	FROM trading_sessions
	WHERE symbol = ?
	`

	var totalSessions, executedCount int
	var firstSession, lastSession string

	err := s.db.QueryRow(query, symbol).Scan(
		&totalSessions,
		&executedCount,
		&firstSession,
		&lastSession,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats := map[string]interface{}{
		"total_sessions":  totalSessions,
		"executed_count":  executedCount,
		"first_session":   firstSession,
		"last_session":    lastSession,
		"execution_rate":  0.0,
	}

	if totalSessions > 0 {
		stats["execution_rate"] = float64(executedCount) / float64(totalSessions) * 100
	}

	return stats, nil
}

// UpdateExecutionResult updates the execution result for a session
func (s *Storage) UpdateExecutionResult(sessionID int64, executed bool, result string) error {
	query := `
	UPDATE trading_sessions
	SET executed = ?, execution_result = ?
	WHERE id = ?
	`

	_, err := s.db.Exec(query, executed, result, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update execution result: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
