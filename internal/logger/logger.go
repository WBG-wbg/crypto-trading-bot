package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"

	// Foreground colors
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright foreground colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"

	// Background colors
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

// ColorLogger provides colored terminal output
type ColorLogger struct {
	logger zerolog.Logger
	writer io.Writer
}

// NewColorLogger creates a new ColorLogger instance
func NewColorLogger(debug bool) *ColorLogger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	logger := zerolog.New(output).With().Timestamp().Logger()

	return &ColorLogger{
		logger: logger,
		writer: os.Stdout,
	}
}

// Header prints a header with the given text
func (l *ColorLogger) Header(text string, char rune, width int) {
	line := strings.Repeat(string(char), width)
	fmt.Fprintf(l.writer, "\n%s%s%s%s\n", Bold, BrightCyan, line, Reset)
	fmt.Fprintf(l.writer, "%s%s%s%s\n", Bold, BrightCyan, center(text, width), Reset)
	fmt.Fprintf(l.writer, "%s%s%s%s\n\n", Bold, BrightCyan, line, Reset)
}

// Subheader prints a subheader
func (l *ColorLogger) Subheader(text string, char rune, width int) {
	line := strings.Repeat(string(char), width)
	fmt.Fprintf(l.writer, "\n%s%s%s\n", BrightBlue, line, Reset)
	fmt.Fprintf(l.writer, "%s%s%s%s\n", Bold, BrightBlue, text, Reset)
	fmt.Fprintf(l.writer, "%s%s%s\n\n", BrightBlue, line, Reset)
}

// Success prints a success message
func (l *ColorLogger) Success(text string) {
	fmt.Fprintf(l.writer, "%s✅ %s%s\n", BrightGreen, text, Reset)
	l.logger.Info().Msg(text)
}

// Error prints an error message
func (l *ColorLogger) Error(text string) {
	fmt.Fprintf(l.writer, "%s❌ %s%s\n", BrightRed, text, Reset)
	l.logger.Error().Msg(text)
}

// Warning prints a warning message
func (l *ColorLogger) Warning(text string) {
	fmt.Fprintf(l.writer, "%s⚠️  %s%s\n", BrightYellow, text, Reset)
	l.logger.Warn().Msg(text)
}

// Info prints an info message
func (l *ColorLogger) Info(text string) {
	fmt.Fprintf(l.writer, "%sℹ️  %s%s\n", Cyan, text, Reset)
	l.logger.Info().Msg(text)
}

// Step prints a step message
func (l *ColorLogger) Step(stepNum int, text string) {
	fmt.Fprintf(l.writer, "%s%s🔄 [步骤 %d] %s%s\n", Bold, BrightMagenta, stepNum, text, Reset)
	l.logger.Info().Int("step", stepNum).Msg(text)
}

// ToolCall prints a tool call message
func (l *ColorLogger) ToolCall(toolName string) {
	fmt.Fprintf(l.writer, "%s🔧 调用工具: %s%s%s\n", Yellow, Bold, toolName, Reset)
	l.logger.Debug().Str("tool", toolName).Msg("Tool called")
}

// ToolResult prints a tool result
func (l *ColorLogger) ToolResult(toolName string, result string, maxLines int) {
	fmt.Fprintf(l.writer, "\n%s%s%s Tool Message: %s %s\n", Bold, BgBlue, White, toolName, Reset)
	fmt.Fprintf(l.writer, "%s%s%s\n", Green, strings.Repeat("─", 80), Reset)

	lines := strings.Split(result, "\n")
	if len(lines) > maxLines {
		fmt.Fprintln(l.writer, strings.Join(lines[:maxLines], "\n"))
		fmt.Fprintf(l.writer, "%s... (省略 %d 行)%s\n", Yellow, len(lines)-maxLines, Reset)
	} else {
		fmt.Fprintln(l.writer, result)
	}

	fmt.Fprintf(l.writer, "%s%s%s\n\n", Green, strings.Repeat("─", 80), Reset)
}

// LLMResponse prints an LLM response
func (l *ColorLogger) LLMResponse(agentName string, content string, maxLines int) {
	fmt.Fprintf(l.writer, "\n%s%s%s %s LLM 响应 %s\n", Bold, BgMagenta, White, agentName, Reset)
	fmt.Fprintf(l.writer, "%s%s%s\n", Magenta, strings.Repeat("─", 80), Reset)

	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		fmt.Fprintln(l.writer, strings.Join(lines[:maxLines], "\n"))
		fmt.Fprintf(l.writer, "%s... (省略 %d 行)%s\n", Yellow, len(lines)-maxLines, Reset)
	} else {
		fmt.Fprintln(l.writer, content)
	}

	fmt.Fprintf(l.writer, "%s%s%s\n\n", Magenta, strings.Repeat("─", 80), Reset)
}

// PositionInfo prints position information
func (l *ColorLogger) PositionInfo(info string) {
	fmt.Fprintf(l.writer, "\n%s%s%s 💼 账户和持仓信息 %s\n", Bold, BgCyan, White, Reset)
	fmt.Fprintf(l.writer, "%s%s%s\n", Cyan, strings.Repeat("─", 80), Reset)
	fmt.Fprintln(l.writer, info)
	fmt.Fprintf(l.writer, "%s%s%s\n\n", Cyan, strings.Repeat("─", 80), Reset)
}

// Decision prints the final trading decision
func (l *ColorLogger) Decision(decisionText string) {
	fmt.Fprintf(l.writer, "\n%s%s%s ✅ 最终交易决策 %s\n", Bold, BgGreen, White, Reset)
	fmt.Fprintf(l.writer, "%s%s%s\n", Green, strings.Repeat("=", 80), Reset)
	fmt.Fprintln(l.writer, decisionText)
	fmt.Fprintf(l.writer, "%s%s%s\n\n", Green, strings.Repeat("=", 80), Reset)
}

// Timestamp returns a formatted timestamp
func (l *ColorLogger) Timestamp() string {
	return fmt.Sprintf("%s[%s]%s", Cyan, time.Now().Format("15:04:05"), Reset)
}

// Debug prints a debug message (only if debug mode is enabled)
func (l *ColorLogger) Debug(text string) {
	l.logger.Debug().Msg(text)
}

// Helper function to center text
func center(text string, width int) string {
	if len(text) >= width {
		return text
	}
	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text
}

// Global logger instance
var Global *ColorLogger

// Init initializes the global logger
func Init(debug bool) {
	Global = NewColorLogger(debug)
}