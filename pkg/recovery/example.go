package recovery

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"movie-data-capture/pkg/retry"
)

// Example 演示如何在实际应用程序中使用恢复系统
func Example() {
	// 创建恢复管理器
	config := &RecoveryConfig{
		StateFile:          "app_recovery_state.json",
		BackupDir:          "recovery_backups",
		SaveInterval:       30 * time.Second,
		MaxBackups:         5,
		AutoRecovery:       true,
		RecoveryTimeout:    2 * time.Minute,
		CleanupOldStates:   true,
		MaxStateAge:        24 * time.Hour,
		CompressionEnabled: false,
		EncryptionEnabled:  false,
	}

	rm, err := NewRecoveryManager(config)
	if err != nil {
		log.Fatalf("Failed to create recovery manager: %v", err)
	}

	// 注册自定义恢复策略
	rm.RegisterStrategy(&CustomFileDownloadRecoveryStrategy{})
	rm.RegisterStrategy(&CustomDatabaseRecoveryStrategy{})

	// 示例1：带恢复的文件处理
	processFileWithRecovery(rm, "example_file.txt")

	// 示例2：带恢复的网络操作
	downloadFileWithRecovery(rm, "https://example.com/file.zip", "downloaded_file.zip")

	// 示例3：带恢复的数据库操作
	databaseOperationWithRecovery(rm, "user_data_sync")

	// 示例4：带恢复的批处理
	batchProcessWithRecovery(rm, []string{"file1.txt", "file2.txt", "file3.txt"})

	// 启动自动恢复监控
	startAutoRecoveryMonitoring(rm)

	// 定期清理旧状态
	startPeriodicCleanup(rm)

	// 打印恢复统计信息
	printRecoveryStats(rm)
}

// processFileWithRecovery 演示带恢复的文件处理
func processFileWithRecovery(rm *RecoveryManager, filename string) {
	processID := fmt.Sprintf("file_process_%s", filename)
	_ = rm.CreateProcess(processID, "File Processing", 4)

	_ = context.Background()

	// 步骤1：读取文件
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusRunning
		state.CurrentStep = "reading_file"
		state.Data["file_path"] = filename
	})

	err := performWithRecovery(rm, processID, func() error {
		return readFile(filename)
	})

	if err != nil {
		log.Printf("Failed to read file %s: %v", filename, err)
		return
	}

	rm.CreateCheckpoint(processID, "file_read", map[string]interface{}{
		"file_path": filename,
		"timestamp": time.Now(),
	})

	// 步骤2：处理文件内容
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "processing_content"
		state.CompletedSteps = 1
		state.Progress = 25.0
	})

	err = performWithRecovery(rm, processID, func() error {
		return processFileContent(filename)
	})

	if err != nil {
		log.Printf("Failed to process file content %s: %v", filename, err)
		return
	}

	// 步骤3：验证处理后的数据
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "validating_data"
		state.CompletedSteps = 2
		state.Progress = 50.0
	})

	err = performWithRecovery(rm, processID, func() error {
		return validateProcessedData(filename)
	})

	if err != nil {
		log.Printf("Failed to validate data %s: %v", filename, err)
		return
	}

	// 步骤4：保存结果
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "saving_results"
		state.CompletedSteps = 3
		state.Progress = 75.0
	})

	err = performWithRecovery(rm, processID, func() error {
		return saveResults(filename)
	})

	if err != nil {
		log.Printf("Failed to save results %s: %v", filename, err)
		return
	}

	// 完成处理
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusCompleted
		state.CompletedSteps = 4
		state.Progress = 100.0
		state.CurrentStep = "completed"
	})

	log.Printf("Successfully processed file: %s", filename)
}

// downloadFileWithRecovery 演示带恢复的网络操作
func downloadFileWithRecovery(rm *RecoveryManager, url, filename string) {
	processID := fmt.Sprintf("download_%s", filename)
	_ = rm.CreateProcess(processID, "File Download", 3)

	_ = context.Background()

	// 步骤1：验证URL
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusRunning
		state.CurrentStep = "validating_url"
		state.Data["url"] = url
		state.Data["filename"] = filename
	})

	err := performWithRecovery(rm, processID, func() error {
		return validateURL(url)
	})

	if err != nil {
		log.Printf("Failed to validate URL %s: %v", url, err)
		return
	}

	// 步骤2：下载文件
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "downloading_file"
		state.CompletedSteps = 1
		state.Progress = 33.3
	})

	err = performWithRecovery(rm, processID, func() error {
		return downloadFile(url, filename)
	})

	if err != nil {
		log.Printf("Failed to download file %s: %v", url, err)
		return
	}

	rm.CreateCheckpoint(processID, "file_downloaded", map[string]interface{}{
		"url":      url,
		"filename": filename,
		"size":     getFileSize(filename),
	})

	// 步骤3：验证下载
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "verifying_download"
		state.CompletedSteps = 2
		state.Progress = 66.6
	})

	err = performWithRecovery(rm, processID, func() error {
		return verifyDownload(filename)
	})

	if err != nil {
		log.Printf("Failed to verify download %s: %v", filename, err)
		return
	}

	// 完成处理
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusCompleted
		state.CompletedSteps = 3
		state.Progress = 100.0
		state.CurrentStep = "completed"
	})

	log.Printf("Successfully downloaded file: %s", filename)
}

// databaseOperationWithRecovery 演示带恢复功能的数据库操作
func databaseOperationWithRecovery(rm *RecoveryManager, operation string) {
	processID := fmt.Sprintf("db_op_%s", operation)
	_ = rm.CreateProcess(processID, "Database Operation", 5)

	_ = context.Background()

	// 步骤 1: 连接数据库
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusRunning
		state.CurrentStep = "connecting_database"
		state.Data["operation"] = operation
	})

	err := performWithRecovery(rm, processID, func() error {
		return connectDatabase()
	})

	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return
	}

	// 步骤 2: 开始事务
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "begin_transaction"
		state.CompletedSteps = 1
		state.Progress = 20.0
	})

	err = performWithRecovery(rm, processID, func() error {
		return beginTransaction()
	})

	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return
	}

	rm.CreateCheckpoint(processID, "transaction_started", map[string]interface{}{
		"operation": operation,
		"timestamp": time.Now(),
	})

	// 步骤 3: 执行操作
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "executing_operation"
		state.CompletedSteps = 2
		state.Progress = 40.0
	})

	err = performWithRecovery(rm, processID, func() error {
		return executeOperation(operation)
	})

	if err != nil {
		log.Printf("Failed to execute operation %s: %v", operation, err)
		// 回滚事务
		rollbackTransaction()
		return
	}

	// 步骤 4: 验证结果
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "validating_results"
		state.CompletedSteps = 3
		state.Progress = 60.0
	})

	err = performWithRecovery(rm, processID, func() error {
		return validateResults(operation)
	})

	if err != nil {
		log.Printf("Failed to validate results %s: %v", operation, err)
		rollbackTransaction()
		return
	}

	// 步骤 5: 提交事务
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.CurrentStep = "committing_transaction"
		state.CompletedSteps = 4
		state.Progress = 80.0
	})

	err = performWithRecovery(rm, processID, func() error {
		return commitTransaction()
	})

	if err != nil {
		log.Printf("Failed to commit transaction %s: %v", operation, err)
		rollbackTransaction()
		return
	}

	// Complete process
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusCompleted
		state.CompletedSteps = 5
		state.Progress = 100.0
		state.CurrentStep = "completed"
	})

	log.Printf("Successfully completed database operation: %s", operation)
}

// batchProcessWithRecovery 演示带恢复功能的批处理
func batchProcessWithRecovery(rm *RecoveryManager, files []string) {
	processID := "batch_process"
	_ = rm.CreateProcess(processID, "Batch Processing", len(files))

	_ = context.Background()

	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusRunning
		state.Data["files"] = files
		state.Data["total_files"] = len(files)
	})

	for i, file := range files {
		rm.UpdateProcess(processID, func(state *ProcessState) {
			state.CurrentStep = fmt.Sprintf("processing_file_%d", i+1)
			state.CompletedSteps = i
			state.Progress = float64(i) / float64(len(files)) * 100
			state.Data["current_file"] = file
		})

		err := performWithRecovery(rm, processID, func() error {
			return processFile(file)
		})

		if err != nil {
			log.Printf("Failed to process file %s in batch: %v", file, err)
			// 继续处理下一个文件而不是让整个批处理失败
			rm.UpdateProcess(processID, func(state *ProcessState) {
				state.ErrorCount++
				state.LastError = err.Error()
			})
			continue
		}

		// 为每个已处理的文件创建检查点
		rm.CreateCheckpoint(processID, fmt.Sprintf("file_%d_processed", i+1), map[string]interface{}{
			"file_index": i,
			"file_name":  file,
			"timestamp":  time.Now(),
		})
	}

	// 完成批处理
	rm.UpdateProcess(processID, func(state *ProcessState) {
		state.Status = StatusCompleted
		state.CompletedSteps = len(files)
		state.Progress = 100.0
		state.CurrentStep = "batch_completed"
	})

	log.Printf("Batch processing completed. Processed %d files", len(files))
}

// performWithRecovery 包装操作执行并提供恢复功能
func performWithRecovery(rm *RecoveryManager, processID string, operation func() error) error {
	ctx := context.Background()

	err := operation()
	if err != nil {
		// 更新进程错误信息
		rm.UpdateProcess(processID, func(state *ProcessState) {
			state.Status = StatusFailed
			state.ErrorCount++
			state.LastError = err.Error()
		})

		// 尝试恢复
		recoveryErr := rm.RecoverProcess(ctx, processID, err)
		if recoveryErr != nil {
			return fmt.Errorf("operation failed and recovery failed: %w", recoveryErr)
		}

		// 恢复成功后重试操作
		return operation()
	}

	return nil
}

// startAutoRecoveryMonitoring 启动自动恢复监控
func startAutoRecoveryMonitoring(rm *RecoveryManager) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx := context.Background()
				if err := rm.AutoRecover(ctx); err != nil {
					log.Printf("Auto recovery failed: %v", err)
				}
			}
		}
	}()
}

// startPeriodicCleanup 启动旧状态的定期清理
func startPeriodicCleanup(rm *RecoveryManager) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := rm.CleanupOldStates(); err != nil {
					log.Printf("Cleanup failed: %v", err)
				}
			}
		}
	}()
}

// printRecoveryStats 打印恢复统计信息
func printRecoveryStats(rm *RecoveryManager) {
	stats := rm.GetRecoveryStats()
	log.Printf("Recovery Statistics:")
	log.Printf("  Total Processes: %v", stats["total_processes"])
	log.Printf("  Running: %v", stats["running_processes"])
	log.Printf("  Completed: %v", stats["completed_processes"])
	log.Printf("  Failed: %v", stats["failed_processes"])
	log.Printf("  Total Errors: %v", stats["total_errors"])
	log.Printf("  Total Retries: %v", stats["total_retries"])
}

// 自定义恢复策略

// CustomFileDownloadRecoveryStrategy 处理文件下载失败
type CustomFileDownloadRecoveryStrategy struct{}

func (cfdrs *CustomFileDownloadRecoveryStrategy) GetName() string {
	return "custom_file_download_recovery"
}

func (cfdrs *CustomFileDownloadRecoveryStrategy) GetPriority() int {
	return 85
}

func (cfdrs *CustomFileDownloadRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	if err == nil {
		return false
	}

	// 检查这是否是下载相关的进程
	if url, ok := state.Data["url"].(string); ok && url != "" {
		errorStr := err.Error()
		downloadErrors := []string{
			"connection reset",
			"timeout",
			"temporary failure",
			"service unavailable",
			"partial download",
		}

		for _, downloadErr := range downloadErrors {
			if contains(errorStr, downloadErr) {
				return true
			}
		}
	}

	return false
}

func (cfdrs *CustomFileDownloadRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	url, _ := state.Data["url"].(string)
	filename, _ := state.Data["filename"].(string)

	// 检查部分文件是否存在并恢复下载
	if fileExists(filename) {
		log.Printf("Resuming download for %s", filename)
		return resumeDownload(url, filename)
	}

	// 否则，使用重试逻辑重新开始下载
	retryConfig := retry.NetworkConfig()
	return retry.RetryWithContext(ctx, func(ctx context.Context) error {
		return downloadFile(url, filename)
	}, retryConfig)
}

// CustomDatabaseRecoveryStrategy 处理数据库操作失败
type CustomDatabaseRecoveryStrategy struct{}

func (cdrs *CustomDatabaseRecoveryStrategy) GetName() string {
	return "custom_database_recovery"
}

func (cdrs *CustomDatabaseRecoveryStrategy) GetPriority() int {
	return 75
}

func (cdrs *CustomDatabaseRecoveryStrategy) CanRecover(err error, state *ProcessState) bool {
	if err == nil {
		return false
	}

	// 检查这是否是数据库相关的进程
	if operation, ok := state.Data["operation"].(string); ok && operation != "" {
		errorStr := err.Error()
		dbErrors := []string{
			"connection lost",
			"deadlock",
			"timeout",
			"lock wait timeout",
			"connection refused",
		}

		for _, dbErr := range dbErrors {
			if contains(errorStr, dbErr) {
				return true
			}
		}
	}

	return false
}

func (cdrs *CustomDatabaseRecoveryStrategy) Recover(ctx context.Context, err error, state *ProcessState) error {
	operation, _ := state.Data["operation"].(string)

	// 回滚任何待处理的事务
	if err := rollbackTransaction(); err != nil {
		log.Printf("Failed to rollback transaction: %v", err)
	}

	// 重新连接数据库
	if err := reconnectDatabase(); err != nil {
		return fmt.Errorf("failed to reconnect to database: %w", err)
	}

	// 重试操作
	retryConfig := &retry.Config{
		MaxAttempts:     3,
		InitialDelay:    2 * time.Second,
		MaxDelay:        10 * time.Second,
		BackoffStrategy: retry.ExponentialBackoff,
		Jitter:          true,
		RetryIf:         retry.DefaultRetryIf,
	}

	return retry.RetryWithContext(ctx, func(ctx context.Context) error {
		if err := beginTransaction(); err != nil {
			return err
		}
		if err := executeOperation(operation); err != nil {
			rollbackTransaction()
			return err
		}
		return commitTransaction()
	}, retryConfig)
}

// 示例函数的虚拟实现

func readFile(filename string) error {
	_, err := os.ReadFile(filename)
	return err
}

func processFileContent(filename string) error {
	// 模拟处理
	time.Sleep(100 * time.Millisecond)
	return nil
}

func validateProcessedData(filename string) error {
	// 模拟验证
	return nil
}

func saveResults(filename string) error {
	// 模拟保存
	return nil
}

func validateURL(url string) error {
	resp, err := http.Head(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func downloadFile(url, filename string) error {
	// 模拟下载
	time.Sleep(200 * time.Millisecond)
	return nil
}

func resumeDownload(url, filename string) error {
	// 模拟恢复下载
	return nil
}

func verifyDownload(filename string) error {
	// 模拟验证
	return nil
}

func getFileSize(filename string) int64 {
	info, err := os.Stat(filename)
	if err != nil {
		return 0
	}
	return info.Size()
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func connectDatabase() error {
	// 模拟数据库连接
	return nil
}

func reconnectDatabase() error {
	// 模拟数据库重连
	return nil
}

func beginTransaction() error {
	// 模拟事务开始
	return nil
}

func executeOperation(operation string) error {
	// 模拟操作执行
	time.Sleep(50 * time.Millisecond)
	return nil
}

func validateResults(operation string) error {
	// 模拟结果验证
	return nil
}

func commitTransaction() error {
	// 模拟事务提交
	return nil
}

func rollbackTransaction() error {
	// 模拟事务回滚
	return nil
}

func processFile(filename string) error {
	// 模拟文件处理
	time.Sleep(100 * time.Millisecond)
	return nil
}

// contains 是一个辅助函数，用于检查字符串是否包含子字符串
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