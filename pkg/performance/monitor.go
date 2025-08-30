package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// PerformanceMonitor 监控系统性能指标
type PerformanceMonitor struct {
	mu                sync.RWMutex
	metrics          *Metrics
	config           *MonitorConfig
	running          int32
	stopCh           chan struct{}
	callbacks        []MetricsCallback
	lastGCStats      runtime.MemStats
	lastUpdateTime   time.Time
	processStartTime time.Time
}

// MonitorConfig 性能监控的配置
type MonitorConfig struct {
	UpdateInterval    time.Duration `json:"update_interval"`
	MemoryThreshold   uint64        `json:"memory_threshold"`
	GoroutineLimit    int           `json:"goroutine_limit"`
	CPUThreshold      float64       `json:"cpu_threshold"`
	EnableGCMonitor   bool          `json:"enable_gc_monitor"`
	EnableNetMonitor  bool          `json:"enable_net_monitor"`
	HistorySize       int           `json:"history_size"`
	AlertEnabled      bool          `json:"alert_enabled"`
	LogMetrics        bool          `json:"log_metrics"`
}

// Metrics 保存性能指标
type Metrics struct {
	mu                sync.RWMutex
	MemoryUsage       uint64        `json:"memory_usage"`
	MemoryAllocated   uint64        `json:"memory_allocated"`
	MemoryReleased    uint64        `json:"memory_released"`
	GoroutineCount    int           `json:"goroutine_count"`
	CPUUsage          float64       `json:"cpu_usage"`
	GCCount           uint32        `json:"gc_count"`
	GCPauseTime       time.Duration `json:"gc_pause_time"`
	NetworkRequests   uint64        `json:"network_requests"`
	NetworkErrors     uint64        `json:"network_errors"`
	NetworkLatency    time.Duration `json:"network_latency"`
	Throughput        float64       `json:"throughput"`
	ErrorRate         float64       `json:"error_rate"`
	Uptime            time.Duration `json:"uptime"`
	LastUpdated       time.Time     `json:"last_updated"`
	History           []MetricPoint `json:"history"`
}

// MetricPoint 表示某个时间点的指标快照
type MetricPoint struct {
	Timestamp       time.Time     `json:"timestamp"`
	MemoryUsage     uint64        `json:"memory_usage"`
	GoroutineCount  int           `json:"goroutine_count"`
	CPUUsage        float64       `json:"cpu_usage"`
	NetworkLatency  time.Duration `json:"network_latency"`
	Throughput      float64       `json:"throughput"`
}

// MetricsCallback 指标回调函数类型
type MetricsCallback func(*Metrics)

// AlertLevel 表示警报严重级别
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelCritical
)

// Alert 表示性能警报
type Alert struct {
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	Metric    string     `json:"metric"`
	Value     interface{} `json:"value"`
	Threshold interface{} `json:"threshold"`
	Timestamp time.Time  `json:"timestamp"`
}

// DefaultMonitorConfig 返回默认监控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		UpdateInterval:   5 * time.Second,
		MemoryThreshold:  500 * 1024 * 1024, // 500MB
		GoroutineLimit:   1000,
		CPUThreshold:     80.0, // 80%
		EnableGCMonitor:  true,
		EnableNetMonitor: true,
		HistorySize:      100,
		AlertEnabled:     true,
		LogMetrics:       false,
	}
}

// NewPerformanceMonitor 创建新的性能监控器
func NewPerformanceMonitor(config *MonitorConfig) *PerformanceMonitor {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	return &PerformanceMonitor{
		metrics: &Metrics{
			History:     make([]MetricPoint, 0, config.HistorySize),
			LastUpdated: time.Now(),
		},
		config:           config,
		stopCh:           make(chan struct{}),
		callbacks:        make([]MetricsCallback, 0),
		processStartTime: time.Now(),
	}
}

// Start 开始性能监控
func (pm *PerformanceMonitor) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&pm.running, 0, 1) {
		return fmt.Errorf("performance monitor is already running")
	}

	pm.lastUpdateTime = time.Now()
	runtime.ReadMemStats(&pm.lastGCStats)

	go pm.monitorLoop(ctx)
	return nil
}

// Stop 停止性能监控
func (pm *PerformanceMonitor) Stop() {
	if atomic.CompareAndSwapInt32(&pm.running, 1, 0) {
		close(pm.stopCh)
	}
}

// IsRunning 返回监控器是否正在运行
func (pm *PerformanceMonitor) IsRunning() bool {
	return atomic.LoadInt32(&pm.running) == 1
}

// GetMetrics 返回当前性能指标
func (pm *PerformanceMonitor) GetMetrics() *Metrics {
	pm.metrics.mu.RLock()
	defer pm.metrics.mu.RUnlock()

	// 创建副本以避免竞态条件
	metricsCopy := *pm.metrics
	metricsCopy.History = make([]MetricPoint, len(pm.metrics.History))
	copy(metricsCopy.History, pm.metrics.History)

	return &metricsCopy
}

// AddCallback 添加指标回调函数
func (pm *PerformanceMonitor) AddCallback(callback MetricsCallback) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.callbacks = append(pm.callbacks, callback)
}

// RecordNetworkRequest 记录网络请求指标
func (pm *PerformanceMonitor) RecordNetworkRequest(latency time.Duration, success bool) {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	pm.metrics.NetworkRequests++
	if !success {
		pm.metrics.NetworkErrors++
	}

	// 更新平均延迟
	if pm.metrics.NetworkRequests == 1 {
		pm.metrics.NetworkLatency = latency
	} else {
		// 指数移动平均
		alpha := 0.1
		pm.metrics.NetworkLatency = time.Duration(float64(pm.metrics.NetworkLatency)*(1-alpha) + float64(latency)*alpha)
	}

	// 更新错误率
	pm.metrics.ErrorRate = float64(pm.metrics.NetworkErrors) / float64(pm.metrics.NetworkRequests) * 100
}

// RecordThroughput 记录吞吐量指标
func (pm *PerformanceMonitor) RecordThroughput(itemsProcessed float64, duration time.Duration) {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	throughput := itemsProcessed / duration.Seconds()
	if pm.metrics.Throughput == 0 {
		pm.metrics.Throughput = throughput
	} else {
		// 指数移动平均
		alpha := 0.2
		pm.metrics.Throughput = pm.metrics.Throughput*(1-alpha) + throughput*alpha
	}
}

// GetAlerts 返回当前性能警报
func (pm *PerformanceMonitor) GetAlerts() []Alert {
	var alerts []Alert
	metrics := pm.GetMetrics()

	if !pm.config.AlertEnabled {
		return alerts
	}

	// 内存使用警报
	if metrics.MemoryUsage > pm.config.MemoryThreshold {
		alerts = append(alerts, Alert{
			Level:     AlertLevelWarning,
			Message:   "High memory usage detected",
			Metric:    "memory_usage",
			Value:     metrics.MemoryUsage,
			Threshold: pm.config.MemoryThreshold,
			Timestamp: time.Now(),
		})
	}

	// 协程数量警报
	if metrics.GoroutineCount > pm.config.GoroutineLimit {
		alerts = append(alerts, Alert{
			Level:     AlertLevelCritical,
			Message:   "Goroutine count exceeded limit",
			Metric:    "goroutine_count",
			Value:     metrics.GoroutineCount,
			Threshold: pm.config.GoroutineLimit,
			Timestamp: time.Now(),
		})
	}

	// CPU使用率警报
	if metrics.CPUUsage > pm.config.CPUThreshold {
		alerts = append(alerts, Alert{
			Level:     AlertLevelWarning,
			Message:   "High CPU usage detected",
			Metric:    "cpu_usage",
			Value:     metrics.CPUUsage,
			Threshold: pm.config.CPUThreshold,
			Timestamp: time.Now(),
		})
	}

	// 高错误率警报
	if metrics.ErrorRate > 10.0 { // 10% 错误率阈值
		alerts = append(alerts, Alert{
			Level:     AlertLevelWarning,
			Message:   "High error rate detected",
			Metric:    "error_rate",
			Value:     metrics.ErrorRate,
			Threshold: 10.0,
			Timestamp: time.Now(),
		})
	}

	return alerts
}

// GetSummary 返回性能摘要
func (pm *PerformanceMonitor) GetSummary() map[string]interface{} {
	metrics := pm.GetMetrics()
	alerts := pm.GetAlerts()

	return map[string]interface{}{
		"uptime":            metrics.Uptime,
		"memory_usage_mb":   float64(metrics.MemoryUsage) / 1024 / 1024,
		"goroutine_count":   metrics.GoroutineCount,
		"cpu_usage_percent": metrics.CPUUsage,
		"gc_count":          metrics.GCCount,
		"network_requests":  metrics.NetworkRequests,
		"error_rate":        metrics.ErrorRate,
		"throughput":        metrics.Throughput,
		"alert_count":       len(alerts),
		"last_updated":      metrics.LastUpdated,
	}
}

// monitorLoop 运行主监控循环
func (pm *PerformanceMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(pm.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.updateMetrics()
			pm.notifyCallbacks()
		}
	}
}

// updateMetrics 更新所有性能指标
func (pm *PerformanceMonitor) updateMetrics() {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	now := time.Now()
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 更新内存指标
	pm.metrics.MemoryUsage = memStats.Alloc
	pm.metrics.MemoryAllocated = memStats.TotalAlloc
	pm.metrics.MemoryReleased = memStats.TotalAlloc - memStats.Alloc

	// 更新协程数量
	pm.metrics.GoroutineCount = runtime.NumGoroutine()

	// 更新GC指标
	pm.metrics.GCCount = memStats.NumGC
	if memStats.NumGC > pm.lastGCStats.NumGC {
		// 计算最近GC的平均暂停时间
		var totalPause time.Duration
		gcCount := memStats.NumGC - pm.lastGCStats.NumGC
		for i := uint32(0); i < gcCount && i < 256; i++ {
			idx := (memStats.NumGC - 1 - i) % 256
			totalPause += time.Duration(memStats.PauseNs[idx])
		}
		pm.metrics.GCPauseTime = totalPause / time.Duration(gcCount)
	}

	// 更新CPU使用率（简化计算）
	pm.metrics.CPUUsage = pm.calculateCPUUsage()

	// 更新运行时间
	pm.metrics.Uptime = now.Sub(pm.processStartTime)
	pm.metrics.LastUpdated = now

	// 添加到历史记录
	point := MetricPoint{
		Timestamp:      now,
		MemoryUsage:    pm.metrics.MemoryUsage,
		GoroutineCount: pm.metrics.GoroutineCount,
		CPUUsage:       pm.metrics.CPUUsage,
		NetworkLatency: pm.metrics.NetworkLatency,
		Throughput:     pm.metrics.Throughput,
	}

	pm.metrics.History = append(pm.metrics.History, point)
	if len(pm.metrics.History) > pm.config.HistorySize {
		pm.metrics.History = pm.metrics.History[1:]
	}

	pm.lastGCStats = memStats
	pm.lastUpdateTime = now
}

// calculateCPUUsage 计算CPU使用率百分比（简化版）
func (pm *PerformanceMonitor) calculateCPUUsage() float64 {
	// 这是一个简化的CPU使用率计算
	// 在实际实现中，您会使用更复杂的方法
	// 例如在Linux上读取/proc/stat或使用系统API
	
	// 目前，我们将使用基于协程数量的简单近似值
	goroutineRatio := float64(pm.metrics.GoroutineCount) / float64(runtime.NumCPU()) / 10.0
	if goroutineRatio > 100.0 {
		goroutineRatio = 100.0
	}
	
	return goroutineRatio
}

// notifyCallbacks 通知所有已注册的回调
func (pm *PerformanceMonitor) notifyCallbacks() {
	pm.mu.RLock()
	callbacks := make([]MetricsCallback, len(pm.callbacks))
	copy(callbacks, pm.callbacks)
	pm.mu.RUnlock()

	metrics := pm.GetMetrics()
	for _, callback := range callbacks {
		go func(cb MetricsCallback) {
			defer func() {
				if r := recover(); r != nil {
					// 记录回调中的panic但不崩溃监控器
					fmt.Printf("Panic in metrics callback: %v\n", r)
				}
			}()
			cb(metrics)
		}(callback)
	}
}

// ForceGC 强制垃圾回收并更新指标
func (pm *PerformanceMonitor) ForceGC() {
	runtime.GC()
	pm.updateMetrics()
}

// GetMemoryProfile 返回详细的内存分析信息
func (pm *PerformanceMonitor) GetMemoryProfile() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"alloc_mb":        float64(memStats.Alloc) / 1024 / 1024,
		"total_alloc_mb":  float64(memStats.TotalAlloc) / 1024 / 1024,
		"sys_mb":          float64(memStats.Sys) / 1024 / 1024,
		"heap_alloc_mb":   float64(memStats.HeapAlloc) / 1024 / 1024,
		"heap_sys_mb":     float64(memStats.HeapSys) / 1024 / 1024,
		"heap_idle_mb":    float64(memStats.HeapIdle) / 1024 / 1024,
		"heap_inuse_mb":   float64(memStats.HeapInuse) / 1024 / 1024,
		"heap_released_mb": float64(memStats.HeapReleased) / 1024 / 1024,
		"heap_objects":    memStats.HeapObjects,
		"stack_inuse_mb":  float64(memStats.StackInuse) / 1024 / 1024,
		"stack_sys_mb":    float64(memStats.StackSys) / 1024 / 1024,
		"gc_count":        memStats.NumGC,
		"gc_cpu_fraction": memStats.GCCPUFraction,
		"next_gc_mb":      float64(memStats.NextGC) / 1024 / 1024,
		"last_gc_time":    time.Unix(0, int64(memStats.LastGC)),
	}
}