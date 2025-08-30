package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerPool 管理用于并发处理的工作池
type WorkerPool struct {
	mu          sync.RWMutex
	workers     []*Worker
	jobQueue    chan Job
	resultQueue chan JobResult
	config      *WorkerPoolConfig
	running     int32
	stopCh      chan struct{}
	wg          sync.WaitGroup
	stats       *PoolStats
}

// WorkerPoolConfig 工作池配置
type WorkerPoolConfig struct {
	WorkerCount    int           `json:"worker_count"`
	QueueSize      int           `json:"queue_size"`
	ResultSize     int           `json:"result_size"`
	Timeout        time.Duration `json:"timeout"`
	RetryAttempts  int           `json:"retry_attempts"`
	RetryDelay     time.Duration `json:"retry_delay"`
	EnableMetrics  bool          `json:"enable_metrics"`
	GracefulStop   bool          `json:"graceful_stop"`
	MaxIdleTime    time.Duration `json:"max_idle_time"`
	AutoScale      bool          `json:"auto_scale"`
	MinWorkers     int           `json:"min_workers"`
	MaxWorkers     int           `json:"max_workers"`
	ScaleThreshold float64       `json:"scale_threshold"`
}

// Worker 表示池中的单个工作者
type Worker struct {
	id          int
	jobQueue    chan Job
	resultQueue chan JobResult
	stopCh      chan struct{}
	lastActive  time.Time
	processed   uint64
	errors      uint64
	running     int32
}

// Job 表示一个工作单元
type Job struct {
	ID       string
	Data     interface{}
	Handler  JobHandler
	Timeout  time.Duration
	Retries  int
	Priority int
	Created  time.Time
}

// JobResult 表示作业的结果
type JobResult struct {
	JobID     string
	Result    interface{}
	Error     error
	Duration  time.Duration
	WorkerID  int
	Retries   int
	Completed time.Time
}

// JobHandler 处理作业的函数类型
type JobHandler func(ctx context.Context, data interface{}) (interface{}, error)

// PoolStats 保存工作池的统计信息
type PoolStats struct {
	mu              sync.RWMutex
	JobsSubmitted   uint64        `json:"jobs_submitted"`
	JobsCompleted   uint64        `json:"jobs_completed"`
	JobsFailed      uint64        `json:"jobs_failed"`
	JobsRetried     uint64        `json:"jobs_retried"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	ActiveWorkers   int           `json:"active_workers"`
	QueueLength     int           `json:"queue_length"`
	Throughput      float64       `json:"throughput"`
	LastUpdated     time.Time     `json:"last_updated"`
}

// RateLimiter 控制操作的速率
type RateLimiter struct {
	mu       sync.Mutex
	tokens   int
	capacity int
	rate     time.Duration
	lastRefill time.Time
	stopCh   chan struct{}
}

// Semaphore 提供计数信号量功能
type Semaphore struct {
	ch chan struct{}
}

// CircuitBreaker 实现熔断器模式以提供容错能力
type CircuitBreaker struct {
	mu           sync.RWMutex
	name         string
	state        CircuitState
	failureCount uint64
	lastFailure  time.Time
	nextAttempt  time.Time
	config       *CircuitBreakerConfig
	stats        *CircuitStats
}

// CircuitState 表示熔断器的状态
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureThreshold uint64        `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`
	SuccessThreshold uint64        `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
}

// CircuitStats 保存熔断器统计信息
type CircuitStats struct {
	mu            sync.RWMutex
	TotalRequests uint64 `json:"total_requests"`
	Successes     uint64 `json:"successes"`
	Failures      uint64 `json:"failures"`
	Timeouts      uint64 `json:"timeouts"`
	Rejected      uint64 `json:"rejected"`
}

// DefaultWorkerPoolConfig 返回默认的工作池配置
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		WorkerCount:    runtime.NumCPU(),
		QueueSize:      1000,
		ResultSize:     1000,
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     1 * time.Second,
		EnableMetrics:  true,
		GracefulStop:   true,
		MaxIdleTime:    5 * time.Minute,
		AutoScale:      false,
		MinWorkers:     1,
		MaxWorkers:     runtime.NumCPU() * 2,
		ScaleThreshold: 0.8,
	}
}

// NewWorkerPool 创建一个新的工作池
func NewWorkerPool(config *WorkerPoolConfig) *WorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	return &WorkerPool{
		config:      config,
		jobQueue:    make(chan Job, config.QueueSize),
		resultQueue: make(chan JobResult, config.ResultSize),
		stopCh:      make(chan struct{}),
		stats: &PoolStats{
			LastUpdated: time.Now(),
		},
	}
}

// Start 启动工作池
func (wp *WorkerPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&wp.running, 0, 1) {
		return fmt.Errorf("worker pool is already running")
	}

	wp.mu.Lock()
	defer wp.mu.Unlock()

	// 创建初始工作者
	workerCount := wp.config.WorkerCount
	if wp.config.AutoScale && wp.config.MinWorkers > 0 {
		workerCount = wp.config.MinWorkers
	}

	wp.workers = make([]*Worker, 0, wp.config.MaxWorkers)
	for i := 0; i < workerCount; i++ {
		worker := wp.createWorker(i)
		wp.workers = append(wp.workers, worker)
		wp.startWorker(ctx, worker)
	}

	// 如果启用了自动扩缩容，则启动
	if wp.config.AutoScale {
		go wp.autoScaleLoop(ctx)
	}

	// 如果启用了指标收集，则启动
	if wp.config.EnableMetrics {
		go wp.metricsLoop(ctx)
	}

	return nil
}

// Stop 停止工作池
func (wp *WorkerPool) Stop() {
	if !atomic.CompareAndSwapInt32(&wp.running, 1, 0) {
		return
	}

	close(wp.stopCh)

	if wp.config.GracefulStop {
		// 等待所有作业完成
		wp.wg.Wait()
	}

	wp.mu.Lock()
	for _, worker := range wp.workers {
		atomic.StoreInt32(&worker.running, 0)
		close(worker.stopCh)
	}
	wp.mu.Unlock()
}

// Submit 向工作池提交作业
func (wp *WorkerPool) Submit(job Job) error {
	if atomic.LoadInt32(&wp.running) == 0 {
		return fmt.Errorf("worker pool is not running")
	}

	job.Created = time.Now()
	if job.ID == "" {
		job.ID = fmt.Sprintf("job_%d", time.Now().UnixNano())
	}

	select {
	case wp.jobQueue <- job:
		atomic.AddUint64(&wp.stats.JobsSubmitted, 1)
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// GetResult 从结果队列获取作业结果
func (wp *WorkerPool) GetResult() <-chan JobResult {
	return wp.resultQueue
}

// GetStats 返回当前池统计信息
func (wp *WorkerPool) GetStats() *PoolStats {
	wp.stats.mu.RLock()
	defer wp.stats.mu.RUnlock()

	stats := *wp.stats
	stats.QueueLength = len(wp.jobQueue)
	stats.ActiveWorkers = len(wp.workers)

	return &stats
}

// createWorker 创建一个新的工作者
func (wp *WorkerPool) createWorker(id int) *Worker {
	return &Worker{
		id:          id,
		jobQueue:    wp.jobQueue,
		resultQueue: wp.resultQueue,
		stopCh:      make(chan struct{}),
		lastActive:  time.Now(),
	}
}

// startWorker 启动工作者协程
func (wp *WorkerPool) startWorker(ctx context.Context, worker *Worker) {
	atomic.StoreInt32(&worker.running, 1)
	wp.wg.Add(1)

	go func() {
		defer wp.wg.Done()
		worker.run(ctx, wp.config)
	}()
}

// run 运行工作者循环
func (w *Worker) run(ctx context.Context, config *WorkerPoolConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case job := <-w.jobQueue:
			w.processJob(ctx, job, config)
		}
	}
}

// processJob 处理单个作业
func (w *Worker) processJob(ctx context.Context, job Job, config *WorkerPoolConfig) {
	start := time.Now()
	w.lastActive = start

	// 创建带超时的作业上下文
	jobCtx := ctx
	if job.Timeout > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, job.Timeout)
		defer cancel()
	} else if config.Timeout > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	var result interface{}
	var err error
	retries := 0
	maxRetries := job.Retries
	if maxRetries == 0 {
		maxRetries = config.RetryAttempts
	}

	for retries <= maxRetries {
		result, err = job.Handler(jobCtx, job.Data)
		if err == nil {
			break
		}

		retries++
		if retries <= maxRetries {
			time.Sleep(config.RetryDelay)
		}
	}

	duration := time.Since(start)
	jobResult := JobResult{
		JobID:     job.ID,
		Result:    result,
		Error:     err,
		Duration:  duration,
		WorkerID:  w.id,
		Retries:   retries,
		Completed: time.Now(),
	}

	// 更新工作者统计信息
	atomic.AddUint64(&w.processed, 1)
	if err != nil {
		atomic.AddUint64(&w.errors, 1)
	}

	// 发送结果
	select {
	case w.resultQueue <- jobResult:
	default:
		// 结果队列已满，丢弃结果
	}
}

// autoScaleLoop 处理工作者的自动扩缩容
func (wp *WorkerPool) autoScaleLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopCh:
			return
		case <-ticker.C:
			wp.checkAndScale(ctx)
		}
	}
}

// checkAndScale 检查是否需要扩缩容并执行
func (wp *WorkerPool) checkAndScale(ctx context.Context) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	queueLength := len(wp.jobQueue)
	workerCount := len(wp.workers)
	queueUtilization := float64(queueLength) / float64(wp.config.QueueSize)

	// 如果队列利用率高则扩容
	if queueUtilization > wp.config.ScaleThreshold && workerCount < wp.config.MaxWorkers {
		newWorker := wp.createWorker(workerCount)
		wp.workers = append(wp.workers, newWorker)
		wp.startWorker(ctx, newWorker)
	}

	// 如果工作者空闲则缩容
	if queueUtilization < 0.2 && workerCount > wp.config.MinWorkers {
		// 查找空闲工作者
		now := time.Now()
		for i := len(wp.workers) - 1; i >= wp.config.MinWorkers; i-- {
			worker := wp.workers[i]
			if now.Sub(worker.lastActive) > wp.config.MaxIdleTime {
				// 停止工作者
				atomic.StoreInt32(&worker.running, 0)
				close(worker.stopCh)
				// 从切片中移除
				wp.workers = append(wp.workers[:i], wp.workers[i+1:]...)
				break
			}
		}
	}
}

// metricsLoop 收集和更新指标
func (wp *WorkerPool) metricsLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopCh:
			return
		case <-ticker.C:
			wp.updateMetrics()
		}
	}
}

// updateMetrics 更新池指标
func (wp *WorkerPool) updateMetrics() {
	wp.stats.mu.Lock()
	defer wp.stats.mu.Unlock()

	now := time.Now()
	duration := now.Sub(wp.stats.LastUpdated)

	if duration > 0 {
		completedJobs := atomic.LoadUint64(&wp.stats.JobsCompleted)
		wp.stats.Throughput = float64(completedJobs) / duration.Seconds()
	}

	// 更新平均持续时间
	if wp.stats.JobsCompleted > 0 {
		wp.stats.AverageDuration = wp.stats.TotalDuration / time.Duration(wp.stats.JobsCompleted)
	}

	wp.stats.LastUpdated = now
}

// NewRateLimiter 创建一个新的速率限制器
func NewRateLimiter(capacity int, rate time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:     capacity,
		capacity:   capacity,
		rate:       rate,
		lastRefill: time.Now(),
		stopCh:     make(chan struct{}),
	}
}

// Allow 检查是否允许操作
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// Wait 等待直到允许操作
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(rl.rate / time.Duration(rl.capacity)):
			// 继续下一次迭代
		}
	}
}

// refill 根据经过的时间补充令牌
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.rate)

	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.capacity {
			rl.tokens = rl.capacity
		}
		rl.lastRefill = now
	}
}

// NewSemaphore 创建一个具有给定容量的新信号量
func NewSemaphore(capacity int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, capacity),
	}
}

// Acquire 获取信号量许可
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire 尝试非阻塞地获取信号量许可
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release 释放信号量许可
func (s *Semaphore) Release() {
	select {
	case <-s.ch:
	default:
		// 如果正确使用不应该发生
	}
}

// Available 返回可用许可的数量
func (s *Semaphore) Available() int {
	return cap(s.ch) - len(s.ch)
}

// NewCircuitBreaker 创建一个新的熔断器
func NewCircuitBreaker(name string, config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = &CircuitBreakerConfig{
			FailureThreshold: 5,
			RecoveryTimeout:  30 * time.Second,
			SuccessThreshold: 3,
			Timeout:          10 * time.Second,
		}
	}

	return &CircuitBreaker{
		name:   name,
		state:  CircuitClosed,
		config: config,
		stats:  &CircuitStats{},
	}
}

// Execute 在熔断器保护下执行函数
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	cb.mu.Lock()
	state := cb.state
	cb.mu.Unlock()

	atomic.AddUint64(&cb.stats.TotalRequests, 1)

	switch state {
	case CircuitOpen:
		if time.Now().Before(cb.nextAttempt) {
			atomic.AddUint64(&cb.stats.Rejected, 1)
			return nil, fmt.Errorf("circuit breaker %s is open", cb.name)
		}
		cb.setState(CircuitHalfOpen)
		fallthrough
	case CircuitHalfOpen:
		fallthrough
	case CircuitClosed:
		// 创建带超时的上下文
		fnCtx, cancel := context.WithTimeout(ctx, cb.config.Timeout)
		defer cancel()

		resultCh := make(chan interface{}, 1)
		errorCh := make(chan error, 1)

		go func() {
			result, err := fn()
			if err != nil {
				errorCh <- err
			} else {
				resultCh <- result
			}
		}()

		select {
		case result := <-resultCh:
			cb.onSuccess()
			return result, nil
		case err := <-errorCh:
			cb.onFailure()
			return nil, err
		case <-fnCtx.Done():
			atomic.AddUint64(&cb.stats.Timeouts, 1)
			cb.onFailure()
			return nil, fmt.Errorf("circuit breaker %s: operation timeout", cb.name)
		}
	}

	return nil, fmt.Errorf("unknown circuit breaker state")
}

// onSuccess 处理成功执行
func (cb *CircuitBreaker) onSuccess() {
	atomic.AddUint64(&cb.stats.Successes, 1)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		// 检查是否有足够的成功次数来关闭熔断器
		if atomic.LoadUint64(&cb.stats.Successes) >= cb.config.SuccessThreshold {
			cb.setState(CircuitClosed)
			cb.failureCount = 0
		}
	} else if cb.state == CircuitClosed {
		cb.failureCount = 0
	}
}

// onFailure 处理失败执行
func (cb *CircuitBreaker) onFailure() {
	atomic.AddUint64(&cb.stats.Failures, 1)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.state == CircuitClosed && cb.failureCount >= cb.config.FailureThreshold {
		cb.setState(CircuitOpen)
		cb.nextAttempt = time.Now().Add(cb.config.RecoveryTimeout)
	} else if cb.state == CircuitHalfOpen {
		cb.setState(CircuitOpen)
		cb.nextAttempt = time.Now().Add(cb.config.RecoveryTimeout)
	}
}

// setState 设置熔断器状态
func (cb *CircuitBreaker) setState(state CircuitState) {
	cb.state = state
}

// GetState 返回当前熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 返回熔断器统计信息
func (cb *CircuitBreaker) GetStats() *CircuitStats {
	cb.stats.mu.RLock()
	defer cb.stats.mu.RUnlock()
	return &CircuitStats{
		TotalRequests: atomic.LoadUint64(&cb.stats.TotalRequests),
		Successes:     atomic.LoadUint64(&cb.stats.Successes),
		Failures:      atomic.LoadUint64(&cb.stats.Failures),
		Timeouts:      atomic.LoadUint64(&cb.stats.Timeouts),
		Rejected:      atomic.LoadUint64(&cb.stats.Rejected),
	}
}