package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecoveryManager_CreateProcess 测试进程创建
func TestRecoveryManager_CreateProcess(t *testing.T) {
	tempDir := t.TempDir()
	config := &RecoveryConfig{
		StateFile: filepath.Join(tempDir, "test_state.json"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 创建一个进程
	process := rm.CreateProcess("test-process-1", "Test Process", 5)
	if process == nil {
		t.Fatal("Failed to create process")
	}

	if process.ID != "test-process-1" {
		t.Errorf("Expected process ID 'test-process-1', got '%s'", process.ID)
	}

	if process.Name != "Test Process" {
		t.Errorf("Expected process name 'Test Process', got '%s'", process.Name)
	}

	if process.TotalSteps != 5 {
		t.Errorf("Expected total steps 5, got %d", process.TotalSteps)
	}

	if process.Status != StatusPending {
		t.Errorf("Expected status pending, got %v", process.Status)
	}
}

// TestRecoveryManager_UpdateProcess 测试进程更新
func TestRecoveryManager_UpdateProcess(t *testing.T) {
	tempDir := t.TempDir()
	config := &RecoveryConfig{
		StateFile: filepath.Join(tempDir, "test_state.json"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 创建并更新一个进程
	_ = rm.CreateProcess("test-process-2", "Test Process 2", 3)

	err = rm.UpdateProcess("test-process-2", func(state *ProcessState) {
		state.Status = StatusRunning
		state.CompletedSteps = 1
		state.Progress = 33.33
		state.CurrentStep = "Step 1"
	})

	if err != nil {
		t.Fatalf("Failed to update process: %v", err)
	}

	// 验证更新
	updatedProcess, err := rm.GetProcess("test-process-2")
	if err != nil {
		t.Fatalf("Failed to get process: %v", err)
	}

	if updatedProcess.Status != StatusRunning {
		t.Errorf("Expected status running, got %v", updatedProcess.Status)
	}

	if updatedProcess.CompletedSteps != 1 {
		t.Errorf("Expected completed steps 1, got %d", updatedProcess.CompletedSteps)
	}

	if updatedProcess.CurrentStep != "Step 1" {
		t.Errorf("Expected current step 'Step 1', got '%s'", updatedProcess.CurrentStep)
	}
}

// TestRecoveryManager_CreateCheckpoint 测试检查点创建
func TestRecoveryManager_CreateCheckpoint(t *testing.T) {
	tempDir := t.TempDir()
	config := &RecoveryConfig{
		StateFile: filepath.Join(tempDir, "test_state.json"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 创建一个进程和检查点
	_ = rm.CreateProcess("test-process-3", "Test Process 3", 2)

	checkpointData := map[string]interface{}{
		"file_path": "/tmp/test.txt",
		"progress":  50,
	}

	err = rm.CreateCheckpoint("test-process-3", "file_processing", checkpointData)
	if err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	// 验证检查点
	updatedProcess, err := rm.GetProcess("test-process-3")
	if err != nil {
		t.Fatalf("Failed to get process: %v", err)
	}

	if len(updatedProcess.Checkpoints) != 1 {
		t.Errorf("Expected 1 checkpoint, got %d", len(updatedProcess.Checkpoints))
	}

	checkpoint := updatedProcess.Checkpoints[0]
	if checkpoint.Step != "file_processing" {
		t.Errorf("Expected checkpoint step 'file_processing', got '%s'", checkpoint.Step)
	}

	if checkpoint.Data["file_path"] != "/tmp/test.txt" {
		t.Errorf("Expected file_path '/tmp/test.txt', got '%v'", checkpoint.Data["file_path"])
	}
}

// TestRecoveryManager_SaveLoadState 测试状态持久化
func TestRecoveryManager_SaveLoadState(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")
	config := &RecoveryConfig{
		StateFile: stateFile,
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	// 创建第一个管理器并添加进程
	rm1, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create recovery manager: %v", err)
	}

	process1 := rm1.CreateProcess("test-process-4", "Test Process 4", 3)
	rm1.UpdateProcess("test-process-4", func(state *ProcessState) {
		state.Status = StatusRunning
		state.CompletedSteps = 2
	})

	// 保存状态
	err = rm1.SaveState()
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// 创建第二个管理器并加载状态
	rm2, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create second recovery manager: %v", err)
	}

	// 验证加载的状态
	loadedProcess, err := rm2.GetProcess("test-process-4")
	if err != nil {
		t.Fatalf("Failed to get loaded process: %v", err)
	}

	if loadedProcess.Name != "Test Process 4" {
		t.Errorf("Expected process name 'Test Process 4', got '%s'", loadedProcess.Name)
	}

	if loadedProcess.Status != StatusRunning {
		t.Errorf("Expected status running, got %v", loadedProcess.Status)
	}

	if loadedProcess.CompletedSteps != 2 {
		t.Errorf("Expected completed steps 2, got %d", loadedProcess.CompletedSteps)
	}
}

// TestNetworkRecoveryStrategy 测试网络恢复
func TestNetworkRecoveryStrategy(t *testing.T) {
	strategy := &NetworkRecoveryStrategy{
		MaxRetries:    2,
		RetryInterval: 100 * time.Millisecond,
	}

	// 测试 CanRecover
	networkErr := errors.New("connection refused")
	state := &ProcessState{ID: "test", RetryCount: 0}

	if !strategy.CanRecover(networkErr, state) {
		t.Error("Expected strategy to handle network error")
	}

	normalErr := errors.New("some other error")
	if strategy.CanRecover(normalErr, state) {
		t.Error("Expected strategy to not handle normal error")
	}

	// 测试 Recover
	ctx := context.Background()
	err := strategy.Recover(ctx, networkErr, state)
	if err != nil {
		t.Errorf("Expected successful recovery, got error: %v", err)
	}
}

// TestFileRecoveryStrategy 测试文件恢复
func TestFileRecoveryStrategy(t *testing.T) {
	strategy := &FileRecoveryStrategy{
		MaxRetries:    2,
		RetryInterval: 100 * time.Millisecond,
		CreateDirs:    true,
	}

	// 测试 CanRecover
	fileErr := errors.New("no such file or directory")
	state := &ProcessState{ID: "test", RetryCount: 0}

	if !strategy.CanRecover(fileErr, state) {
		t.Error("Expected strategy to handle file error")
	}

	normalErr := errors.New("some other error")
	if strategy.CanRecover(normalErr, state) {
		t.Error("Expected strategy to not handle normal error")
	}

	// 测试目录创建恢复
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test_recovery_dir")
	state.Data = map[string]interface{}{
		"directory_path": testDir,
	}

	ctx := context.Background()
	err := strategy.Recover(ctx, fileErr, state)
	if err != nil {
		t.Errorf("Expected successful recovery, got error: %v", err)
	}

	// 验证目录已创建
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}
}

// TestRetryRecoveryStrategy 测试重试恢复
func TestRetryRecoveryStrategy(t *testing.T) {
	strategy := &RetryRecoveryStrategy{
		MaxRetries:    2,
		RetryInterval: 50 * time.Millisecond,
	}

	// 测试 CanRecover
	anyErr := errors.New("any error")
	state := &ProcessState{ID: "test", RetryCount: 0, MaxRetries: 3}

	if !strategy.CanRecover(anyErr, state) {
		t.Error("Expected strategy to handle any error when retries available")
	}

	// 测试超过最大重试次数
	state.RetryCount = 3
	if strategy.CanRecover(anyErr, state) {
		t.Error("Expected strategy to not handle error when max retries exceeded")
	}

	// 测试 Recover
	state.RetryCount = 0
	ctx := context.Background()
	err := strategy.Recover(ctx, anyErr, state)
	if err != nil {
		t.Errorf("Expected successful recovery, got error: %v", err)
	}
}

// TestRestartRecoveryStrategy 测试重启恢复
func TestRestartRecoveryStrategy(t *testing.T) {
	cleanupCalled := false
	initCalled := false

	strategy := &RestartRecoveryStrategy{
		MaxRestarts:     2,
		RestartInterval: 50 * time.Millisecond,
		CleanupFunc: func(state *ProcessState) error {
			cleanupCalled = true
			return nil
		},
		InitFunc: func(state *ProcessState) error {
			initCalled = true
			return nil
		},
	}

	// 测试 CanRecover
	severeErr := errors.New("panic: runtime error")
	state := &ProcessState{ID: "test", RetryCount: 0, ErrorCount: 0}

	if !strategy.CanRecover(severeErr, state) {
		t.Error("Expected strategy to handle severe error")
	}

	// 测试高错误计数
	state.ErrorCount = 5
	normalErr := errors.New("normal error")
	if !strategy.CanRecover(normalErr, state) {
		t.Error("Expected strategy to handle error with high error count")
	}

	// 测试 Recover
	ctx := context.Background()
	err := strategy.Recover(ctx, severeErr, state)
	if err != nil {
		t.Errorf("Expected successful recovery, got error: %v", err)
	}

	if !cleanupCalled {
		t.Error("Expected cleanup function to be called")
	}

	if !initCalled {
		t.Error("Expected init function to be called")
	}

	if state.Status != StatusPending {
		t.Errorf("Expected status to be reset to pending, got %v", state.Status)
	}

	if state.CompletedSteps != 0 {
		t.Errorf("Expected completed steps to be reset to 0, got %d", state.CompletedSteps)
	}
}

// TestCompositeRecoveryStrategy 测试复合恢复
func TestCompositeRecoveryStrategy(t *testing.T) {
	networkStrategy := &NetworkRecoveryStrategy{}
	fileStrategy := &FileRecoveryStrategy{CreateDirs: true}

	composite := NewCompositeRecoveryStrategy("composite", 100, networkStrategy, fileStrategy)

	// 测试网络错误恢复
	networkErr := errors.New("connection timeout")
	state := &ProcessState{ID: "test", RetryCount: 0}

	if !composite.CanRecover(networkErr, state) {
		t.Error("Expected composite strategy to handle network error")
	}

	ctx := context.Background()
	err := composite.Recover(ctx, networkErr, state)
	if err != nil {
		t.Errorf("Expected successful recovery, got error: %v", err)
	}

	// 测试文件错误恢复
	fileErr := errors.New("no such file or directory")
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "composite_test_dir")
	state.Data = map[string]interface{}{
		"directory_path": testDir,
	}

	if !composite.CanRecover(fileErr, state) {
		t.Error("Expected composite strategy to handle file error")
	}

	err = composite.Recover(ctx, fileErr, state)
	if err != nil {
		t.Errorf("Expected successful recovery, got error: %v", err)
	}
}

// TestRecoveryManager_RecoverProcess 测试进程恢复
func TestRecoveryManager_RecoverProcess(t *testing.T) {
	tempDir := t.TempDir()
	config := &RecoveryConfig{
		StateFile: filepath.Join(tempDir, "test_state.json"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 创建一个进程
	_ = rm.CreateProcess("test-process-5", "Test Process 5", 3)
	rm.UpdateProcess("test-process-5", func(state *ProcessState) {
		state.Status = StatusFailed
		state.ErrorCount = 1
	})

	// 尝试恢复
	ctx := context.Background()
	networkErr := errors.New("connection refused")
	err = rm.RecoverProcess(ctx, "test-process-5", networkErr)
	if err != nil {
		t.Errorf("Expected successful recovery, got error: %v", err)
	}

	// 验证进程状态
	recoveredProcess, err := rm.GetProcess("test-process-5")
	if err != nil {
		t.Fatalf("Failed to get recovered process: %v", err)
	}

	if recoveredProcess.Status != StatusRunning {
		t.Errorf("Expected status running after recovery, got %v", recoveredProcess.Status)
	}
}

// TestRecoveryManager_GetRecoveryStats 测试恢复统计
func TestRecoveryManager_GetRecoveryStats(t *testing.T) {
	tempDir := t.TempDir()
	config := &RecoveryConfig{
		StateFile: filepath.Join(tempDir, "test_state.json"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 创建不同状态的进程
	rm.CreateProcess("pending-1", "Pending Process", 3)

	process2 := rm.CreateProcess("running-1", "Running Process", 3)
	rm.UpdateProcess("running-1", func(state *ProcessState) {
		state.Status = StatusRunning
	})

	process3 := rm.CreateProcess("completed-1", "Completed Process", 3)
	rm.UpdateProcess("completed-1", func(state *ProcessState) {
		state.Status = StatusCompleted
	})

	process4 := rm.CreateProcess("failed-1", "Failed Process", 3)
	rm.UpdateProcess("failed-1", func(state *ProcessState) {
		state.Status = StatusFailed
		state.ErrorCount = 2
		state.RetryCount = 1
	})

	// 获取统计信息
	stats := rm.GetRecoveryStats()

	if stats["total_processes"] != 4 {
		t.Errorf("Expected 4 total processes, got %v", stats["total_processes"])
	}

	if stats["pending_processes"] != 1 {
		t.Errorf("Expected 1 pending process, got %v", stats["pending_processes"])
	}

	if stats["running_processes"] != 1 {
		t.Errorf("Expected 1 running process, got %v", stats["running_processes"])
	}

	if stats["completed_processes"] != 1 {
		t.Errorf("Expected 1 completed process, got %v", stats["completed_processes"])
	}

	if stats["failed_processes"] != 1 {
		t.Errorf("Expected 1 failed process, got %v", stats["failed_processes"])
	}

	if stats["total_errors"] != 2 {
		t.Errorf("Expected 2 total errors, got %v", stats["total_errors"])
	}

	if stats["total_retries"] != 1 {
		t.Errorf("Expected 1 total retry, got %v", stats["total_retries"])
	}
}

// TestRecoveryManager_CleanupOldStates 测试状态清理
func TestRecoveryManager_CleanupOldStates(t *testing.T) {
	tempDir := t.TempDir()
	config := &RecoveryConfig{
		StateFile:        filepath.Join(tempDir, "test_state.json"),
		BackupDir:        filepath.Join(tempDir, "backups"),
		CleanupOldStates: true,
		MaxStateAge:      1 * time.Millisecond, // 测试用的很短时间
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		t.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 创建旧的已完成进程
	_ = rm.CreateProcess("old-completed", "Old Completed Process", 3)
	rm.UpdateProcess("old-completed", func(state *ProcessState) {
		state.Status = StatusCompleted
		state.LastUpdate = time.Now().Add(-2 * time.Millisecond) // 使其变旧
	})

	// 创建最近的进程
	rm.CreateProcess("recent", "Recent Process", 3)

	// 等待旧进程超过最大年龄
	time.Sleep(5 * time.Millisecond)

	// 清理旧状态
	err = rm.CleanupOldStates()
	if err != nil {
		t.Fatalf("Failed to cleanup old states: %v", err)
	}

	// 验证旧进程已被移除
	_, err = rm.GetProcess("old-completed")
	if err == nil {
		t.Error("Expected old completed process to be removed")
	}

	// 验证最近的进程仍然存在
	_, err = rm.GetProcess("recent")
	if err != nil {
		t.Errorf("Expected recent process to still exist, got error: %v", err)
	}
}

// BenchmarkRecoveryManager_CreateProcess 基准测试进程创建
func BenchmarkRecoveryManager_CreateProcess(b *testing.B) {
	tempDir := b.TempDir()
	config := &RecoveryConfig{
		StateFile: filepath.Join(tempDir, "bench_state.json"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		b.Fatalf("Failed to create recovery manager: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processID := fmt.Sprintf("bench-process-%d", i)
		rm.CreateProcess(processID, "Benchmark Process", 5)
	}
}

// BenchmarkRecoveryManager_UpdateProcess 基准测试进程更新
func BenchmarkRecoveryManager_UpdateProcess(b *testing.B) {
	tempDir := b.TempDir()
	config := &RecoveryConfig{
		StateFile: filepath.Join(tempDir, "bench_state.json"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		b.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 创建用于基准测试的进程
	rm.CreateProcess("bench-process", "Benchmark Process", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.UpdateProcess("bench-process", func(state *ProcessState) {
			state.CompletedSteps = i % 100
			state.Progress = float64(i%100) / 100.0 * 100
		})
	}
}