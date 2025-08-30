package performance

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// PerformanceOptimizedApp 演示如何将所有性能优化组件一起使用
type PerformanceOptimizedApp struct {
	monitor      *PerformanceMonitor
	workerPool   *WorkerPool
	rateLimiter  *RateLimiter
	memoryPool   *MemoryPool
	cache        *Cache
	httpPool     *HTTPClientPool
	requestCache *RequestCache
	netMonitor   *NetworkMonitor
	gcOptimizer  *GCOptimizer
	running      bool
	mu           sync.RWMutex
}

// NewPerformanceOptimizedApp 创建一个新的性能优化应用程序
func NewPerformanceOptimizedApp() *PerformanceOptimizedApp {
	// 配置性能监控器
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.UpdateInterval = 5 * time.Second
	monitorConfig.EnableMetrics = true
	monitorConfig.MemoryThreshold = 500 * 1024 * 1024 // 500MB
	monitorConfig.GoroutineThreshold = 1000

	// 配置工作池
	workerConfig := DefaultWorkerPoolConfig()
	workerConfig.WorkerCount = runtime.NumCPU()
	workerConfig.MaxWorkers = runtime.NumCPU() * 2
	workerConfig.QueueSize = 1000
	workerConfig.EnableAutoScaling = true
	workerConfig.ScaleUpThreshold = 80
	workerConfig.ScaleDownThreshold = 20

	// 配置速率限制器
	rateLimiterConfig := DefaultRateLimiterConfig()
	rateLimiterConfig.Rate = 100 // 100 requests per second
	rateLimiterConfig.Burst = 50

	// 配置内存池
	memoryConfig := DefaultMemoryPoolConfig()
	memoryConfig.BufferSize = 64 * 1024 // 64KB buffers
	memoryConfig.MaxBuffers = 100
	memoryConfig.EnableMetrics = true

	// 配置缓存
	cacheConfig := DefaultCacheConfig()
	cacheConfig.MaxSize = 10000
	cacheConfig.DefaultTTL = 1 * time.Hour
	cacheConfig.EvictionPolicy = "lru"
	cacheConfig.EnableMetrics = true

	// 配置HTTP客户端池
	httpConfig := DefaultHTTPClientPoolConfig()
	httpConfig.MaxIdleConns = 100
	httpConfig.MaxIdleConnsPerHost = 10
	httpConfig.IdleConnTimeout = 90 * time.Second
	httpConfig.EnableMetrics = true

	// 配置请求缓存
	requestCacheConfig := DefaultRequestCacheConfig()
	requestCacheConfig.MaxSize = 1000
	requestCacheConfig.DefaultTTL = 30 * time.Minute
	requestCacheConfig.EnableMetrics = true

	// 配置网络监控器
	networkConfig := DefaultNetworkMonitorConfig()
	networkConfig.UpdateInterval = 10 * time.Second
	networkConfig.EnableMetrics = true

	// 配置GC优化器
	gcConfig := DefaultGCOptimizerConfig()
	gcConfig.EnableAdaptive = true
	gcConfig.TargetGCPercent = 100
	gcConfig.EnableMetrics = true

	return &PerformanceOptimizedApp{
		monitor:      NewPerformanceMonitor(monitorConfig),
		workerPool:   NewWorkerPool(workerConfig),
		rateLimiter:  NewRateLimiter(rateLimiterConfig),
		memoryPool:   NewMemoryPool(memoryConfig),
		cache:        NewCache(cacheConfig),
		httpPool:     NewHTTPClientPool(httpConfig),
		requestCache: NewRequestCache(requestCacheConfig),
		netMonitor:   NewNetworkMonitor(networkConfig),
		gcOptimizer:  NewGCOptimizer(gcConfig),
	}
}

// Start 启动所有性能优化组件
func (app *PerformanceOptimizedApp) Start(ctx context.Context) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.running {
		return fmt.Errorf("application is already running")
	}

	// 启动所有组件
	if err := app.monitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start performance monitor: %w", err)
	}

	if err := app.workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	if err := app.requestCache.Start(ctx); err != nil {
		return fmt.Errorf("failed to start request cache: %w", err)
	}

	if err := app.httpPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start HTTP pool: %w", err)
	}

	if err := app.netMonitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start network monitor: %w", err)
	}

	if err := app.gcOptimizer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start GC optimizer: %w", err)
	}

	// 设置回调和监控
	app.setupCallbacks()

	app.running = true
	log.Println("Performance optimized application started successfully")

	return nil
}

// Stop 停止所有性能优化组件
func (app *PerformanceOptimizedApp) Stop() {
	app.mu.Lock()
	defer app.mu.Unlock()

	if !app.running {
		return
	}

	// 停止所有组件
	app.monitor.Stop()
	app.workerPool.Stop()
	app.requestCache.Stop()
	app.httpPool.Stop()
	app.netMonitor.Stop()
	app.gcOptimizer.Stop()

	app.running = false
	log.Println("Performance optimized application stopped")
}

// setupCallbacks 设置监控回调
func (app *PerformanceOptimizedApp) setupCallbacks() {
	// 性能监控器回调
	app.monitor.AddCallback(func(metrics *Metrics) {
		if metrics.MemoryUsage > 400*1024*1024 { // 400MB
			log.Printf("High memory usage detected: %d MB", metrics.MemoryUsage/(1024*1024))
			// 触发GC
			app.gcOptimizer.ForceGC()
		}

		if metrics.GoroutineCount > 800 {
			log.Printf("High goroutine count detected: %d", metrics.GoroutineCount)
		}
	})

	// 性能监控器警报回调
	app.monitor.AddAlertCallback(func(alert *Alert) {
		log.Printf("Performance Alert: %s - %s (Severity: %s)", alert.Type, alert.Message, alert.Severity)
		
		// 根据警报类型采取行动
		switch alert.Type {
		case "memory":
			// 强制垃圾回收
			app.gcOptimizer.ForceGC()
		case "goroutine":
			// 如果可能，缩减工作池
			app.workerPool.ScaleDown()
		case "cpu":
			// 减少工作池大小
			app.workerPool.ScaleDown()
		}
	})

	// 网络监控器回调
	app.netMonitor.AddCallback(func(metrics *NetworkMetrics) {
		if metrics.Latency > 1*time.Second {
			log.Printf("High network latency detected: %v", metrics.Latency)
		}

		if metrics.ConnectionErrors > 10 {
			log.Printf("High connection error rate: %d", metrics.ConnectionErrors)
		}
	})
}

// ProcessTask 演示优化的任务处理
func (app *PerformanceOptimizedApp) ProcessTask(ctx context.Context, taskData interface{}) (interface{}, error) {
	// 速率限制
	if !app.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// 首先检查缓存
	cacheKey := fmt.Sprintf("task-%v", taskData)
	if result, exists := app.cache.Get(cacheKey); exists {
		return result, nil
	}

	// 将任务提交到工作池
	task := func() interface{} {
		// 从内存池获取缓冲区
		buffer := app.memoryPool.Get()
		defer app.memoryPool.Put(buffer)

		// 模拟任务处理
		result := app.processTaskInternal(taskData, buffer)

		// 缓存结果
		app.cache.Set(cacheKey, result, 30*time.Minute)

		return result
	}

	return app.workerPool.SubmitWithResult(task, 30*time.Second)
}

// processTaskInternal 模拟内部任务处理
func (app *PerformanceOptimizedApp) processTaskInternal(taskData interface{}, buffer []byte) interface{} {
	// 模拟一些处理工作
	time.Sleep(10 * time.Millisecond)
	return fmt.Sprintf("processed-%v", taskData)
}

// MakeHTTPRequest 演示优化的HTTP请求处理
func (app *PerformanceOptimizedApp) MakeHTTPRequest(ctx context.Context, url string) (*http.Response, error) {
	// 速率限制
	if err := app.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 检查请求缓存
	cacheKey := app.requestCache.GenerateKey(req)
	if cachedResp, exists := app.requestCache.Get(cacheKey); exists {
		// 将缓存的响应转换回http.Response
		// 注意：这是一个简化的示例
		log.Printf("Cache hit for %s", url)
		return &http.Response{
			StatusCode: cachedResp.StatusCode,
			Header:     make(http.Header),
		}, nil
	}

	// 通过HTTP池发出请求
	resp, err := app.httpPool.DoRequest(ctx, req, "default", nil)
	if err != nil {
		return nil, err
	}

	// 缓存成功的响应
	if resp.StatusCode == http.StatusOK && app.requestCache.IsCacheable(req.Method, resp.StatusCode) {
		if cachedResp, err := CreateCachedResponse(resp, 30*time.Minute); err == nil {
			app.requestCache.Set(cacheKey, cachedResp)
		}
	}

	return resp, nil
}

// BatchProcessTasks 演示优化的批处理
func (app *PerformanceOptimizedApp) BatchProcessTasks(ctx context.Context, tasks []interface{}) ([]interface{}, error) {
	results := make([]interface{}, len(tasks))
	errorChan := make(chan error, len(tasks))
	var wg sync.WaitGroup

	// 使用信号量限制并发处理
	sem := NewSemaphore(runtime.NumCPU())

	for i, task := range tasks {
		wg.Add(1)
		go func(index int, taskData interface{}) {
			defer wg.Done()

			// 获取信号量
			if err := sem.Acquire(ctx); err != nil {
				errorChan <- err
				return
			}
			defer sem.Release()

			// 处理任务
			result, err := app.ProcessTask(ctx, taskData)
			if err != nil {
				errorChan <- err
				return
			}

			results[index] = result
		}(i, task)
	}

	wg.Wait()
	close(errorChan)

	// 检查错误
	for err := range errorChan {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// GetPerformanceStats 返回全面的性能统计信息
func (app *PerformanceOptimizedApp) GetPerformanceStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 性能监控器统计
	if metrics := app.monitor.GetMetrics(); metrics != nil {
		stats["performance"] = map[string]interface{}{
			"memory_usage":     metrics.MemoryUsage,
			"goroutine_count":  metrics.GoroutineCount,
			"cpu_usage":        metrics.CPUUsage,
			"gc_count":         metrics.GCCount,
			"gc_pause_total":   metrics.GCPauseTotal,
			"network_requests": metrics.NetworkRequests,
			"throughput":       metrics.Throughput,
		}
	}

	// 工作池统计
	if workerStats := app.workerPool.GetStats(); workerStats != nil {
		stats["worker_pool"] = map[string]interface{}{
			"active_workers":   workerStats.ActiveWorkers,
			"queued_tasks":     workerStats.QueuedTasks,
			"completed_tasks":  workerStats.CompletedTasks,
			"failed_tasks":     workerStats.FailedTasks,
			"average_duration": workerStats.AverageDuration,
		}
	}

	// 内存池统计
	if memStats := app.memoryPool.GetStats(); memStats != nil {
		stats["memory_pool"] = map[string]interface{}{
			"buffers_created": memStats.BuffersCreated,
			"buffers_reused":  memStats.BuffersReused,
			"active_buffers":  memStats.ActiveBuffers,
			"pool_hits":       memStats.PoolHits,
			"pool_misses":     memStats.PoolMisses,
		}
	}

	// 缓存统计
	if cacheStats := app.cache.GetStats(); cacheStats != nil {
		stats["cache"] = map[string]interface{}{
			"hits":         cacheStats.Hits,
			"misses":       cacheStats.Misses,
			"evictions":    cacheStats.Evictions,
			"size":         cacheStats.Size,
			"memory_usage": cacheStats.MemoryUsage,
			"hit_ratio":    cacheStats.HitRatio,
		}
	}

	// HTTP池统计
	if httpStats := app.httpPool.GetStats(); httpStats != nil {
		stats["http_pool"] = map[string]interface{}{
			"total_requests":     httpStats.TotalRequests,
			"success_requests":   httpStats.SuccessRequests,
			"failed_requests":    httpStats.FailedRequests,
			"average_latency":    httpStats.AverageLatency,
			"active_clients":     httpStats.ActiveClients,
			"connections_created": httpStats.ConnectionsCreated,
			"connections_reused":  httpStats.ConnectionsReused,
		}
	}

	// 请求缓存统计
	if reqCacheStats := app.requestCache.GetStats(); reqCacheStats != nil {
		stats["request_cache"] = map[string]interface{}{
			"hits":         reqCacheStats.Hits,
			"misses":       reqCacheStats.Misses,
			"stores":       reqCacheStats.Stores,
			"evictions":    reqCacheStats.Evictions,
			"size":         reqCacheStats.Size,
			"memory_usage": reqCacheStats.MemoryUsage,
			"hit_ratio":    reqCacheStats.HitRatio,
		}
	}

	// Network monitor stats
	if netStats := app.netMonitor.GetMetrics(); netStats != nil {
		stats["network"] = map[string]interface{}{
			"latency":            netStats.Latency,
			"throughput":         netStats.Throughput,
			"packet_loss":        netStats.PacketLoss,
			"connection_errors":  netStats.ConnectionErrors,
			"timeout_errors":     netStats.TimeoutErrors,
			"dns_resolution_time": netStats.DNSResolutionTime,
			"tcp_connect_time":   netStats.TCPConnectTime,
			"tls_handshake_time": netStats.TLSHandshakeTime,
			"first_byte_time":    netStats.FirstByteTime,
		}
	}

	// GC optimizer stats
	if gcStats := app.gcOptimizer.GetStats(); gcStats != nil {
		stats["gc_optimizer"] = map[string]interface{}{
			"gc_runs":           gcStats.GCRuns,
			"forced_gc_runs":    gcStats.ForcedGCRuns,
			"total_pause_time":  gcStats.TotalPauseTime,
			"average_pause_time": gcStats.AveragePauseTime,
			"current_gc_percent": gcStats.CurrentGCPercent,
			"memory_freed":      gcStats.MemoryFreed,
		}
	}

	return stats
}

// PrintPerformanceReport prints a detailed performance report
func (app *PerformanceOptimizedApp) PrintPerformanceReport() {
	stats := app.GetPerformanceStats()

	fmt.Println("\n=== Performance Report ===")
	fmt.Printf("Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()

	for category, data := range stats {
		fmt.Printf("[%s]\n", category)
		if categoryData, ok := data.(map[string]interface{}); ok {
			for key, value := range categoryData {
				switch v := value.(type) {
				case time.Duration:
					fmt.Printf("  %s: %v\n", key, v)
				case uint64:
					fmt.Printf("  %s: %d\n", key, v)
				case int:
					fmt.Printf("  %s: %d\n", key, v)
				case int64:
					fmt.Printf("  %s: %d\n", key, v)
				case float64:
					fmt.Printf("  %s: %.2f\n", key, v)
				default:
					fmt.Printf("  %s: %v\n", key, v)
				}
			}
		}
		fmt.Println()
	}
}

// Example usage function
func ExampleUsage() {
	// Create and start the performance-optimized application
	app := NewPerformanceOptimizedApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
	defer app.Stop()

	// Example 1: Process individual tasks
	log.Println("Processing individual tasks...")
	for i := 0; i < 10; i++ {
		result, err := app.ProcessTask(ctx, fmt.Sprintf("task-%d", i))
		if err != nil {
			log.Printf("Task %d failed: %v", i, err)
		} else {
			log.Printf("Task %d result: %v", i, result)
		}
	}

	// Example 2: Batch process tasks
	log.Println("\nBatch processing tasks...")
	tasks := make([]interface{}, 20)
	for i := range tasks {
		tasks[i] = fmt.Sprintf("batch-task-%d", i)
	}

	results, err := app.BatchProcessTasks(ctx, tasks)
	if err != nil {
		log.Printf("Batch processing failed: %v", err)
	} else {
		log.Printf("Batch processing completed: %d results", len(results))
	}

	// Example 3: Make HTTP requests
	log.Println("\nMaking HTTP requests...")
	urls := []string{
		"https://httpbin.org/delay/1",
		"https://httpbin.org/json",
		"https://httpbin.org/uuid",
	}

	for _, url := range urls {
		resp, err := app.MakeHTTPRequest(ctx, url)
		if err != nil {
			log.Printf("HTTP request to %s failed: %v", url, err)
		} else {
			log.Printf("HTTP request to %s succeeded: %d", url, resp.StatusCode)
			resp.Body.Close()
		}
	}

	// Wait a bit for metrics to be collected
	time.Sleep(2 * time.Second)

	// Print performance report
	app.PrintPerformanceReport()
}

// Demonstration of advanced performance patterns
func AdvancedPerformancePatterns() {
	log.Println("\n=== Advanced Performance Patterns ===")

	// Pattern 1: Memory-efficient data processing
	log.Println("\n1. Memory-efficient data processing")
	memoryPool := NewMemoryPool(DefaultMemoryPoolConfig())

	// Process large dataset with memory reuse
	processLargeDataset := func(data [][]byte) {
		for _, chunk := range data {
			buffer := memoryPool.Get()
			// Process chunk with reused buffer
			copy(buffer[:len(chunk)], chunk)
			// ... processing logic ...
			memoryPool.Put(buffer)
		}
	}

	// Simulate processing
	testData := make([][]byte, 100)
	for i := range testData {
		testData[i] = make([]byte, 1024)
	}
	processLargeDataset(testData)

	// Pattern 2: Adaptive rate limiting
	log.Println("\n2. Adaptive rate limiting")
	rateLimiter := NewRateLimiter(DefaultRateLimiterConfig())

	// Simulate adaptive rate limiting based on system load
	adaptiveRateLimit := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Adjust rate based on memory usage
		memoryUsageMB := m.Alloc / 1024 / 1024
		if memoryUsageMB > 100 {
			// Reduce rate when memory usage is high
			rateLimiter.UpdateRate(50) // Reduce to 50 req/sec
		} else {
			// Normal rate
			rateLimiter.UpdateRate(100) // Normal 100 req/sec
		}
	}

	adaptiveRateLimit()

	// Pattern 3: Circuit breaker with fallback
	log.Println("\n3. Circuit breaker with fallback")
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// Simulate service call with circuit breaker
	callExternalService := func() (interface{}, error) {
		return cb.Execute(func() (interface{}, error) {
			// Simulate external service call
			time.Sleep(10 * time.Millisecond)
			return "service response", nil
		})
	}

	for i := 0; i < 5; i++ {
		result, err := callExternalService()
		if err != nil {
			log.Printf("Service call %d failed: %v", i, err)
		} else {
			log.Printf("Service call %d succeeded: %v", i, result)
		}
	}

	// Pattern 4: Smart caching with TTL
	log.Println("\n4. Smart caching with TTL")
	cache := NewCache(DefaultCacheConfig())

	// Cache with different TTLs based on data type
	smartCache := func(key string, dataType string, value interface{}) {
		var ttl time.Duration
		switch dataType {
		case "user_profile":
			ttl = 1 * time.Hour
		case "session_data":
			ttl = 30 * time.Minute
		case "temporary_data":
			ttl = 5 * time.Minute
		default:
			ttl = 15 * time.Minute
		}
		cache.Set(key, value, ttl)
	}

	smartCache("user:123", "user_profile", "John Doe")
	smartCache("session:abc", "session_data", "session_info")
	smartCache("temp:xyz", "temporary_data", "temp_value")

	log.Println("Advanced performance patterns demonstration completed")
}