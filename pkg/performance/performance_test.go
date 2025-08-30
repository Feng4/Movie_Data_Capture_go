package performance

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
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
	monitor := NewPerformanceMonitor(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// 等待收集一些指标
	time.Sleep(100 * time.Millisecond)

	metrics := monitor.GetMetrics()
	if metrics == nil {
		t.Error("Expected metrics, got nil")
	}

	if metrics.MemoryUsage == 0 {
		t.Error("Expected non-zero memory usage")
	}

	if metrics.GoroutineCount == 0 {
		t.Error("Expected non-zero goroutine count")
	}
}

func TestPerformanceMonitor_Callbacks(t *testing.T) {
	monitor := NewPerformanceMonitor(nil)
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

	// 等待回调被调用
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !callbackCalled {
		t.Error("Expected callback to be called")
	}
	mu.Unlock()
}

func TestPerformanceMonitor_Alerts(t *testing.T) {
	config := DefaultMonitorConfig()
	config.MemoryThreshold = 1 // 非常低的阈值以触发警报
	monitor := NewPerformanceMonitor(config)

	alertTriggered := false
	var mu sync.Mutex

	monitor.AddAlertCallback(func(alert *Alert) {
		mu.Lock()
		alertTriggered = true
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// 等待警报被触发
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !alertTriggered {
		t.Error("Expected alert to be triggered")
	}
	mu.Unlock()
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

	// 提交一个任务
	taskDone := make(chan bool)
	task := func() interface{} {
		taskDone <- true
		return "result"
	}

	err = pool.Submit(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// 等待任务完成
	select {
	case <-taskDone:
		// 任务成功完成
	case <-time.After(1 * time.Second):
		t.Error("Task did not complete within timeout")
	}
}

func TestWorkerPool_MultipleTasksWithResults(t *testing.T) {
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

	numTasks := 5
	results := make([]interface{}, numTasks)
	var wg sync.WaitGroup

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			task := func() interface{} {
				return fmt.Sprintf("result-%d", index)
			}

			result, err := pool.SubmitWithResult(task, 1*time.Second)
			if err != nil {
				t.Errorf("Failed to submit task %d: %v", index, err)
				return
			}
			results[index] = result
		}(i)
	}

	wg.Wait()

	// 验证所有结果
	for i, result := range results {
		expected := fmt.Sprintf("result-%d", i)
		if result != expected {
			t.Errorf("Expected result %s, got %v", expected, result)
		}
	}
}

func TestWorkerPool_AutoScaling(t *testing.T) {
	config := DefaultWorkerPoolConfig()
	config.WorkerCount = 1
	config.MaxWorkers = 3
	config.EnableAutoScaling = true
	config.ScaleUpThreshold = 1
	config.ScaleDownThreshold = 0

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop()

	// 提交多个任务以触发扩容
	for i := 0; i < 5; i++ {
		task := func() interface{} {
			time.Sleep(100 * time.Millisecond)
			return "done"
		}
		pool.Submit(task)
	}

	// 等待自动扩容发生
	time.Sleep(200 * time.Millisecond)

	stats := pool.GetStats()
	if stats.ActiveWorkers <= 1 {
		t.Error("Expected worker pool to scale up")
	}
}

// 速率限制器测试

func TestRateLimiter_Basic(t *testing.T) {
	config := DefaultRateLimiterConfig()
	config.Rate = 10 // 每秒10个请求
	config.Burst = 5

	limiter := NewRateLimiter(config)

	// 应该允许初始突发
	for i := 0; i < config.Burst; i++ {
		if !limiter.Allow() {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// 应该拒绝下一个请求（突发已耗尽）
	if limiter.Allow() {
		t.Error("Expected request to be denied after burst")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	config := DefaultRateLimiterConfig()
	config.Rate = 100 // 高速率以加快测试
	config.Burst = 1

	limiter := NewRateLimiter(config)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 第一个请求应该是立即的
	start := time.Now()
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if time.Since(start) > 10*time.Millisecond {
		t.Error("First request should be immediate")
	}

	// 第二个请求应该等待
	start = time.Now()
	err = limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if time.Since(start) < 5*time.Millisecond {
		t.Error("Second request should wait")
	}
}

// 信号量测试

func TestSemaphore_Basic(t *testing.T) {
	sem := NewSemaphore(2)

	// 应该成功获取
	if !sem.TryAcquire() {
		t.Error("Expected to acquire semaphore")
	}

	if !sem.TryAcquire() {
		t.Error("Expected to acquire semaphore")
	}

	// 应该获取失败（达到限制）
	if sem.TryAcquire() {
		t.Error("Expected to fail acquiring semaphore")
	}

	// 释放并重试
	sem.Release()
	if !sem.TryAcquire() {
		t.Error("Expected to acquire semaphore after release")
	}
}

func TestSemaphore_Acquire(t *testing.T) {
	sem := NewSemaphore(1)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 第一次获取应该成功
	err := sem.Acquire(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 第二次获取应该超时
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	err = sem.Acquire(ctx2)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// 内存池测试

func TestMemoryPool_Basic(t *testing.T) {
	config := DefaultMemoryPoolConfig()
	config.BufferSize = 1024
	config.MaxBuffers = 10

	pool := NewMemoryPool(config)

	// 获取缓冲区
	buf := pool.Get()
	if buf == nil {
		t.Error("Expected buffer, got nil")
	}

	if len(buf) != config.BufferSize {
		t.Errorf("Expected buffer size %d, got %d", config.BufferSize, len(buf))
	}

	// 将缓冲区放回
	pool.Put(buf)

	// 再次获取缓冲区（应该重用）
	buf2 := pool.Get()
	if buf2 == nil {
		t.Error("Expected buffer, got nil")
	}

	stats := pool.GetStats()
	if stats.BuffersCreated == 0 {
		t.Error("Expected at least one buffer to be created")
	}
}

func TestMemoryPool_Concurrent(t *testing.T) {
	config := DefaultMemoryPoolConfig()
	config.BufferSize = 512
	config.MaxBuffers = 5

	pool := NewMemoryPool(config)

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get()
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
	if stats.BuffersCreated > config.MaxBuffers {
		t.Errorf("Created more buffers (%d) than max allowed (%d)", stats.BuffersCreated, config.MaxBuffers)
	}
}

// 缓存测试

func TestCache_Basic(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 10
	config.DefaultTTL = 1 * time.Hour

	cache := NewCache(config)

	// 设置和获取
	cache.Set("key1", "value1", 0)
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	// 不存在的键
	_, exists = cache.Get("nonexistent")
	if exists {
		t.Error("Expected key to not exist")
	}
}

func TestCache_TTL(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 10
	config.DefaultTTL = 50 * time.Millisecond

	cache := NewCache(config)

	// 设置短TTL
	cache.Set("key1", "value1", 50*time.Millisecond)

	// 应该立即存在
	_, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key to exist")
	}

	// 等待过期
	time.Sleep(100 * time.Millisecond)

	// 过期后应该不存在
	_, exists = cache.Get("key1")
	if exists {
		t.Error("Expected key to be expired")
	}
}

func TestCache_LRU(t *testing.T) {
	config := DefaultCacheConfig()
	config.MaxSize = 2
	config.DefaultTTL = 1 * time.Hour

	cache := NewCache(config)

	// 填充缓存
	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)

	// 访问key1使其成为最近使用
	cache.Get("key1")

	// 添加key3（应该驱逐key2）
	cache.Set("key3", "value3", 0)

	// key1应该仍然存在
	_, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to still exist")
	}

	// key2应该被驱逐
	_, exists = cache.Get("key2")
	if exists {
		t.Error("Expected key2 to be evicted")
	}

	// key3应该存在
	_, exists = cache.Get("key3")
	if !exists {
		t.Error("Expected key3 to exist")
	}
}

// HTTP客户端池测试

func TestHTTPClientPool_Basic(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	config := DefaultHTTPClientPoolConfig()
	pool := NewHTTPClientPool(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start HTTP client pool: %v", err)
	}
	defer pool.Stop()

	// 获取客户端并发起请求
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
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	config := DefaultHTTPClientPoolConfig()
	pool := NewHTTPClientPool(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start HTTP client pool: %v", err)
	}
	defer pool.Stop()

	// 创建请求
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// 通过池发起请求
	resp, err := pool.DoRequest(ctx, req, "test", nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 检查统计信息
	stats := pool.GetStats()
	if stats.TotalRequests == 0 {
		t.Error("Expected at least one request in stats")
	}
}

// 请求缓存测试

func TestRequestCache_Basic(t *testing.T) {
	config := DefaultRequestCacheConfig()
	cache := NewRequestCache(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := cache.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start request cache: %v", err)
	}
	defer cache.Stop()

	// 测试缓存未命中
	_, exists := cache.Get("test-key")
	if exists {
		t.Error("Expected cache miss")
	}

	// 设置缓存条目
	response := &CachedResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte("test response"),
		Expiry:     time.Now().Add(1 * time.Hour),
		Created:    time.Now(),
		Size:       13,
	}

	cache.Set("test-key", response)

	// 测试缓存命中
	cachedResp, exists := cache.Get("test-key")
	if !exists {
		t.Error("Expected cache hit")
	}

	if cachedResp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", cachedResp.StatusCode)
	}

	if string(cachedResp.Body) != "test response" {
		t.Errorf("Expected 'test response', got %s", string(cachedResp.Body))
	}
}

func TestRequestCache_Expiry(t *testing.T) {
	config := DefaultRequestCacheConfig()
	cache := NewRequestCache(config)

	// 设置短过期时间的缓存条目
	response := &CachedResponse{
		StatusCode: 200,
		Headers:    map[string]string{},
		Body:       []byte("test"),
		Expiry:     time.Now().Add(50 * time.Millisecond),
		Created:    time.Now(),
		Size:       4,
	}

	cache.Set("test-key", response)

	// 应该立即存在
	_, exists := cache.Get("test-key")
	if !exists {
		t.Error("Expected cache hit")
	}

	// 等待过期
	time.Sleep(100 * time.Millisecond)

	// 过期后应该不存在
	_, exists = cache.Get("test-key")
	if exists {
		t.Error("Expected cache miss after expiry")
	}
}

// 网络监控器测试

func TestNetworkMonitor_Basic(t *testing.T) {
	config := DefaultNetworkMonitorConfig()
	config.UpdateInterval = 50 * time.Millisecond
	monitor := NewNetworkMonitor(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start network monitor: %v", err)
	}
	defer monitor.Stop()

	// 记录一些延迟
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
	config.UpdateInterval = 50 * time.Millisecond
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

	// 等待回调
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !callbackCalled {
		t.Error("Expected callback to be called")
	}
	mu.Unlock()
}

// 基准测试

func BenchmarkWorkerPool_Submit(b *testing.B) {
	config := DefaultWorkerPoolConfig()
	config.WorkerCount = 4
	config.QueueSize = 1000

	pool := NewWorkerPool(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	defer pool.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			task := func() interface{} {
				return "done"
			}
			pool.Submit(task)
		}
	})
}

func BenchmarkMemoryPool_GetPut(b *testing.B) {
	config := DefaultMemoryPoolConfig()
	config.BufferSize = 1024
	config.MaxBuffers = 100

	pool := NewMemoryPool(config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
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
			cache.Set(key, value, 0)
			cache.Get(key)
			i++
		}
	})
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	config := DefaultRateLimiterConfig()
	config.Rate = 1000000 // 基准测试的高速率
	config.Burst = 1000

	limiter := NewRateLimiter(config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// 测试辅助函数

func allocateMemory(size int) []byte {
	return make([]byte, size)
}

func triggerGC() {
	runtime.GC()
	runtime.GC() // 调用两次以确保清理
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

func createFailingTestServer(failureRate float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if time.Now().UnixNano()%100 < int64(failureRate*100) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		}
	}))
}