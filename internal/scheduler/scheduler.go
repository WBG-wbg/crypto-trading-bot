package scheduler

import (
	"fmt"
	"sync"
	"time"
)

// TradingScheduler handles trading schedule based on K-line timeframe
// TradingScheduler 根据 K 线时间周期处理交易调度
type TradingScheduler struct {
	mu        sync.RWMutex // Protects timeframe and minutes / 保护 timeframe 和 minutes
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
// GetNextTimeframeTime 返回下一个 K 线周期开始时间
func (s *TradingScheduler) GetNextTimeframeTime() time.Time {
	s.mu.RLock()
	minutes := s.minutes
	s.mu.RUnlock()

	now := time.Now()

	// Calculate current minute of the day
	// 计算当天的当前分钟数
	currentMinute := now.Hour()*60 + now.Minute()

	// Calculate next period
	// 计算下一个周期
	nextPeriod := ((currentMinute / minutes) + 1) * minutes

	// Handle cross-day case
	// 处理跨天情况
	if nextPeriod >= 1440 { // 24 hours = 1440 minutes
		nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		nextPeriodMinutes := nextPeriod - 1440
		return nextDay.Add(time.Duration(nextPeriodMinutes) * time.Minute)
	}

	// Same day
	// 同一天
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return today.Add(time.Duration(nextPeriod) * time.Minute)
}

// WaitForNextTimeframe waits until the next K-line period starts
// WaitForNextTimeframe 等待直到下一个 K 线周期开始
func (s *TradingScheduler) WaitForNextTimeframe(verbose bool) {
	nextTime := s.GetNextTimeframeTime()
	now := time.Now()
	waitDuration := nextTime.Sub(now)

	if verbose {
		s.mu.RLock()
		timeframe := s.timeframe
		s.mu.RUnlock()

		fmt.Printf("⏰ 当前时间: %s\n", now.Format("2006-01-02 15:04:05"))
		fmt.Printf("⏳ 下一个 %s K线周期: %s\n", timeframe, nextTime.Format("2006-01-02 15:04:05"))
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
// IsOnTimeframe 检查当前时间是否在 K 线周期边界上
func (s *TradingScheduler) IsOnTimeframe() bool {
	s.mu.RLock()
	minutes := s.minutes
	s.mu.RUnlock()

	now := time.Now()
	currentMinute := now.Hour()*60 + now.Minute()

	// Check if on period boundary (allow 60 second tolerance)
	// 检查是否在周期边界（允许 60 秒容差）
	return currentMinute%minutes == 0 && now.Second() < 60
}

// GetAlignedIntervals returns all aligned time points in a day
// GetAlignedIntervals 返回一天内所有对齐的时间点
func (s *TradingScheduler) GetAlignedIntervals() []string {
	s.mu.RLock()
	minutes := s.minutes
	s.mu.RUnlock()

	intervals := []string{}
	totalMinutes := 0

	for totalMinutes < 1440 { // 24 hours
		hour := totalMinutes / 60
		minute := totalMinutes % 60
		intervals = append(intervals, fmt.Sprintf("%02d:%02d", hour, minute))
		totalMinutes += minutes
	}

	return intervals
}

// GetTimeframe returns the timeframe string
// GetTimeframe 返回时间周期字符串
func (s *TradingScheduler) GetTimeframe() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.timeframe
}

// GetMinutes returns the timeframe in minutes
// GetMinutes 返回时间周期的分钟数
func (s *TradingScheduler) GetMinutes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.minutes
}

// UpdateTimeframe updates the trading timeframe dynamically (hot reload)
// UpdateTimeframe 动态更新交易时间周期（热更新）
func (s *TradingScheduler) UpdateTimeframe(newTimeframe string) error {
	// Validate timeframe
	// 验证时间周期
	minutes, ok := timeframeMinutes[newTimeframe]
	if !ok {
		return fmt.Errorf("unsupported timeframe: %s", newTimeframe)
	}

	// Update with write lock
	// 使用写锁更新
	s.mu.Lock()
	defer s.mu.Unlock()

	s.timeframe = newTimeframe
	s.minutes = minutes

	return nil
}
