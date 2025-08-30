package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RecoveryManager 管理错误恢复和状态持久化
type RecoveryManager struct {
	mu            sync.RWMutex
	stateFile     string
	backupDir     string
	strategies    map[string]RecoveryStrategy
	processStates map[string]*ProcessState
	config        *RecoveryConfig
}

// RecoveryConfig 保存恢复配置
type RecoveryConfig struct {
	StateFile           string        `json:"state_file"`
	BackupDir           string        `json:"backup_dir"`
	SaveInterval        time.Duration `json:"save_interval"`
	MaxBackups          int           `json:"max_backups"`
	AutoRecovery        bool          `json:"auto_recovery"`
	RecoveryTimeout     time.Duration `json:"recovery_timeout"`
	CleanupOldStates    bool          `json:"cleanup_old_states"`
	MaxStateAge         time.Duration `json:"max_state_age"`
	CompressionEnabled  bool          `json:"compression_enabled"`
	EncryptionEnabled   bool          `json:"encryption_enabled"`
	EncryptionKey       string        `json:"encryption_key,omitempty"`
}

// ProcessState 表示进程的状态
type ProcessState struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Status        ProcessStatus          `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	LastUpdate    time.Time              `json:"last_update"`
	Progress      float64                `json:"progress"`
	CurrentStep   string                 `json:"current_step"`
	TotalSteps    int                    `json:"total_steps"`
	CompletedSteps int                   `json:"completed_steps"`
	ErrorCount    int                    `json:"error_count"`
	LastError     string                 `json:"last_error,omitempty"`
	RetryCount    int                    `json:"retry_count"`
	MaxRetries    int                    `json:"max_retries"`
	Data          map[string]interface{} `json:"data"`
	Checkpoints   []Checkpoint           `json:"checkpoints"`
}

// ProcessStatus 表示进程的状态
type ProcessStatus int

const (
	StatusPending ProcessStatus = iota
	StatusRunning
	StatusPaused
	StatusCompleted
	StatusFailed
	StatusRecovering
	StatusCancelled
)

// String 返回 ProcessStatus 的字符串表示
func (ps ProcessStatus) String() string {
	switch ps {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusPaused:
		return "paused"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusRecovering:
		return "recovering"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Checkpoint 表示恢复检查点
type Checkpoint struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Step      string                 `json:"step"`
	Data      map[string]interface{} `json:"data"`
	Hash      string                 `json:"hash"`
}

// RecoveryStrategy 定义如何从特定错误中恢复
type RecoveryStrategy interface {
	CanRecover(err error, state *ProcessState) bool
	Recover(ctx context.Context, err error, state *ProcessState) error
	GetName() string
	GetPriority() int
}

// DefaultRecoveryConfig 返回默认恢复配置
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		StateFile:          "recovery_state.json",
		BackupDir:          "recovery_backups",
		SaveInterval:       30 * time.Second,
		MaxBackups:         10,
		AutoRecovery:       true,
		RecoveryTimeout:    5 * time.Minute,
		CleanupOldStates:   true,
		MaxStateAge:        24 * time.Hour,
		CompressionEnabled: false,
		EncryptionEnabled:  false,
	}
}

// NewRecoveryManager 创建新的恢复管理器
func NewRecoveryManager(config *RecoveryConfig) (*RecoveryManager, error) {
	if config == nil {
		config = DefaultRecoveryConfig()
	}

	rm := &RecoveryManager{
		stateFile:     config.StateFile,
		backupDir:     config.BackupDir,
		strategies:    make(map[string]RecoveryStrategy),
		processStates: make(map[string]*ProcessState),
		config:        config,
	}

	// 如果备份目录不存在则创建
	if err := os.MkdirAll(config.BackupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 加载现有状态
	if err := rm.LoadState(); err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// 注册默认恢复策略
	rm.RegisterStrategy(&NetworkRecoveryStrategy{})
	rm.RegisterStrategy(&FileRecoveryStrategy{})
	rm.RegisterStrategy(&RetryRecoveryStrategy{})
	rm.RegisterStrategy(&RestartRecoveryStrategy{})

	return rm, nil
}

// RegisterStrategy 注册恢复策略
func (rm *RecoveryManager) RegisterStrategy(strategy RecoveryStrategy) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.strategies[strategy.GetName()] = strategy
}

// UnregisterStrategy 取消注册恢复策略
func (rm *RecoveryManager) UnregisterStrategy(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.strategies, name)
}

// CreateProcess 创建新的进程状态
func (rm *RecoveryManager) CreateProcess(id, name string, totalSteps int) *ProcessState {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state := &ProcessState{
		ID:             id,
		Name:           name,
		Status:         StatusPending,
		StartTime:      time.Now(),
		LastUpdate:     time.Now(),
		TotalSteps:     totalSteps,
		CompletedSteps: 0,
		MaxRetries:     3,
		Data:           make(map[string]interface{}),
		Checkpoints:    make([]Checkpoint, 0),
	}

	rm.processStates[id] = state
	rm.saveStateAsync()
	return state
}

// UpdateProcess 更新进程状态
func (rm *RecoveryManager) UpdateProcess(id string, updates func(*ProcessState)) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.processStates[id]
	if !exists {
		return fmt.Errorf("process %s not found", id)
	}

	updates(state)
	state.LastUpdate = time.Now()
	rm.saveStateAsync()
	return nil
}

// GetProcess 返回进程状态
func (rm *RecoveryManager) GetProcess(id string) (*ProcessState, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	state, exists := rm.processStates[id]
	if !exists {
		return nil, fmt.Errorf("process %s not found", id)
	}

	return state, nil
}

// ListProcesses 返回所有进程状态
func (rm *RecoveryManager) ListProcesses() map[string]*ProcessState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[string]*ProcessState)
	for id, state := range rm.processStates {
		result[id] = state
	}
	return result
}

// CreateCheckpoint 创建恢复检查点
func (rm *RecoveryManager) CreateCheckpoint(processID, step string, data map[string]interface{}) error {
	return rm.UpdateProcess(processID, func(state *ProcessState) {
		checkpoint := Checkpoint{
			ID:        fmt.Sprintf("%s_%d", processID, len(state.Checkpoints)),
			Timestamp: time.Now(),
			Step:      step,
			Data:      data,
			Hash:      rm.calculateCheckpointHash(data),
		}
		state.Checkpoints = append(state.Checkpoints, checkpoint)
	})
}

// RecoverProcess 尝试恢复失败的进程
func (rm *RecoveryManager) RecoverProcess(ctx context.Context, processID string, err error) error {
	rm.mu.RLock()
	state, exists := rm.processStates[processID]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("进程 %s 未找到", processID)
	}

	// 更新进程状态为恢复中
	rm.UpdateProcess(processID, func(s *ProcessState) {
		s.Status = StatusRecovering
		s.LastError = err.Error()
	})

	// 查找合适的恢复策略
	strategy := rm.findRecoveryStrategy(err, state)
	if strategy == nil {
		return fmt.Errorf("未找到适合的错误恢复策略: %w", err)
	}

	// 使用超时尝试恢复
	recoveryCtx, cancel := context.WithTimeout(ctx, rm.config.RecoveryTimeout)
	defer cancel()

	recoveryErr := strategy.Recover(recoveryCtx, err, state)
	if recoveryErr != nil {
		rm.UpdateProcess(processID, func(s *ProcessState) {
			s.Status = StatusFailed
			s.ErrorCount++
			s.LastError = recoveryErr.Error()
		})
		return fmt.Errorf("恢复失败: %w", recoveryErr)
	}

	// 恢复成功
	rm.UpdateProcess(processID, func(s *ProcessState) {
		s.Status = StatusRunning
		s.LastError = ""
	})

	return nil
}

// findRecoveryStrategy 为错误查找最佳恢复策略
func (rm *RecoveryManager) findRecoveryStrategy(err error, state *ProcessState) RecoveryStrategy {
	var bestStrategy RecoveryStrategy
	bestPriority := -1

	for _, strategy := range rm.strategies {
		if strategy.CanRecover(err, state) && strategy.GetPriority() > bestPriority {
			bestStrategy = strategy
			bestPriority = strategy.GetPriority()
		}
	}

	return bestStrategy
}

// SaveState 将当前状态保存到磁盘
func (rm *RecoveryManager) SaveState() error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	data, err := json.MarshalIndent(rm.processStates, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// 保存前创建备份
	if err := rm.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// 写入状态文件
	if err := ioutil.WriteFile(rm.stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState 从磁盘加载状态
func (rm *RecoveryManager) LoadState() error {
	if _, err := os.Stat(rm.stateFile); os.IsNotExist(err) {
		return nil // 尚未存在状态文件
	}

	data, err := ioutil.ReadFile(rm.stateFile)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if err := json.Unmarshal(data, &rm.processStates); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return nil
}

// createBackup 创建当前状态文件的备份
func (rm *RecoveryManager) createBackup() error {
	if _, err := os.Stat(rm.stateFile); os.IsNotExist(err) {
		return nil // 没有状态文件需要备份
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(rm.backupDir, fmt.Sprintf("state_backup_%s.json", timestamp))

	data, err := ioutil.ReadFile(rm.stateFile)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(backupFile, data, 0644); err != nil {
		return err
	}

	// 清理旧备份
	return rm.cleanupOldBackups()
}

// cleanupOldBackups 删除旧的备份文件
func (rm *RecoveryManager) cleanupOldBackups() error {
	files, err := ioutil.ReadDir(rm.backupDir)
	if err != nil {
		return err
	}

	if len(files) <= rm.config.MaxBackups {
		return nil
	}

	// 按修改时间排序文件（最旧的在前）
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].ModTime().After(files[j].ModTime()) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// 删除最旧的文件
	filesToRemove := len(files) - rm.config.MaxBackups
	for i := 0; i < filesToRemove; i++ {
		filePath := filepath.Join(rm.backupDir, files[i].Name())
		if err := os.Remove(filePath); err != nil {
			return err
		}
	}

	return nil
}

// saveStateAsync 异步保存状态
func (rm *RecoveryManager) saveStateAsync() {
	go func() {
		if err := rm.SaveState(); err != nil {
			// 记录错误（在实际实现中，使用适当的日志记录）
			fmt.Printf("Failed to save state: %v\n", err)
		}
	}()
}

// calculateCheckpointHash 计算检查点数据的哈希值
func (rm *RecoveryManager) calculateCheckpointHash(data map[string]interface{}) string {
	// 简单的哈希实现 - 在生产环境中，使用适当的哈希函数
	jsonData, _ := json.Marshal(data)
	return fmt.Sprintf("%x", len(jsonData))
}

// AutoRecover 尝试自动恢复失败的进程
func (rm *RecoveryManager) AutoRecover(ctx context.Context) error {
	if !rm.config.AutoRecovery {
		return nil
	}

	rm.mu.RLock()
	failedProcesses := make([]*ProcessState, 0)
	for _, state := range rm.processStates {
		if state.Status == StatusFailed && state.RetryCount < state.MaxRetries {
			failedProcesses = append(failedProcesses, state)
		}
	}
	rm.mu.RUnlock()

	for _, state := range failedProcesses {
		if state.LastError != "" {
			err := fmt.Errorf(state.LastError)
			if recoveryErr := rm.RecoverProcess(ctx, state.ID, err); recoveryErr != nil {
				// 增加重试计数
				rm.UpdateProcess(state.ID, func(s *ProcessState) {
					s.RetryCount++
				})
			}
		}
	}

	return nil
}

// CleanupOldStates 删除旧的进程状态
func (rm *RecoveryManager) CleanupOldStates() error {
	if !rm.config.CleanupOldStates {
		return nil
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoff := time.Now().Add(-rm.config.MaxStateAge)
	for id, state := range rm.processStates {
		if (state.Status == StatusCompleted || state.Status == StatusCancelled) &&
			state.LastUpdate.Before(cutoff) {
			delete(rm.processStates, id)
		}
	}

	return rm.SaveState()
}

// GetRecoveryStats 返回恢复统计信息
func (rm *RecoveryManager) GetRecoveryStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_processes":    len(rm.processStates),
		"pending_processes":  0,
		"running_processes":  0,
		"completed_processes": 0,
		"failed_processes":   0,
		"recovering_processes": 0,
		"cancelled_processes": 0,
		"total_errors":       0,
		"total_retries":      0,
	}

	for _, state := range rm.processStates {
		switch state.Status {
		case StatusPending:
			stats["pending_processes"] = stats["pending_processes"].(int) + 1
		case StatusRunning:
			stats["running_processes"] = stats["running_processes"].(int) + 1
		case StatusCompleted:
			stats["completed_processes"] = stats["completed_processes"].(int) + 1
		case StatusFailed:
			stats["failed_processes"] = stats["failed_processes"].(int) + 1
		case StatusRecovering:
			stats["recovering_processes"] = stats["recovering_processes"].(int) + 1
		case StatusCancelled:
			stats["cancelled_processes"] = stats["cancelled_processes"].(int) + 1
		}
		stats["total_errors"] = stats["total_errors"].(int) + state.ErrorCount
		stats["total_retries"] = stats["total_retries"].(int) + state.RetryCount
	}

	return stats
}