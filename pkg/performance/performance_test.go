package performance

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// 性能监控器测试

func TestPerformanceMonitor_Start(t *testing.T) {
	monitor := NewPerformanceMonitor(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	// 测试重复启动
	err = monitor.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running monitor")
	}

	monitor.Stop()
}

func TestPerformanceMonitor_Metrics(t *testing.T) {
	config := DefaultMonitorConfig()
	config.UpdateInterval = 10 * time.Millisecond
	monitor := NewPerformanceMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// 等待至少一个采集周期完成
	waitFor(t, time.Second, func() bool {
		return monitor.GetMetrics().MemoryUsage > 0
	}, "metrics to be collected")

	metrics := monitor.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics, got nil")
	}

	if metrics.MemoryUsage == 0 {
		t.Error("Expected non-zero memory usage")
	}

	if metrics.GoroutineCount == 0 {
		t.Error("Expected non-zero goroutine count")
	}
}

func TestPerformanceMonitor_Callbacks(t *testing.T) {
	config := DefaultMonitorConfig()
	config.UpdateInterval = 10 * time.Millisecond
	monitor := NewPerformanceMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackCalled := false
	var mu sync.Mutex

	monitor.AddCallback(func(metrics *Metrics) {
		mu.Lock()
		callbackCalled = true
		mu.Unlock()
	})

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return callbackCalled
	}, "callback to be called")
}

// TestPerformanceMonitor_Alerts 验证阈值越界时能读到警报。
// 监控器不提供警报回调，警报通过 GetAlerts 轮询获取。
func TestPerformanceMonitor_Alerts(t *testing.T) {
	config := DefaultMonitorConfig()
	config.UpdateInterval = 10 * time.Millisecond
	config.MemoryThreshold = 1 // 极低阈值以必然触发警报
	monitor := NewPerformanceMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// 等待指标采集后警报出现
	waitFor(t, time.Second, func() bool {
		return len(monitor.GetAlerts()) > 0
	}, "alert to be triggered")

	var found bool
	for _, alert := range monitor.GetAlerts() {
		if alert.Metric == "memory_usage" {
			found = true
			if alert.Level != AlertLevelWarning {
				t.Errorf("Expected warning level for memory alert, got %v", alert.Level)
			}
		}
	}

	if !found {
		t.Error("Expected a memory_usage alert")
	}
}

// TestPerformanceMonitor_AlertsDisabled 验证关闭警报后不再产生警报
func TestPerformanceMonitor_AlertsDisabled(t *testing.T) {
	config := DefaultMonitorConfig()
	config.UpdateInterval = 10 * time.Millisecond
	config.MemoryThreshold = 1
	config.AlertEnabled = false
	monitor := NewPerformanceMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	waitFor(t, time.Second, func() bool {
		return monitor.GetMetrics().MemoryUsage > 0
	}, "metrics to be collected")

	if alerts := monitor.GetAlerts(); len(alerts) != 0 {
		t.Errorf("Expected no alerts when disabled, got %d", len(alerts))
	}
}

// 工作池测试

func TestWorkerPool_Basic(t *testing.T) {
	config := DefaultWorkerPoolConfig()
	config.WorkerCount = 2
	config.QueueSize = 10

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	job := Job{
		ID: "job-1",
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			return "result", nil
		},
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	select {
	case res := <-pool.GetResult():
		if res.Error != nil {
			t.Errorf("Expected no error, got: %v", res.Error)
		}
		if res.Result != "result" {
			t.Errorf("Expected 'result', got %v", res.Result)
		}
	case <-time.After(2 * time.Second):
		t.Error("Job did not complete within timeout")
	}
}

func TestWorkerPool_MultipleJobsWithResults(t *testing.T) {
	config := DefaultWorkerPoolConfig()
	config.WorkerCount = 3
	config.QueueSize = 10

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	numJobs := 5
	for i := 0; i < numJobs; i++ {
		index := i
		job := Job{
			ID: fmt.Sprintf("job-%d", index),
			Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
				return fmt.Sprintf("result-%d", index), nil
			},
		}
		if err := pool.Submit(job); err != nil {
			t.Fatalf("Failed to submit job %d: %v", index, err)
		}
	}

	// 结果顺序不保证，按 JobID 收集后比对
	got := make(map[string]interface{}, numJobs)
	deadline := time.After(3 * time.Second)
	for len(got) < numJobs {
		select {
		case res := <-pool.GetResult():
			if res.Error != nil {
				t.Errorf("Job %s failed: %v", res.JobID, res.Error)
			}
			got[res.JobID] = res.Result
		case <-deadline:
			t.Fatalf("Only received %d of %d results before timeout", len(got), numJobs)
		}
	}

	for i := 0; i < numJobs; i++ {
		key := fmt.Sprintf("job-%d", i)
		expected := fmt.Sprintf("result-%d", i)
		if got[key] != expected {
			t.Errorf("Expected %s for %s, got %v", expected, key, got[key])
		}
	}
}

func TestWorkerPool_SubmitBeforeStart(t *testing.T) {
	pool := NewWorkerPool(DefaultWorkerPoolConfig())

	job := Job{
		ID: "job-before-start",
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			return nil, nil
		},
	}

	if err := pool.Submit(job); err == nil {
		t.Error("Expected error when submitting to a pool that is not running")
	}
}

func TestWorkerPool_Stats(t *testing.T) {
	config := DefaultWorkerPoolConfig()
	config.WorkerCount = 2
	config.QueueSize = 10

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	job := Job{
		ID: "stats-job",
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			return "done", nil
		},
	}
	if err := pool.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	select {
	case <-pool.GetResult():
	case <-time.After(2 * time.Second):
		t.Fatal("Job did not complete within timeout")
	}

	stats := pool.GetStats()
	if stats.JobsSubmitted == 0 {
		t.Error("Expected at least one submitted job in stats")
	}
}

// 速率限制器测试

func TestRateLimiter_Basic(t *testing.T) {
	// 容量 5，每 100ms 补充一个令牌
	limiter := NewRateLimiter(5, 100*time.Millisecond)

	// 应该允许初始突发
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// 令牌耗尽后应被拒绝
	if limiter.Allow() {
		t.Error("Expected request to be denied after burst")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	limiter := NewRateLimiter(1, 20*time.Millisecond)

	if !limiter.Allow() {
		t.Fatal("Expected first request to be allowed")
	}
	if limiter.Allow() {
		t.Fatal("Expected second request to be denied")
	}

	// 等待补充后应重新放行
	waitFor(t, time.Second, limiter.Allow, "rate limiter to refill")
}

func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(1, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 第一个请求应立即通过
	start := time.Now()
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("First request should be immediate, took %v", elapsed)
	}

	// 第二个请求需等待补充
	start = time.Now()
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Errorf("Second request should wait for refill, took %v", elapsed)
	}
}

func TestRateLimiter_WaitCancelled(t *testing.T) {
	limiter := NewRateLimiter(1, 10*time.Second)

	if !limiter.Allow() {
		t.Fatal("Expected first request to be allowed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	if err := limiter.Wait(ctx); err == nil {
		t.Error("Expected context error when waiting past deadline")
	}
}

// 信号量测试

func TestSemaphore_Basic(t *testing.T) {
	sem := NewSemaphore(2)

	if !sem.TryAcquire() {
		t.Error("Expected to acquire semaphore")
	}

	if !sem.TryAcquire() {
		t.Error("Expected to acquire semaphore")
	}

	// 达到容量上限后应失败
	if sem.TryAcquire() {
		t.Error("Expected to fail acquiring semaphore")
	}

	if avail := sem.Available(); avail != 0 {
		t.Errorf("Expected 0 available permits, got %d", avail)
	}

	// 释放后可再次获取
	sem.Release()
	if !sem.TryAcquire() {
		t.Error("Expected to acquire semaphore after release")
	}
}

func TestSemaphore_Acquire(t *testing.T) {
	sem := NewSemaphore(1)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := sem.Acquire(ctx); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 容量已满，第二次获取应超时
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	if err := sem.Acquire(ctx2); err == nil {
		t.Error("Expected timeout error")
	}
}

// 熔断器测试

func TestCircuitBreaker(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
		SuccessThreshold: 1,
		Timeout:          time.Second,
	}
	cb := NewCircuitBreaker("test", config)
	ctx := context.Background()

	if state := cb.GetState(); state != CircuitClosed {
		t.Errorf("Expected initial state to be closed, got %v", state)
	}

	// 成功执行
	if _, err := cb.Execute(ctx, func() (interface{}, error) {
		return "ok", nil
	}); err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// 连续失败以打开电路
	for i := uint64(0); i < config.FailureThreshold; i++ {
		_, _ = cb.Execute(ctx, func() (interface{}, error) {
			return nil, fmt.Errorf("failure")
		})
	}

	if state := cb.GetState(); state != CircuitOpen {
		t.Errorf("Expected state to be open after failures, got %v", state)
	}

	// 开路状态应直接拒绝
	if _, err := cb.Execute(ctx, func() (interface{}, error) {
		return "ok", nil
	}); err == nil {
		t.Error("Expected circuit breaker to prevent execution when open")
	}

	// 等待恢复超时后成功执行应重新闭合
	waitFor(t, 2*time.Second, func() bool {
		_, err := cb.Execute(ctx, func() (interface{}, error) {
			return "ok", nil
		})
		return err == nil
	}, "circuit breaker to allow execution after recovery timeout")

	if state := cb.GetState(); state != CircuitClosed {
		t.Errorf("Expected state to be closed after successful execution, got %v", state)
	}
}

// 内存池测试

func TestMemoryPool_Basic(t *testing.T) {
	pool := NewMemoryPool(DefaultMemoryPoolConfig())

	buf := pool.Get(1024)
	if buf == nil {
		t.Fatal("Expected buffer, got nil")
	}

	if buf.Len() < 1024 {
		t.Errorf("Expected buffer of at least 1024 bytes, got %d", buf.Len())
	}

	if len(buf.Data()) < 1024 {
		t.Errorf("Expected data slice of at least 1024 bytes, got %d", len(buf.Data()))
	}

	// 归还后再次获取（应复用）
	pool.Put(buf)

	buf2 := pool.Get(1024)
	if buf2 == nil {
		t.Fatal("Expected buffer, got nil")
	}
	pool.Put(buf2)

	stats := pool.GetStats()
	if stats.Allocations == 0 && stats.Hits == 0 {
		t.Error("Expected pool activity to be recorded in stats")
	}
}

func TestMemoryPool_OversizedAllocation(t *testing.T) {
	pool := NewMemoryPool(DefaultMemoryPoolConfig())

	// 超出所有池档位的尺寸走直接分配路径
	huge := 64 * 1024 * 1024
	buf := pool.Get(huge)
	if buf == nil {
		t.Fatal("Expected buffer for oversized request, got nil")
	}
	if buf.Len() != huge {
		t.Errorf("Expected buffer of %d bytes, got %d", huge, buf.Len())
	}
	pool.Put(buf)
}

func TestMemoryPool_Concurrent(t *testing.T) {
	pool := NewMemoryPool(DefaultMemoryPoolConfig())

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get(512)
			if buf == nil {
				t.Error("Expected buffer, got nil")
				return
			}
			time.Sleep(10 * time.Millisecond)
			pool.Put(buf)
		}()
	}

	wg.Wait()

	stats := pool.GetStats()
	total := stats.Allocations + stats.Hits
	if total < uint64(numGoroutines) {
		t.Errorf("Expected at least %d pool operations, got %d", numGoroutines, total)
	}
}

// 缓存测试

func TestCache_Basic(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 10

	cache := NewCache(config)

	cache.Set("key1", "value1")
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	if _, exists = cache.Get("nonexistent"); exists {
		t.Error("Expected key to not exist")
	}

	cache.Delete("key1")
	if _, exists = cache.Get("key1"); exists {
		t.Error("Expected key to be deleted")
	}
}

func TestCache_TTL(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 10

	cache := NewCache(config)

	cache.SetWithTTL("key1", "value1", 30*time.Millisecond)

	if _, exists := cache.Get("key1"); !exists {
		t.Error("Expected key to exist immediately after set")
	}

	// 等待过期
	waitFor(t, time.Second, func() bool {
		_, exists := cache.Get("key1")
		return !exists
	}, "cache entry to expire")
}

func TestCache_Clear(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 10

	cache := NewCache(config)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Clear()

	if _, exists := cache.Get("key1"); exists {
		t.Error("Expected key1 to be cleared")
	}
	if _, exists := cache.Get("key2"); exists {
		t.Error("Expected key2 to be cleared")
	}
}

// HTTP客户端池测试

func TestHTTPClientPool_Basic(t *testing.T) {
	server := createTestServer("test response", 0)
	defer server.Close()

	pool := NewHTTPClientPool(DefaultHTTPClientPoolConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start HTTP client pool: %v", err)
	}
	defer pool.Stop()

	client := pool.GetClient("test", nil)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHTTPClientPool_DoRequest(t *testing.T) {
	server := createTestServer("test response", 0)
	defer server.Close()

	pool := NewHTTPClientPool(DefaultHTTPClientPoolConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start HTTP client pool: %v", err)
	}
	defer pool.Stop()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := pool.DoRequest(ctx, req, "test", nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	stats := pool.GetStats()
	if stats.TotalRequests == 0 {
		t.Error("Expected at least one request in stats")
	}
}

// 请求缓存测试

func TestRequestCache_Basic(t *testing.T) {
	cache := NewRequestCache(DefaultRequestCacheConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start request cache: %v", err)
	}
	defer cache.Stop()

	if _, exists := cache.Get("test-key"); exists {
		t.Error("Expected cache miss")
	}

	response := &CachedResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte("test response"),
		Expiry:     time.Now().Add(1 * time.Hour),
		Created:    time.Now(),
		Size:       13,
	}

	cache.Set("test-key", response)

	cachedResp, exists := cache.Get("test-key")
	if !exists {
		t.Fatal("Expected cache hit")
	}

	if cachedResp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", cachedResp.StatusCode)
	}

	if string(cachedResp.Body) != "test response" {
		t.Errorf("Expected 'test response', got %s", string(cachedResp.Body))
	}
}

func TestRequestCache_Expiry(t *testing.T) {
	cache := NewRequestCache(DefaultRequestCacheConfig())

	response := &CachedResponse{
		StatusCode: 200,
		Headers:    map[string]string{},
		Body:       []byte("test"),
		Expiry:     time.Now().Add(30 * time.Millisecond),
		Created:    time.Now(),
		Size:       4,
	}

	cache.Set("test-key", response)

	if _, exists := cache.Get("test-key"); !exists {
		t.Error("Expected cache hit immediately after set")
	}

	waitFor(t, time.Second, func() bool {
		_, exists := cache.Get("test-key")
		return !exists
	}, "cached response to expire")
}

// 网络监控器测试

func TestNetworkMonitor_Basic(t *testing.T) {
	config := DefaultNetworkMonitorConfig()
	config.UpdateInterval = 10 * time.Millisecond
	monitor := NewNetworkMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start network monitor: %v", err)
	}
	defer monitor.Stop()

	monitor.RecordLatency(50 * time.Millisecond)
	monitor.RecordLatency(100 * time.Millisecond)

	metrics := monitor.GetMetrics()
	if len(metrics.LatencyHistory) == 0 {
		t.Error("Expected latency history to be recorded")
	}

	if metrics.Latency == 0 {
		t.Error("Expected non-zero latency")
	}
}

func TestNetworkMonitor_Callbacks(t *testing.T) {
	config := DefaultNetworkMonitorConfig()
	config.UpdateInterval = 10 * time.Millisecond
	monitor := NewNetworkMonitor(config)

	callbackCalled := false
	var mu sync.Mutex

	monitor.AddCallback(func(metrics *NetworkMetrics) {
		mu.Lock()
		callbackCalled = true
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start network monitor: %v", err)
	}
	defer monitor.Stop()

	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return callbackCalled
	}, "network callback to be called")
}

// 基准测试

func BenchmarkWorkerPool_Submit(b *testing.B) {
	config := DefaultWorkerPoolConfig()
	config.WorkerCount = 4
	config.QueueSize = 10000
	config.ResultSize = 10000

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		b.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	// 持续排空结果队列，避免队列写满反压影响提交耗时
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-pool.GetResult():
			case <-done:
				return
			}
		}
	}()
	defer close(done)

	handler := func(ctx context.Context, data interface{}) (interface{}, error) {
		return "done", nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Submit(Job{Handler: handler})
		}
	})
}

func BenchmarkMemoryPool_GetPut(b *testing.B) {
	pool := NewMemoryPool(DefaultMemoryPoolConfig())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get(1024)
			pool.Put(buf)
		}
	})
}

func BenchmarkCache_SetGet(b *testing.B) {
	config := DefaultCacheConfig()
	config.MaxSize = 10000

	cache := NewCache(config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%1000)
			value := fmt.Sprintf("value-%d", i)
			cache.Set(key, value)
			cache.Get(key)
			i++
		}
	})
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := NewRateLimiter(1000, time.Microsecond)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// 测试辅助函数

// waitFor 轮询 cond 直到其为真或超时，替代固定 sleep 以避免慢机器上的偶发失败
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, description string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("Timed out after %v waiting for %s", timeout, description)
}

func createTestServer(response string, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
}
