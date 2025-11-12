package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/constant"
	"github.com/oak/crypto-trading-bot/internal/storage"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadConfig(constant.BlankStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Open database
	db, err := storage.NewStorage(cfg.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	command := os.Args[1]

	switch command {
	case "stats":
		handleStats(db, cfg)
	case "latest":
		limit := 10
		if len(os.Args) >= 3 {
			limit, _ = strconv.Atoi(os.Args[2])
		}
		handleLatest(db, limit)
	case "symbol":
		if len(os.Args) < 3 {
			fmt.Println("Usage: query symbol <SYMBOL> [limit]")
			os.Exit(1)
		}
		symbol := os.Args[2]
		limit := 10
		if len(os.Args) >= 4 {
			limit, _ = strconv.Atoi(os.Args[3])
		}
		handleSymbol(db, symbol, limit)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: query <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  stats              - Show database statistics")
	fmt.Println("  latest [N]         - Show latest N sessions (default: 10)")
	fmt.Println("  symbol <SYM> [N]   - Show latest N sessions for symbol (default: 10)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  query stats")
	fmt.Println("  query latest 5")
	fmt.Println("  query symbol BTC/USDT 10")
}

func handleStats(db *storage.Storage, cfg *config.Config) {
	// Use first symbol from config or ask user
	symbol := cfg.CryptoSymbols[0]
	if len(cfg.CryptoSymbols) > 1 {
		fmt.Printf("Multiple symbols configured: %v\n", cfg.CryptoSymbols)
		fmt.Printf("Showing stats for: %s\n\n", symbol)
	}

	stats, err := db.GetSessionStats(symbol)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get stats: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Trading Sessions Statistics ===")
	fmt.Printf("Symbol:           %s\n", symbol)
	fmt.Printf("Total Sessions:   %d\n", stats["total_sessions"].(int))
	fmt.Printf("Executed Trades:  %d\n", stats["executed_count"].(int))
	fmt.Printf("Execution Rate:   %.1f%%\n", stats["execution_rate"].(float64))

	if stats["first_session"] != nil && stats["first_session"].(string) != "" {
		fmt.Printf("First Session:    %s\n", stats["first_session"].(string))
		fmt.Printf("Last Session:     %s\n", stats["last_session"].(string))
	}
}

func handleLatest(db *storage.Storage, limit int) {
	sessions, err := db.GetLatestSessions(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found in database.")
		return
	}

	fmt.Printf("=== Latest %d Trading Sessions ===\n\n", len(sessions))

	for i, session := range sessions {
		fmt.Printf("[%d] Session ID: %d\n", i+1, session.ID)
		fmt.Printf("    Symbol:      %s\n", session.Symbol)
		fmt.Printf("    Timeframe:   %s\n", session.Timeframe)
		fmt.Printf("    Created:     %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("    Executed:    %v\n", session.Executed)

		// Show decision preview (first 100 chars)
		if len(session.Decision) > 0 {
			preview := session.Decision
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("    Decision:    %s\n", preview)
		}

		if session.Executed && session.ExecutionResult != "" {
			fmt.Printf("    Result:      %s\n", session.ExecutionResult)
		}
		fmt.Println()
	}
}

func handleSymbol(db *storage.Storage, symbol string, limit int) {
	sessions, err := db.GetSessionsBySymbol(symbol, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Printf("No sessions found for symbol: %s\n", symbol)
		return
	}

	fmt.Printf("=== Latest %d Sessions for %s ===\n\n", len(sessions), symbol)

	for i, session := range sessions {
		fmt.Printf("[%d] Session ID: %d\n", i+1, session.ID)
		fmt.Printf("    Timeframe:   %s\n", session.Timeframe)
		fmt.Printf("    Created:     %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("    Executed:    %v\n", session.Executed)

		// Show decision preview
		if len(session.Decision) > 0 {
			preview := session.Decision
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("    Decision:    %s\n", preview)
		}

		if session.Executed && session.ExecutionResult != "" {
			fmt.Printf("    Result:      %s\n", session.ExecutionResult)
		}
		fmt.Println()
	}
}
