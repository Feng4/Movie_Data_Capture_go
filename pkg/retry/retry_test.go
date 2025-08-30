package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestRetry_Success 测试无重试的成功执行
func TestRetry_Success(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return nil // 第一次尝试成功
	}

	config := DefaultConfig()
	err := Retry(fn, config)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got: %d", attempts)
	}
}

// TestRetry_SuccessAfterRetries 测试经过一些重试后的成功
func TestRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil // 第三次尝试成功
	}

	config := &Config{
		MaxAttempts:     5,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf:         DefaultRetryIf,
	}

	err := Retry(fn, config)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}
}

// TestRetry_MaxAttemptsExceeded 测试超过最大尝试次数后的失败
func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("persistent failure")
	}

	config := &Config{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf:         DefaultRetryIf,
	}

	err := Retry(fn, config)

	if err == nil {
		t.Error("Expected error after max attempts exceeded")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}

	expectedMsg := "max retry attempts (3) exceeded"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %s", expectedMsg, err.Error())
	}
}

// TestRetry_NonRetryableError 测试不可重试错误的处理
func TestRetry_NonRetryableError(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("non-retryable error")
	}

	config := &Config{
		MaxAttempts:     5,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf: func(err error) bool {
			return false // 永不重试
		},
	}

	err := Retry(fn, config)

	if err == nil {
		t.Error("Expected error for non-retryable error")
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got: %d", attempts)
	}

	expectedMsg := "non-retryable error after attempt 1"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %s", expectedMsg, err.Error())
	}
}

// TestRetryWithContext_Success 测试上下文感知的重试成功
func TestRetryWithContext_Success(t *testing.T) {
	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errors.New("timeout")
		}
		return nil
	}

	ctx := context.Background()
	config := NetworkConfig()
	config.MaxAttempts = 3
	config.InitialDelay = 1 * time.Millisecond

	err := RetryWithContext(ctx, fn, config)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got: %d", attempts)
	}
}

// TestRetryWithContext_Cancelled 测试上下文取消
func TestRetryWithContext_Cancelled(t *testing.T) {
	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errors.New("timeout")
	}

	ctx, cancel := context.WithCancel(context.Background())
	config := &Config{
		MaxAttempts:     10,
		InitialDelay:    50 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf:         DefaultRetryIf,
	}

	// 短暂延迟后取消上下文
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	err := RetryWithContext(ctx, fn, config)

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}

	// 应该至少尝试一次
	if attempts < 1 {
		t.Errorf("Expected at least 1 attempt, got: %d", attempts)
	}
}

// TestRetryWithContext_Timeout 测试上下文超时
func TestRetryWithContext_Timeout(t *testing.T) {
	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errors.New("connection refused")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	config := &Config{
		MaxAttempts:     10,
		InitialDelay:    20 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf:         NetworkRetryIf,
	}

	err := RetryWithContext(ctx, fn, config)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}
}

// TestBackoffStrategies 测试不同的退避策略
func TestBackoffStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy BackoffStrategy
		attempt  int
		initial  time.Duration
		expected time.Duration
	}{
		{"Constant", ConstantBackoff, 1, 100 * time.Millisecond, 100 * time.Millisecond},
		{"Constant", ConstantBackoff, 5, 100 * time.Millisecond, 100 * time.Millisecond},
		{"Linear", LinearBackoff, 1, 100 * time.Millisecond, 100 * time.Millisecond},
		{"Linear", LinearBackoff, 3, 100 * time.Millisecond, 300 * time.Millisecond},
		{"Exponential", ExponentialBackoff, 1, 100 * time.Millisecond, 100 * time.Millisecond},
		{"Exponential", ExponentialBackoff, 3, 100 * time.Millisecond, 400 * time.Millisecond},
		{"Fibonacci", FibonacciBackoff, 1, 100 * time.Millisecond, 100 * time.Millisecond},
		{"Fibonacci", FibonacciBackoff, 5, 100 * time.Millisecond, 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_attempt_%d", tt.name, tt.attempt), func(t *testing.T) {
			config := &Config{
				InitialDelay:    tt.initial,
				MaxDelay:        10 * time.Second,
				BackoffStrategy: tt.strategy,
				Jitter:          false,
			}

			actual := config.calculateDelay(tt.attempt)
			if actual != tt.expected {
				t.Errorf("Expected delay %v, got %v", tt.expected, actual)
			}
		})
	}
}

// TestMaxDelayLimit 测试最大延迟限制
func TestMaxDelayLimit(t *testing.T) {
	config := &Config{
		InitialDelay:    1 * time.Second,
		MaxDelay:        5 * time.Second,
		BackoffStrategy: ExponentialBackoff,
		Jitter:          false,
	}

	// 尝试 10 通常会给出 512 秒，但应该限制为 5 秒
	actual := config.calculateDelay(10)
	if actual != 5*time.Second {
		t.Errorf("Expected delay to be limited to 5s, got %v", actual)
	}
}

// TestJitter 测试抖动功能
func TestJitter(t *testing.T) {
	config := &Config{
		InitialDelay:    1 * time.Second,
		MaxDelay:        10 * time.Second,
		BackoffStrategy: ConstantBackoff,
		Jitter:          true,
	}

	// 多次计算延迟并确保它们不同（由于抖动）
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = config.calculateDelay(1)
	}

	// 检查至少有一些延迟是不同的
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to produce different delays")
	}

	// 所有延迟都应该在合理范围内（基础延迟 ± 10%）
	baseDelay := 1 * time.Second
	minDelay := baseDelay
	maxDelay := baseDelay + time.Duration(float64(baseDelay)*0.1)

	for i, delay := range delays {
		if delay < minDelay || delay > maxDelay {
			t.Errorf("Delay %d (%v) is outside expected range [%v, %v]", i, delay, minDelay, maxDelay)
		}
	}
}

// TestDefaultRetryIf 测试默认重试条件
func TestDefaultRetryIf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout error", errors.New("timeout"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"temporary failure", errors.New("temporary failure"), true},
		{"service unavailable", errors.New("service unavailable"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"non-retryable error", errors.New("invalid input"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := DefaultRetryIf(tt.err)
			if actual != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, actual, tt.err)
			}
		})
	}
}

// TestNetworkRetryIf 测试网络特定的重试条件
func TestNetworkRetryIf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection timeout", errors.New("connection timeout"), true},
		{"network unreachable", errors.New("network unreachable"), true},
		{"bad gateway", errors.New("502 bad gateway"), true},
		{"service unavailable", errors.New("503 service unavailable"), true},
		{"gateway timeout", errors.New("504 gateway timeout"), true},
		{"too many requests", errors.New("429 too many requests"), true},
		{"client error", errors.New("400 bad request"), false},
		{"not found", errors.New("404 not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := NetworkRetryIf(tt.err)
			if actual != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, actual, tt.err)
			}
		})
	}
}

// TestFileRetryIf 测试文件特定的重试条件
func TestFileRetryIf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"resource temporarily unavailable", errors.New("resource temporarily unavailable"), true},
		{"device busy", errors.New("device busy"), true},
		{"file locked", errors.New("file locked"), true},
		{"sharing violation", errors.New("sharing violation"), true},
		{"access denied", errors.New("access denied"), true},
		{"disk full", errors.New("disk full"), true},
		{"file not found", errors.New("file not found"), false},
		{"invalid path", errors.New("invalid path"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := FileRetryIf(tt.err)
			if actual != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, actual, tt.err)
			}
		})
	}
}

// TestRetryWithCallback 测试带进度回调的重试
func TestRetryWithCallback(t *testing.T) {
	attempts := 0
	callbackCalls := 0
	var callbackErrors []error

	fn := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	onRetry := func(attempt int, err error) {
		callbackCalls++
		callbackErrors = append(callbackErrors, err)
	}

	config := &Config{
		MaxAttempts:     5,
		InitialDelay:    1 * time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf:         DefaultRetryIf,
	}

	err := RetryWithCallback(fn, config, onRetry)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}

	if callbackCalls != 2 {
		t.Errorf("Expected 2 callback calls, got: %d", callbackCalls)
	}

	if len(callbackErrors) != 2 {
		t.Errorf("Expected 2 callback errors, got: %d", len(callbackErrors))
	}
}

// TestCircuitBreaker 测试断路器功能
func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// 测试初始状态
	if cb.GetState() != Closed {
		t.Errorf("Expected initial state to be Closed, got %v", cb.GetState())
	}

	// 测试成功执行
	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// 测试导致开路状态的失败
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return errors.New("failure")
		})
	}

	if cb.GetState() != Open {
		t.Errorf("Expected state to be Open after failures, got %v", cb.GetState())
	}

	// 测试断路器在开路时阻止执行
	err = cb.Execute(func() error {
		return nil
	})
	if err == nil {
		t.Error("Expected circuit breaker to prevent execution when open")
	}

	// 等待重置超时并测试半开状态
	time.Sleep(150 * time.Millisecond)
	err = cb.Execute(func() error {
		return nil // 成功应该关闭电路
	})
	if err != nil {
		t.Errorf("Expected successful execution after reset timeout, got: %v", err)
	}

	if cb.GetState() != Closed {
		t.Errorf("Expected state to be Closed after successful execution, got %v", cb.GetState())
	}
}

// TestRetryableError 测试可重试错误包装器
func TestRetryableError(t *testing.T) {
	originalErr := errors.New("original error")
	retryableErr := NewRetryableError(originalErr, true, 2)

	if retryableErr.Err != originalErr {
		t.Errorf("Expected wrapped error to be %v, got %v", originalErr, retryableErr.Err)
	}

	if !retryableErr.Retryable {
		t.Error("Expected error to be retryable")
	}

	if retryableErr.Attempt != 2 {
		t.Errorf("Expected attempt to be 2, got %d", retryableErr.Attempt)
	}

	expectedMsg := "attempt 2: original error (retryable: true)"
	if retryableErr.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, retryableErr.Error())
	}

	if retryableErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error to be %v, got %v", originalErr, retryableErr.Unwrap())
	}
}

// TestFibonacci 测试斐波那契计算
func TestFibonacci(t *testing.T) {
	tests := []struct {
		n        int
		expected int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 2},
		{4, 3},
		{5, 5},
		{6, 8},
		{7, 13},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("fibonacci(%d)", tt.n), func(t *testing.T) {
			actual := fibonacci(tt.n)
			if actual != tt.expected {
				t.Errorf("Expected fibonacci(%d) = %d, got %d", tt.n, tt.expected, actual)
			}
		})
	}
}

// TestContains 测试 contains 辅助函数
func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "hello", true},
		{"hello world", "world", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"hello", "hello world", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("contains('%s', '%s')", tt.s, tt.substr), func(t *testing.T) {
			actual := contains(tt.s, tt.substr)
			if actual != tt.expected {
				t.Errorf("Expected contains('%s', '%s') = %v, got %v", tt.s, tt.substr, tt.expected, actual)
			}
		})
	}
}

// BenchmarkRetry 基准测试重试性能
func BenchmarkRetry(b *testing.B) {
	config := &Config{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Microsecond,
		MaxDelay:        10 * time.Microsecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf:         DefaultRetryIf,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Retry(func() error {
			return nil // 总是成功
		}, config)
	}
}

// BenchmarkRetryWithFailures 基准测试带失败的重试
func BenchmarkRetryWithFailures(b *testing.B) {
	config := &Config{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Microsecond,
		MaxDelay:        10 * time.Microsecond,
		BackoffStrategy: ConstantBackoff,
		Jitter:          false,
		RetryIf:         DefaultRetryIf,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempts := 0
		Retry(func() error {
			attempts++
			if attempts < 2 {
				return errors.New("temporary failure")
			}
			return nil
		}, config)
	}
}

// BenchmarkCircuitBreaker 基准测试断路器性能
func BenchmarkCircuitBreaker(b *testing.B) {
	cb := NewCircuitBreaker(5, 100*time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() error {
			return nil
		})
	}
}