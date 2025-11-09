package scheduler

import (
	"fmt"
	"time"
)

// TradingScheduler handles trading schedule based on K-line timeframe
type TradingScheduler struct {
	timeframe string
	minutes   int
}

// Timeframe minute mappings
var timeframeMinutes = map[string]int{
	"1m":  1,
	"3m":  3,
	"5m":  5,
	"15m": 15,
	"30m": 30,
	"1h":  60,
	"2h":  120,
	"4h":  240,
	"6h":  360,
	"12h": 720,
	"1d":  1440,
}

// NewTradingScheduler creates a new trading scheduler
func NewTradingScheduler(timeframe string) (*TradingScheduler, error) {
	minutes, ok := timeframeMinutes[timeframe]
	if !ok {
		return nil, fmt.Errorf("unsupported timeframe: %s", timeframe)
	}

	return &TradingScheduler{
		timeframe: timeframe,
		minutes:   minutes,
	}, nil
}

// GetNextTimeframeTime returns the next K-line period start time
func (s *TradingScheduler) GetNextTimeframeTime() time.Time {
	now := time.Now()

	// Calculate current minute of the day
	currentMinute := now.Hour()*60 + now.Minute()

	// Calculate next period
	nextPeriod := ((currentMinute / s.minutes) + 1) * s.minutes

	// Handle cross-day case
	if nextPeriod >= 1440 { // 24 hours = 1440 minutes
		nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		nextPeriodMinutes := nextPeriod - 1440
		return nextDay.Add(time.Duration(nextPeriodMinutes) * time.Minute)
	}

	// Same day
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return today.Add(time.Duration(nextPeriod) * time.Minute)
}

// WaitForNextTimeframe waits until the next K-line period starts
func (s *TradingScheduler) WaitForNextTimeframe(verbose bool) {
	nextTime := s.GetNextTimeframeTime()
	now := time.Now()
	waitDuration := nextTime.Sub(now)

	if verbose {
		fmt.Printf("⏰ 当前时间: %s\n", now.Format("2006-01-02 15:04:05"))
		fmt.Printf("⏳ 下一个 %s K线周期: %s\n", s.timeframe, nextTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("⌛ 需要等待: %d 分 %d 秒\n\n", int(waitDuration.Minutes()), int(waitDuration.Seconds())%60)
	}

	if waitDuration > 0 {
		if verbose {
			// Countdown display
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			remaining := waitDuration
			for remaining > 0 {
				select {
				case <-ticker.C:
					mins := int(remaining.Minutes())
					secs := int(remaining.Seconds()) % 60
					fmt.Printf("\r⏳ 倒计时: %02d:%02d ", mins, secs)
					remaining -= time.Second
				}
			}
			fmt.Println()
		} else {
			time.Sleep(waitDuration)
		}
	}
}

// IsOnTimeframe checks if current time is on a K-line period boundary
func (s *TradingScheduler) IsOnTimeframe() bool {
	now := time.Now()
	currentMinute := now.Hour()*60 + now.Minute()

	// Check if on period boundary (allow 60 second tolerance)
	return currentMinute%s.minutes == 0 && now.Second() < 60
}

// GetAlignedIntervals returns all aligned time points in a day
func (s *TradingScheduler) GetAlignedIntervals() []string {
	intervals := []string{}
	totalMinutes := 0

	for totalMinutes < 1440 { // 24 hours
		hour := totalMinutes / 60
		minute := totalMinutes % 60
		intervals = append(intervals, fmt.Sprintf("%02d:%02d", hour, minute))
		totalMinutes += s.minutes
	}

	return intervals
}

// GetTimeframe returns the timeframe string
func (s *TradingScheduler) GetTimeframe() string {
	return s.timeframe
}

// GetMinutes returns the timeframe in minutes
func (s *TradingScheduler) GetMinutes() int {
	return s.minutes
}
