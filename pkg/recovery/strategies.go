package recovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"movie-data-capture/pkg/retry"
)

// NetworkRecoveryStrategy 处理网络相关错误
type NetworkRecoveryStrategy struct {
	MaxRetries    int
	RetryInterval time.Duration
}

// GetName 返回策略名称
func (nrs *NetworkRecoveryStrategy) GetName() string {
	return "network_recovery"
}

// GetPriority 返回策略优先级
func (nrs *NetworkRecoveryStrategy) GetPriority() int {
	return 80
}

// CanRecover 判断此策略是否能从错误中恢复
func (nrs *NetworkRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network unreachable",
		"host unreachable",
		"no route to host",
		"timeout",
		"dial tcp",
		"dial udp",
		"dns",
		"temporary failure",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errorStr, netErr) {
			return true
		}
	}

	// 检查特定的网络错误类型
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}

// Recover 尝试从网络错误中恢复
func (nrs *NetworkRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	maxRetries := nrs.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	retryInterval := nrs.RetryInterval
	if retryInterval == 0 {
		retryInterval = 5 * time.Second
	}

	// 使用指数退避进行网络恢复
	retryConfig := &retry.Config{
		MaxAttempts:     maxRetries,
		InitialDelay:    retryInterval,
		MaxDelay:        60 * time.Second,
		BackoffStrategy: retry.ExponentialBackoff,
		Jitter:          true,
		RetryIf:         retry.NetworkRetryIf,
	}

	// 通过重试网络操作尝试恢复
	return retry.RetryWithContext(ctx, func(ctx context.Context) error {
		// 在实际实现中，这将重试特定的网络操作
		// 现在我们模拟一个恢复尝试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// 模拟成功恢复
			return nil
		}
	}, retryConfig)
}

// FileRecoveryStrategy 处理文件相关错误
type FileRecoveryStrategy struct {
	MaxRetries    int
	RetryInterval time.Duration
	CreateDirs    bool
	BackupFiles   bool
}

// GetName 返回策略名称
func (frs *FileRecoveryStrategy) GetName() string {
	return "file_recovery"
}

// GetPriority 返回策略优先级
func (frs *FileRecoveryStrategy) GetPriority() int {
	return 70
}

// CanRecover 判断此策略是否能从错误中恢复
func (frs *FileRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())
	fileErrors := []string{
		"no such file or directory",
		"file not found",
		"directory not found",
		"permission denied",
		"access denied",
		"file exists",
		"disk full",
		"no space left",
		"device busy",
		"resource temporarily unavailable",
		"sharing violation",
		"file locked",
	}

	for _, fileErr := range fileErrors {
		if strings.Contains(errorStr, fileErr) {
			return true
		}
	}

	// 检查特定的系统调用错误
	if pathErr, ok := err.(*os.PathError); ok {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			switch errno {
			case syscall.ENOENT, syscall.EACCES, syscall.EEXIST, syscall.ENOSPC, syscall.EBUSY:
				return true
			}
		}
	}

	return false
}

// Recover 尝试从文件错误中恢复
func (frs *FileRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	errorStr := strings.ToLower(err.Error())

	// 处理目录创建
	if strings.Contains(errorStr, "no such file or directory") ||
		strings.Contains(errorStr, "directory not found") {
		if frs.CreateDirs {
			return frs.createMissingDirectories(state)
		}
	}

	// 处理权限问题
	if strings.Contains(errorStr, "permission denied") ||
		strings.Contains(errorStr, "access denied") {
		return frs.handlePermissionError(state)
	}

	// 处理磁盘空间问题
	if strings.Contains(errorStr, "disk full") ||
		strings.Contains(errorStr, "no space left") {
		return frs.handleDiskSpaceError(state)
	}

	// 处理文件锁定问题
	if strings.Contains(errorStr, "file locked") ||
		strings.Contains(errorStr, "sharing violation") ||
		strings.Contains(errorStr, "device busy") {
		return frs.handleFileLockError(ctx, state)
	}

	return fmt.Errorf("unable to recover from file error: %w", err)
}

// createMissingDirectories 创建缺失的目录
func (frs *FileRecoveryStrategy) createMissingDirectories(state *ProcessState) error {
	// 从进程数据中提取目录路径
	if dirPath, ok := state.Data["directory_path"].(string); ok {
		return os.MkdirAll(dirPath, 0755)
	}

	if filePath, ok := state.Data["file_path"].(string); ok {
		dirPath := strings.TrimSuffix(filePath, "/"+getFileName(filePath))
		return os.MkdirAll(dirPath, 0755)
	}

	return fmt.Errorf("no directory path found in process data")
}

// handlePermissionError 处理权限相关错误
func (frs *FileRecoveryStrategy) handlePermissionError(state *ProcessState) error {
	// 在实际实现中，这可能会：
	// 1. 尝试更改文件权限
	// 2. 使用不同的文件位置
	// 3. 请求提升权限
	return fmt.Errorf("permission error recovery not implemented")
}

// handleDiskSpaceError 处理磁盘空间错误
func (frs *FileRecoveryStrategy) handleDiskSpaceError(state *ProcessState) error {
	// 在实际实现中，这可能会：
	// 1. 清理临时文件
	// 2. 将文件移动到不同位置
	// 3. 压缩现有文件
	return fmt.Errorf("disk space error recovery not implemented")
}

// handleFileLockError 处理文件锁定错误
func (frs *FileRecoveryStrategy) handleFileLockError(ctx context.Context, state *ProcessState) error {
	// 等待文件解锁
	retryConfig := &retry.Config{
		MaxAttempts:     10,
		InitialDelay:    500 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		BackoffStrategy: retry.LinearBackoff,
		Jitter:          false,
		RetryIf:         retry.FileRetryIf,
	}

	return retry.RetryWithContext(ctx, func(ctx context.Context) error {
		// 尝试访问文件
		if filePath, ok := state.Data["file_path"].(string); ok {
			file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
			if err != nil {
				return err
			}
			file.Close()
		}
		return nil
	}, retryConfig)
}

// getFileName 从路径中提取文件名
func getFileName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// RetryRecoveryStrategy 实现基于重试的通用恢复策略
type RetryRecoveryStrategy struct {
	MaxRetries      int
	RetryInterval   time.Duration
	BackoffStrategy retry.BackoffStrategy
	Jitter          bool
}

// GetName 返回策略名称
func (rrs *RetryRecoveryStrategy) GetName() string {
	return "retry_recovery"
}

// GetPriority 返回策略优先级
func (rrs *RetryRecoveryStrategy) GetPriority() int {
	return 50 // 较低优先级，用作回退
}

// CanRecover 判断此策略是否能从错误中恢复
func (rrs *RetryRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	// 此策略可以尝试从大多数错误中恢复
	// 但优先级低于特定策略
	return err != nil && state.RetryCount < state.MaxRetries
}

// Recover 通过重试操作尝试恢复
func (rrs *RetryRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	maxRetries := rrs.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	retryInterval := rrs.RetryInterval
	if retryInterval == 0 {
		retryInterval = 1 * time.Second
	}

	backoffStrategy := rrs.BackoffStrategy
	if backoffStrategy == 0 {
		backoffStrategy = retry.ExponentialBackoff
	}

	retryConfig := &retry.Config{
		MaxAttempts:     maxRetries,
		InitialDelay:    retryInterval,
		MaxDelay:        30 * time.Second,
		BackoffStrategy: backoffStrategy,
		Jitter:          rrs.Jitter,
		RetryIf:         retry.DefaultRetryIf,
	}

	return retry.RetryWithContext(ctx, func(ctx context.Context) error {
		// 模拟重试操作
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			// 在实际实现中，这将重试实际操作
			return nil
		}
	}, retryConfig)
}

// RestartRecoveryStrategy 实现进程重启恢复策略
type RestartRecoveryStrategy struct {
	MaxRestarts     int
	RestartInterval time.Duration
	CleanupFunc     func(*ProcessState) error
	InitFunc        func(*ProcessState) error
}

// GetName 返回策略名称
func (rrs *RestartRecoveryStrategy) GetName() string {
	return "restart_recovery"
}

// GetPriority 返回策略优先级
func (rrs *RestartRecoveryStrategy) GetPriority() int {
	return 30 // 较低优先级，用于严重错误
}

// CanRecover 判断此策略是否能从错误中恢复
func (rrs *RestartRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	if err == nil {
		return false
	}

	// 对严重错误或其他策略失败时使用重启恢复
	errorStr := strings.ToLower(err.Error())
	severeErrors := []string{
		"panic",
		"fatal",
		"segmentation fault",
		"out of memory",
		"stack overflow",
		"deadlock",
		"corruption",
	}

	for _, severeErr := range severeErrors {
		if strings.Contains(errorStr, severeErr) {
			return true
		}
	}

	// 也用于多次失败的进程
	return state.ErrorCount >= 3
}

// Recover 通过重启进程尝试恢复
func (rrs *RestartRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	maxRestarts := rrs.MaxRestarts
	if maxRestarts == 0 {
		maxRestarts = 2
	}

	if state.RetryCount >= maxRestarts {
		return fmt.Errorf("maximum restart attempts (%d) exceeded", maxRestarts)
	}

	// 重启前清理
	if rrs.CleanupFunc != nil {
		if err := rrs.CleanupFunc(state); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}
	}

	// 重启前等待
	restartInterval := rrs.RestartInterval
	if restartInterval == 0 {
		restartInterval = 5 * time.Second
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(restartInterval):
		// 继续重启
	}

	// 重置进程状态
	state.Status = StatusPending
	state.CompletedSteps = 0
	state.Progress = 0
	state.CurrentStep = ""
	state.LastError = ""
	state.RetryCount++

	// 为重启初始化
	if rrs.InitFunc != nil {
		if err := rrs.InitFunc(state); err != nil {
			return fmt.Errorf("initialization failed: %w", err)
		}
	}

	return nil
}

// CircuitBreakerRecoveryStrategy 实现熔断器模式
type CircuitBreakerRecoveryStrategy struct {
	MaxFailures  int
	ResetTimeout time.Duration
	breakers     map[string]*retry.CircuitBreaker
}

// GetName 返回策略名称
func (cbrs *CircuitBreakerRecoveryStrategy) GetName() string {
	return "circuit_breaker_recovery"
}

// GetPriority 返回策略优先级
func (cbrs *CircuitBreakerRecoveryStrategy) GetPriority() int {
	return 90 // 高优先级，用于防止级联故障
}

// CanRecover 判断此策略是否能从错误中恢复
func (cbrs *CircuitBreakerRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	// 熔断器可以处理任何错误，但专注于防止级联故障
	return err != nil
}

// Recover 使用熔断器模式尝试恢复
func (cbrs *CircuitBreakerRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	if cbrs.breakers == nil {
		cbrs.breakers = make(map[string]*retry.CircuitBreaker)
	}

	// 获取或创建此进程的熔断器
	breaker, exists := cbrs.breakers[state.ID]
	if !exists {
		maxFailures := cbrs.MaxFailures
		if maxFailures == 0 {
			maxFailures = 5
		}

		resetTimeout := cbrs.ResetTimeout
		if resetTimeout == 0 {
			resetTimeout = 60 * time.Second
		}

		breaker = retry.NewCircuitBreaker(maxFailures, resetTimeout)
		cbrs.breakers[state.ID] = breaker
	}

	// 通过熔断器执行
	return breaker.Execute(func() error {
		// 模拟操作重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// 在实际实现中，这将重试实际操作
			return nil
		}
	})
}

// CompositeRecoveryStrategy 组合多个恢复策略
type CompositeRecoveryStrategy struct {
	strategies []RecoveryStrategy
	name       string
	priority   int
}

// NewCompositeRecoveryStrategy 创建新的组合恢复策略
func NewCompositeRecoveryStrategy(name string, priority int, strategies ...RecoveryStrategy) *CompositeRecoveryStrategy {
	return &CompositeRecoveryStrategy{
		strategies: strategies,
		name:       name,
		priority:   priority,
	}
}

// GetName 返回策略名称
func (crs *CompositeRecoveryStrategy) GetName() string {
	return crs.name
}

// GetPriority 返回策略优先级
func (crs *CompositeRecoveryStrategy) GetPriority() int {
	return crs.priority
}

// CanRecover 判断组合策略中是否有任何一个可以恢复
func (crs *CompositeRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	for _, strategy := range crs.strategies {
		if strategy.CanRecover(err, state) {
			return true
		}
	}
	return false
}

// Recover 使用第一个适用的策略尝试恢复
func (crs *CompositeRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	for _, strategy := range crs.strategies {
		if strategy.CanRecover(err, state) {
			if recoveryErr := strategy.Recover(ctx, err, state); recoveryErr == nil {
				return nil // 恢复成功
			}
			// 如果此策略失败，继续下一个策略
		}
	}
	return fmt.Errorf("all recovery strategies failed for error: %w", err)
}