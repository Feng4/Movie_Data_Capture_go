package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockConfigValidator for testing
type MockConfigValidator struct {
	validateFunc func(*Config) error
}

func (m *MockConfigValidator) Validate(config *Config) error {
	if m.validateFunc != nil {
		return m.validateFunc(config)
	}
	return nil
}

// MockConfigChangeHandler for testing
type MockConfigChangeHandler struct {
	onChangeFunc func(*Config, *Config) error
	changeCount  int
}

func (m *MockConfigChangeHandler) OnConfigChange(oldConfig, newConfig *Config) error {
	m.changeCount++
	if m.onChangeFunc != nil {
		return m.onChangeFunc(oldConfig, newConfig)
	}
	return nil
}

func TestNewConfigManager(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create a basic config file
	configContent := `
common:
  main_mode: 1
  source_folder: "./"
  success_output_folder: "output"
  failed_output_folder: "failed"
proxy:
  switch: false
  timeout: 5
face:
  aspect_ratio: 2.12
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	// Test creating config manager
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	if manager == nil {
		t.Fatal("Config manager should not be nil")
	}
	
	// Test getting config
	config := manager.GetConfig()
	if config == nil {
		t.Fatal("Config should not be nil")
	}
	
	if config.Common.MainMode != 1 {
		t.Errorf("Expected main_mode 1, got %d", config.Common.MainMode)
	}
}

func TestConfigManager_UpdateConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	// Add mock change handler
	mockHandler := &MockConfigChangeHandler{}
	manager.AddChangeHandler(mockHandler)
	
	// Update config
	newConfig := manager.GetConfig()
	newConfig.Common.MainMode = 2
	newConfig.Common.MultiThreading = 4
	
	err = manager.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}
	
	// Verify config was updated
	updatedConfig := manager.GetConfig()
	if updatedConfig.Common.MainMode != 2 {
		t.Errorf("Expected main_mode 2, got %d", updatedConfig.Common.MainMode)
	}
	
	if updatedConfig.Common.MultiThreading != 4 {
		t.Errorf("Expected multi_threading 4, got %d", updatedConfig.Common.MultiThreading)
	}
	
	// Verify change handler was called
	if mockHandler.changeCount != 1 {
		t.Errorf("Expected change handler to be called once, got %d", mockHandler.changeCount)
	}
}

func TestConfigManager_Validation(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	// Add validator that rejects negative values
	validator := &MockConfigValidator{
		validateFunc: func(config *Config) error {
			if config.Common.MultiThreading < 0 {
				return fmt.Errorf("multi_threading cannot be negative")
			}
			return nil
		},
	}
	manager.AddValidator(validator)
	
	// Try to update with invalid config
	invalidConfig := manager.GetConfig()
	invalidConfig.Common.MultiThreading = -1
	
	err = manager.UpdateConfig(invalidConfig)
	if err == nil {
		t.Error("Expected validation error for negative multi_threading")
	}
	
	// Verify config was not updated
	currentConfig := manager.GetConfig()
	if currentConfig.Common.MultiThreading < 0 {
		t.Error("Config should not have been updated with invalid value")
	}
}

func TestConfigManager_GetSetConfigValue(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	// Test getting config value
	value, err := manager.GetConfigValue("common.main_mode")
	if err != nil {
		t.Fatalf("Failed to get config value: %v", err)
	}
	
	if value != 1 {
		t.Errorf("Expected main_mode 1, got %v", value)
	}
	
	// Test setting config value
	err = manager.SetConfigValue("common.main_mode", 3)
	if err != nil {
		t.Fatalf("Failed to set config value: %v", err)
	}
	
	// Verify value was set
	value, err = manager.GetConfigValue("common.main_mode")
	if err != nil {
		t.Fatalf("Failed to get updated config value: %v", err)
	}
	
	if value != 3 {
		t.Errorf("Expected main_mode 3, got %v", value)
	}
	
	// Test setting nested value
	err = manager.SetConfigValue("face.aspect_ratio", 2.5)
	if err != nil {
		t.Fatalf("Failed to set nested config value: %v", err)
	}
	
	value, err = manager.GetConfigValue("face.aspect_ratio")
	if err != nil {
		t.Fatalf("Failed to get nested config value: %v", err)
	}
	
	if value != 2.5 {
		t.Errorf("Expected aspect_ratio 2.5, got %v", value)
	}
}

func TestConfigManager_EnvOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Set environment variables
	os.Setenv("MDC_SOURCE_FOLDER", "/test/source")
	os.Setenv("MDC_MULTI_THREADING", "8")
	os.Setenv("MDC_DEBUG_MODE", "true")
	defer func() {
		os.Unsetenv("MDC_SOURCE_FOLDER")
		os.Unsetenv("MDC_MULTI_THREADING")
		os.Unsetenv("MDC_DEBUG_MODE")
	}()
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	config := manager.GetConfig()
	
	// Verify environment overrides were applied
	if config.Common.SourceFolder != "/test/source" {
		t.Errorf("Expected source folder '/test/source', got '%s'", config.Common.SourceFolder)
	}
	
	if config.Common.MultiThreading != 8 {
		t.Errorf("Expected multi_threading 8, got %d", config.Common.MultiThreading)
	}
	
	if !config.DebugMode.Switch {
		t.Error("Expected debug mode to be enabled")
	}
}

func TestConfigManager_ReloadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	// Add change handler
	mockHandler := &MockConfigChangeHandler{}
	manager.AddChangeHandler(mockHandler)
	
	// Modify config file externally
	time.Sleep(10 * time.Millisecond) // Ensure different modification time
	newConfigContent := `
common:
  main_mode: 2
  source_folder: "./modified"
  success_output_folder: "output"
  failed_output_folder: "failed"
  multi_threading: 6
proxy:
  switch: true
  timeout: 10
face:
  aspect_ratio: 3.0
`
	err = os.WriteFile(configPath, []byte(newConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to modify config file: %v", err)
	}
	
	// Reload config
	err = manager.ReloadConfig()
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}
	
	// Verify config was reloaded
	config := manager.GetConfig()
	if config.Common.MainMode != 2 {
		t.Errorf("Expected main_mode 2, got %d", config.Common.MainMode)
	}
	
	if config.Common.SourceFolder != "./modified" {
		t.Errorf("Expected source folder './modified', got '%s'", config.Common.SourceFolder)
	}
	
	if config.Common.MultiThreading != 6 {
		t.Errorf("Expected multi_threading 6, got %d", config.Common.MultiThreading)
	}
	
	// Verify change handler was called
	if mockHandler.changeCount != 1 {
		t.Errorf("Expected change handler to be called once, got %d", mockHandler.changeCount)
	}
}

func TestConfigManager_WatchingDisabled(t *testing.T) {
	// This test verifies that watching can be started and stopped
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	// Start watching
	manager.StartWatching(100 * time.Millisecond)
	
	// Verify watcher is running
	if manager.watcher == nil || !manager.watcher.running {
		t.Error("Watcher should be running")
	}
	
	// Stop watching
	manager.StopWatching()
	
	// Give some time for the watcher to stop
	time.Sleep(150 * time.Millisecond)
	
	// Verify watcher is stopped
	if manager.watcher.running {
		t.Error("Watcher should be stopped")
	}
}

func TestConfigManager_DeepCopy(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	// Get config copy
	config1 := manager.GetConfig()
	config2 := manager.GetConfig()
	
	// Modify one copy
	config1.Common.MainMode = 99
	
	// Verify the other copy is not affected
	if config2.Common.MainMode == 99 {
		t.Error("Config copies should be independent")
	}
	
	// Verify original config in manager is not affected
	originalConfig := manager.GetConfig()
	if originalConfig.Common.MainMode == 99 {
		t.Error("Original config should not be affected by modifications to copies")
	}
}

func TestConfigManager_InvalidPath(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Create initial config
	_, err := createDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}
	
	manager, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	
	// Test getting invalid path
	_, err = manager.GetConfigValue("invalid.path.here")
	if err == nil {
		t.Error("Expected error for invalid config path")
	}
	
	// Test setting invalid path
	err = manager.SetConfigValue("invalid.path.here", "value")
	if err == nil {
		t.Error("Expected error for setting invalid config path")
	}
}