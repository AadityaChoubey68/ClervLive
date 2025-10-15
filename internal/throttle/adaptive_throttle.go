package throttle

import (
	"runtime"
	"sync/atomic"
	"time"
)

type Config struct {
	CPUThreshold     float64
	MemoryThreshold  float64
	SlowSubThreshold float64

	ThrottleDuration   time.Duration
	CheckInterval      time.Duration
	MinPublishInterval time.Duration
}

func DefaultConfig() Config {
	return Config{
		CPUThreshold:       0.80,
		MemoryThreshold:    0.80,
		SlowSubThreshold:   0.50,
		ThrottleDuration:   5 * time.Second,
		CheckInterval:      1 * time.Second,
		MinPublishInterval: 10 * time.Millisecond,
	}
}

type AdaptiveThrottler struct {
	config Config

	isThrottling  atomic.Bool
	lastCheckTime atomic.Int64

	slowSubCount  atomic.Int32
	totalSubCount atomic.Int32

	lastCPUUsage    atomic.Uint64
	lastMemoryUsage atomic.Uint64
}

func NewAdaptiveThrottler(config Config) *AdaptiveThrottler {
	at := &AdaptiveThrottler{
		config: config,
	}

	at.isThrottling.Store(false)
	at.lastCheckTime.Store(time.Now().Unix())

	return at
}

func (at *AdaptiveThrottler) ShouldThrottle() bool {
	if at.isThrottling.Load() {
		return true
	}

	now := time.Now()
	lastCheck := time.Unix(at.lastCheckTime.Load(), 0)

	if now.Sub(lastCheck) < at.config.CheckInterval {
		return at.isThrottling.Load()
	}

	at.lastCheckTime.Store(now.Unix())

	cpuUsage := at.getCPUUsage()
	memoryUsage := at.getMemoryUsage()

	slowSubs := float64(at.slowSubCount.Load())
	totalSubs := float64(at.totalSubCount.Load())

	if totalSubs == 0 {
		return false
	}

	slowSubPercentage := slowSubs / totalSubs

	shouldThrottle := (slowSubPercentage > at.config.SlowSubThreshold && (cpuUsage > at.config.CPUThreshold || memoryUsage > at.config.MemoryThreshold))

	if shouldThrottle {
		at.StartThrottling()
	}

	return shouldThrottle
}

func (at *AdaptiveThrottler) StartThrottling() {
	at.isThrottling.Store(true)

	go func() {
		time.Sleep(at.config.ThrottleDuration)
		at.StopThrottling()
	}()
}

func (at *AdaptiveThrottler) StopThrottling() {
	at.isThrottling.Store(false)
}

func (at *AdaptiveThrottler) ApplyThrottle() {
	if at.isThrottling.Load() {
		time.Sleep(at.config.MinPublishInterval)
	}
}

func (at *AdaptiveThrottler) UpdateSubscriber(slowCount, totalCOunt int) {
	at.slowSubCount.Store(int32(slowCount))
	at.totalSubCount.Store(int32(totalCOunt))
}

func (at *AdaptiveThrottler) getCPUUsage() float64 {
	numGoroutines := runtime.NumGoroutine()

	var usage float64
	if numGoroutines > 10000 {
		usage = 0.9
	} else if numGoroutines > 5000 {
		usage = 0.7
	} else if numGoroutines > 1000 {
		usage = 0.5
	} else {
		usage = float64(numGoroutines) / 1000.0
	}

	at.lastCPUUsage.Store(uint64(usage * 1000000))

	return usage
}

func (at *AdaptiveThrottler) getMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	maxMemory := uint64(2 * 1024 * 1024 * 1024)

	usage := float64(m.Alloc) / float64(maxMemory)

	if usage > 1.0 {
		usage = 1.0
	}

	at.lastMemoryUsage.Store(uint64(usage * 1000000))
	return usage
}

func (at *AdaptiveThrottler) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"is_throttling":     at.isThrottling.Load(),
		"slow_subscribers":  at.slowSubCount.Load(),
		"total_subscribers": at.totalSubCount.Load(),
		"cpu_usage":         float64(at.lastCPUUsage.Load()),
		"memory_usage":      float64(at.lastMemoryUsage.Load()),
	}
}

func (at *AdaptiveThrottler) IsThrottling() bool {
	return at.isThrottling.Load()
}
