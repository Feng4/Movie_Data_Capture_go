package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigManager manages configuration with hot reload and validation
type ConfigManager struct {
	mu           sync.RWMutex
	config       *Config
	configPath   string
	lastModTime  time.Time
	validators   []ConfigValidator
	changeNotify []ConfigChangeHandler
	watcher      *ConfigWatcher
}

// ConfigValidator validates configuration
type ConfigValidator interface {
	Validate(config *Config) error
}

// ConfigChangeHandler handles configuration changes
type ConfigChangeHandler interface {
	OnConfigChange(oldConfig, newConfig *Config) error
}

// ConfigWatcher watches for configuration file changes
type ConfigWatcher struct {
	manager  *ConfigManager
	stopChan chan bool
	running  bool
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) (*ConfigManager, error) {
	manager := &ConfigManager{
		configPath:   configPath,
		validators:   make([]ConfigValidator, 0),
		changeNotify: make([]ConfigChangeHandler, 0),
	}

	// Load initial configuration
	config, err := Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	manager.config = config

	// Get file modification time
	if stat, err := os.Stat(configPath); err == nil {
		manager.lastModTime = stat.ModTime()
	}

	// Apply environment variable overrides
	manager.applyEnvOverrides()

	// Validate configuration
	if err := manager.validateConfig(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return manager, nil
}

// GetConfig returns a copy of the current configuration
func (cm *ConfigManager) GetConfig() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a deep copy to prevent external modifications
	return cm.deepCopyConfig(cm.config)
}

// UpdateConfig updates the configuration and saves to file
func (cm *ConfigManager) UpdateConfig(newConfig *Config) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Validate new configuration
	for _, validator := range cm.validators {
		if err := validator.Validate(newConfig); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	oldConfig := cm.config
	cm.config = newConfig

	// Save to file
	if err := cm.saveConfig(); err != nil {
		cm.config = oldConfig // Rollback on save failure
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Notify change handlers
	for _, handler := range cm.changeNotify {
		if err := handler.OnConfigChange(oldConfig, newConfig); err != nil {
			// Log error but don't fail the update
			fmt.Printf("Config change handler error: %v\n", err)
		}
	}

	return nil
}

// ReloadConfig reloads configuration from file
func (cm *ConfigManager) ReloadConfig() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if file has been modified
	stat, err := os.Stat(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	if !stat.ModTime().After(cm.lastModTime) {
		return nil // No changes
	}

	// Load new configuration
	newConfig, err := Load(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// Apply environment overrides
	cm.applyEnvOverridesTo(newConfig)

	// Validate new configuration
	for _, validator := range cm.validators {
		if err := validator.Validate(newConfig); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	oldConfig := cm.config
	cm.config = newConfig
	cm.lastModTime = stat.ModTime()

	// Notify change handlers
	for _, handler := range cm.changeNotify {
		if err := handler.OnConfigChange(oldConfig, newConfig); err != nil {
			fmt.Printf("Config change handler error: %v\n", err)
		}
	}

	return nil
}

// AddValidator adds a configuration validator
func (cm *ConfigManager) AddValidator(validator ConfigValidator) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.validators = append(cm.validators, validator)
}

// AddChangeHandler adds a configuration change handler
func (cm *ConfigManager) AddChangeHandler(handler ConfigChangeHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.changeNotify = append(cm.changeNotify, handler)
}

// StartWatching starts watching for configuration file changes
func (cm *ConfigManager) StartWatching(interval time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.watcher != nil && cm.watcher.running {
		return // Already watching
	}

	cm.watcher = &ConfigWatcher{
		manager:  cm,
		stopChan: make(chan bool),
		running:  true,
	}

	go cm.watcher.watch(interval)
}

// StopWatching stops watching for configuration file changes
func (cm *ConfigManager) StopWatching() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.watcher != nil && cm.watcher.running {
		cm.watcher.stopChan <- true
		cm.watcher.running = false
	}
}

// GetConfigValue gets a specific configuration value by path
func (cm *ConfigManager) GetConfigValue(path string) (interface{}, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.getValueByPath(cm.config, path)
}

// SetConfigValue sets a specific configuration value by path
func (cm *ConfigManager) SetConfigValue(path string, value interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Create a copy of the config for modification
	newConfig := cm.deepCopyConfig(cm.config)

	// Set the value
	if err := cm.setValueByPath(newConfig, path, value); err != nil {
		return err
	}

	// Validate and update
	for _, validator := range cm.validators {
		if err := validator.Validate(newConfig); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	oldConfig := cm.config
	cm.config = newConfig

	// Save to file
	if err := cm.saveConfig(); err != nil {
		cm.config = oldConfig // Rollback
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Notify handlers
	for _, handler := range cm.changeNotify {
		if err := handler.OnConfigChange(oldConfig, newConfig); err != nil {
			fmt.Printf("Config change handler error: %v\n", err)
		}
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to current config
func (cm *ConfigManager) applyEnvOverrides() {
	cm.applyEnvOverridesTo(cm.config)
}

// applyEnvOverridesTo applies environment variable overrides to specified config
func (cm *ConfigManager) applyEnvOverridesTo(config *Config) {
	// Define environment variable mappings
	envMappings := map[string]string{
		"MDC_SOURCE_FOLDER":         "common.source_folder",
		"MDC_OUTPUT_FOLDER":         "common.success_output_folder",
		"MDC_FAILED_FOLDER":         "common.failed_output_folder",
		"MDC_PROXY_SWITCH":          "proxy.switch",
		"MDC_PROXY_URL":             "proxy.proxy",
		"MDC_PROXY_TIMEOUT":         "proxy.timeout",
		"MDC_DEBUG_MODE":            "debug_mode.switch",
		"MDC_TRANSLATE_SWITCH":      "translate.switch",
		"MDC_TRANSLATE_ENGINE":      "translate.engine",
		"MDC_TRANSLATE_KEY":         "translate.key",
		"MDC_WATERMARK_SWITCH":      "watermark.switch",
		"MDC_FACE_MODEL":            "face.locations_model",
		"MDC_FACE_ASPECT_RATIO":     "face.aspect_ratio",
		"MDC_MULTI_THREADING":       "common.multi_threading",
		"MDC_PRIORITY_WEBSITE":      "priority.website",
	}

	for envVar, configPath := range envMappings {
		if value := os.Getenv(envVar); value != "" {
			if err := cm.setValueByPath(config, configPath, value); err != nil {
				fmt.Printf("Failed to set env override %s: %v\n", envVar, err)
			}
		}
	}
}

// validateConfig validates the current configuration
func (cm *ConfigManager) validateConfig() error {
	for _, validator := range cm.validators {
		if err := validator.Validate(cm.config); err != nil {
			return err
		}
	}
	return nil
}

// saveConfig saves the current configuration to file
func (cm *ConfigManager) saveConfig() error {
	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create backup of existing config
	backupPath := cm.configPath + ".backup"
	if _, err := os.Stat(cm.configPath); err == nil {
		if err := cm.copyFile(cm.configPath, backupPath); err != nil {
			fmt.Printf("Failed to create config backup: %v\n", err)
		}
	}

	// Write new config
	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Update modification time
	if stat, err := os.Stat(cm.configPath); err == nil {
		cm.lastModTime = stat.ModTime()
	}

	return nil
}

// deepCopyConfig creates a deep copy of the configuration
func (cm *ConfigManager) deepCopyConfig(config *Config) *Config {
	// Use YAML marshal/unmarshal for deep copy
	data, err := yaml.Marshal(config)
	if err != nil {
		return config // Return original on error
	}

	newConfig := &Config{}
	if err := yaml.Unmarshal(data, newConfig); err != nil {
		return config // Return original on error
	}

	return newConfig
}

// getValueByPath gets a value from config using dot notation path
func (cm *ConfigManager) getValueByPath(config *Config, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	value := reflect.ValueOf(config).Elem()

	for _, part := range parts {
		if !value.IsValid() {
			return nil, fmt.Errorf("invalid path: %s", path)
		}

		if value.Kind() == reflect.Ptr {
			value = value.Elem()
		}

		if value.Kind() != reflect.Struct {
			return nil, fmt.Errorf("cannot traverse non-struct at path: %s", path)
		}

		// Find field by name or yaml tag
		field := cm.findField(value, part)
		if !field.IsValid() {
			return nil, fmt.Errorf("field not found: %s", part)
		}

		value = field
	}

	return value.Interface(), nil
}

// setValueByPath sets a value in config using dot notation path
func (cm *ConfigManager) setValueByPath(config *Config, path string, newValue interface{}) error {
	parts := strings.Split(path, ".")
	value := reflect.ValueOf(config).Elem()

	// Navigate to the parent of the target field
	for i, part := range parts[:len(parts)-1] {
		if !value.IsValid() {
			return fmt.Errorf("invalid path at %s", strings.Join(parts[:i+1], "."))
		}

		if value.Kind() == reflect.Ptr {
			value = value.Elem()
		}

		if value.Kind() != reflect.Struct {
			return fmt.Errorf("cannot traverse non-struct at %s", strings.Join(parts[:i+1], "."))
		}

		field := cm.findField(value, part)
		if !field.IsValid() {
			return fmt.Errorf("field not found: %s", part)
		}

		value = field
	}

	// Set the target field
	lastPart := parts[len(parts)-1]
	targetField := cm.findField(value, lastPart)
	if !targetField.IsValid() {
		return fmt.Errorf("target field not found: %s", lastPart)
	}

	if !targetField.CanSet() {
		return fmt.Errorf("cannot set field: %s", lastPart)
	}

	// Convert value to appropriate type
	convertedValue, err := cm.convertValue(newValue, targetField.Type())
	if err != nil {
		return fmt.Errorf("failed to convert value for %s: %w", lastPart, err)
	}

	targetField.Set(convertedValue)
	return nil
}

// findField finds a struct field by name or yaml tag
func (cm *ConfigManager) findField(structValue reflect.Value, name string) reflect.Value {
	structType := structValue.Type()

	for i := 0; i < structValue.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)

		// Check field name
		if strings.EqualFold(field.Name, name) {
			return fieldValue
		}

		// Check yaml tag
		if yamlTag := field.Tag.Get("yaml"); yamlTag != "" {
			tagName := strings.Split(yamlTag, ",")[0]
			if strings.EqualFold(tagName, name) {
				return fieldValue
			}
		}
	}

	return reflect.Value{}
}

// convertValue converts a value to the target type
func (cm *ConfigManager) convertValue(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	valueStr := fmt.Sprintf("%v", value)

	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(valueStr), nil
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(valueStr)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(boolVal), nil
	case reflect.Int:
		intVal, err := strconv.Atoi(valueStr)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(intVal), nil
	case reflect.Float64:
		floatVal, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(floatVal), nil
	default:
		// Try direct assignment
		valueReflect := reflect.ValueOf(value)
		if valueReflect.Type().ConvertibleTo(targetType) {
			return valueReflect.Convert(targetType), nil
		}
		return reflect.Value{}, fmt.Errorf("cannot convert %T to %s", value, targetType)
	}
}

// copyFile copies a file from src to dst
func (cm *ConfigManager) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// watch watches for configuration file changes
func (cw *ConfigWatcher) watch(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-cw.stopChan:
			return
		case <-ticker.C:
			if err := cw.manager.ReloadConfig(); err != nil {
				fmt.Printf("Failed to reload config: %v\n", err)
			}
		}
	}
}