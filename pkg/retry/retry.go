package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryableFunc 表示可以重试的函数
type RetryableFunc func() error

// RetryableFuncWithContext 表示可以使用上下文重试的函数
type RetryableFuncWithContext func(ctx context.Context) error

// Config 保存重试配置
type Config struct {
	MaxAttempts     int           // 最大重试次数
	InitialDelay    time.Duration // 重试之间的初始延迟
	MaxDelay        time.Duration // 重试之间的最大延迟
	BackoffStrategy BackoffStrategy
	Jitter          bool // 为延迟添加随机抖动
	RetryIf         func(error) bool // 确定错误是否可重试的函数
}

// BackoffStrategy 定义不同的退避策略
type BackoffStrategy int

const (
	ConstantBackoff BackoffStrategy = iota
	LinearBackoff
	ExponentialBackoff
	FibonacciBackoff
)

// DefaultConfig 返回默认的重试配置
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffStrategy: ExponentialBackoff,
		Jitter:          true,
		RetryIf:         DefaultRetryIf,
	}
}

// NetworkConfig 返回针对网络操作优化的重试配置
func NetworkConfig() *Config {
	return &Config{
		MaxAttempts:     5,
		InitialDelay:    500 * time.Millisecond,
		MaxDelay:        60 * time.Second,
		BackoffStrategy: ExponentialBackoff,
		Jitter:          true,
		RetryIf:         NetworkRetryIf,
	}
}

// FileConfig 返回针对文件操作优化的重试配置
func FileConfig() *Config {
	return &Config{
		MaxAttempts:     3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		BackoffStrategy: LinearBackoff,
		Jitter:          false,
		RetryIf:         FileRetryIf,
	}
}

// Retry 使用重试逻辑执行函数
func Retry(fn RetryableFunc, config *Config) error {
	return RetryWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	}, config)
}

// RetryWithContext 使用重试逻辑和上下文支持执行函数
func RetryWithContext(ctx context.Context, fn RetryableFuncWithContext, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 执行函数
		err := fn(ctx)
		if err == nil {
			return nil // 成功
		}

		lastErr = err

		// 检查错误是否可重试
		if config.RetryIf != nil && !config.RetryIf(err) {
			return fmt.Errorf("non-retryable error after attempt %d: %w", attempt, err)
		}

		// 最后一次尝试后不延迟
		if attempt == config.MaxAttempts {
			break
		}

		// 计算延迟
		delay := config.calculateDelay(attempt)

		// 等待延迟或上下文取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// 继续下一次尝试
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded, last error: %w", config.MaxAttempts, lastErr)
}

// calculateDelay 计算给定尝试次数的延迟
func (c *Config) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch c.BackoffStrategy {
	case ConstantBackoff:
		delay = c.InitialDelay
	case LinearBackoff:
		delay = time.Duration(attempt) * c.InitialDelay
	case ExponentialBackoff:
		delay = time.Duration(math.Pow(2, float64(attempt-1))) * c.InitialDelay
	case FibonacciBackoff:
		delay = time.Duration(fibonacci(attempt)) * c.InitialDelay
	default:
		delay = c.InitialDelay
	}

	// 应用最大延迟限制
	if delay > c.MaxDelay {
		delay = c.MaxDelay
	}

	// 如果启用则添加抖动
	if c.Jitter {
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.1) // 10% 抖动
		delay += jitter
	}

	return delay
}

// fibonacci 计算第n个斐波那契数
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

// DefaultRetryIf 是确定错误是否可重试的默认函数
func DefaultRetryIf(err error) bool {
	if err == nil {
		return false
	}

	// 在此添加常见的可重试错误模式
	errorStr := err.Error()
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"service unavailable",
		"too many requests",
		"rate limit",
	}

	for _, pattern := range retryablePatterns {
		if contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// NetworkRetryIf 确定网络错误是否可重试
func NetworkRetryIf(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	networkRetryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"connection aborted",
		"network unreachable",
		"host unreachable",
		"no route to host",
		"temporary failure",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
		"too many requests",
		"rate limit",
		"502", "503", "504", "429", // HTTP 状态码
	}

	for _, pattern := range networkRetryablePatterns {
		if contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// FileRetryIf 确定文件操作错误是否可重试
func FileRetryIf(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	fileRetryablePatterns := []string{
		"resource temporarily unavailable",
		"device busy",
		"file locked",
		"sharing violation",
		"access denied", // 在 Windows 上有时是临时的
		"disk full", // 如果释放空间可能会解决
	}

	for _, pattern := range fileRetryablePatterns {
		if contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// contains 检查字符串是否包含子字符串（不区分大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		 len(s) > len(substr) && 
		 (s[:len(substr)] == substr || 
		  s[len(s)-len(substr):] == substr || 
		  indexOf(s, substr) >= 0))
}

// indexOf 查找字符串中子字符串的索引
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// RetryableError 包装带有重试信息的错误
type RetryableError struct {
	Err       error
	Retryable bool
	Attempt   int
}

// Error 实现 error 接口
func (re *RetryableError) Error() string {
	return fmt.Sprintf("attempt %d: %v (retryable: %t)", re.Attempt, re.Err, re.Retryable)
}

// Unwrap 返回包装的错误
func (re *RetryableError) Unwrap() error {
	return re.Err
}

// NewRetryableError 创建新的可重试错误
func NewRetryableError(err error, retryable bool, attempt int) *RetryableError {
	return &RetryableError{
		Err:       err,
		Retryable: retryable,
		Attempt:   attempt,
	}
}

// RetryWithCallback 使用重试逻辑和进度回调执行函数
func RetryWithCallback(fn RetryableFunc, config *Config, onRetry func(attempt int, err error)) error {
	return RetryWithContextAndCallback(context.Background(), func(ctx context.Context) error {
		return fn()
	}, config, onRetry)
}

// RetryWithContextAndCallback 使用重试逻辑、上下文支持和进度回调执行函数
func RetryWithContextAndCallback(ctx context.Context, fn RetryableFuncWithContext, config *Config, onRetry func(attempt int, err error)) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 执行函数
		err := fn(ctx)
		if err == nil {
			return nil // 成功
		}

		lastErr = err

		// 调用重试回调
		if onRetry != nil {
			onRetry(attempt, err)
		}

		// 检查错误是否可重试
		if config.RetryIf != nil && !config.RetryIf(err) {
			return fmt.Errorf("non-retryable error after attempt %d: %w", attempt, err)
		}

		// 最后一次尝试后不延迟
		if attempt == config.MaxAttempts {
			break
		}

		// 计算延迟
		delay := config.calculateDelay(attempt)

		// 等待延迟或上下文取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// 继续下一次尝试
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded, last error: %w", config.MaxAttempts, lastErr)
}

// CircuitBreaker 实现熔断器模式
type CircuitBreaker struct {
	maxFailures     int
	resetTimeout    time.Duration
	failureCount    int
	lastFailureTime time.Time
	state           CircuitState
}

// CircuitState 表示熔断器的状态
type CircuitState int

const (
	Closed CircuitState = iota
	Open
	HalfOpen
)

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        Closed,
	}
}

// Execute 通过熔断器执行函数
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if cb.state == Open {
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = HalfOpen
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := fn()
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// onFailure 处理失败
func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.maxFailures {
		cb.state = Open
	}
}

// onSuccess 处理成功
func (cb *CircuitBreaker) onSuccess() {
	cb.failureCount = 0
	cb.state = Closed
}

// GetState 返回熔断器的当前状态
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.state
}