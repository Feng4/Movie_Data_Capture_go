package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// LoggingConfigHandler 记录配置变更
type LoggingConfigHandler struct {
	logger *log.Logger
}

// NewLoggingConfigHandler 创建一个新的日志配置处理器
func NewLoggingConfigHandler(logger *log.Logger) *LoggingConfigHandler {
	if logger == nil {
		logger = log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)
	}
	return &LoggingConfigHandler{
		logger: logger,
	}
}

// OnConfigChange 记录配置变更
func (h *LoggingConfigHandler) OnConfigChange(oldConfig, newConfig *Config) error {
	h.logger.Printf("Configuration updated at %s", time.Now().Format(time.RFC3339))
	
	// 记录具体变更
	changes := h.detectChanges(oldConfig, newConfig)
	for _, change := range changes {
		h.logger.Printf("  %s", change)
	}
	
	return nil
}

// detectChanges 检测并格式化配置变更
func (h *LoggingConfigHandler) detectChanges(oldConfig, newConfig *Config) []string {
	var changes []string
	
	// 检查通用配置变更
	if oldConfig.Common.SourceFolder != newConfig.Common.SourceFolder {
		changes = append(changes, fmt.Sprintf("Source folder: %s -> %s", oldConfig.Common.SourceFolder, newConfig.Common.SourceFolder))
	}
	
	if oldConfig.Common.SuccessOutputFolder != newConfig.Common.SuccessOutputFolder {
		changes = append(changes, fmt.Sprintf("Success output folder: %s -> %s", oldConfig.Common.SuccessOutputFolder, newConfig.Common.SuccessOutputFolder))
	}
	
	if oldConfig.Common.MultiThreading != newConfig.Common.MultiThreading {
		changes = append(changes, fmt.Sprintf("Multi-threading: %d -> %d", oldConfig.Common.MultiThreading, newConfig.Common.MultiThreading))
	}
	
	// 检查代理配置变更
	if oldConfig.Proxy.Switch != newConfig.Proxy.Switch {
		changes = append(changes, fmt.Sprintf("Proxy enabled: %t -> %t", oldConfig.Proxy.Switch, newConfig.Proxy.Switch))
	}
	
	if oldConfig.Proxy.Proxy != newConfig.Proxy.Proxy {
		changes = append(changes, fmt.Sprintf("Proxy URL: %s -> %s", oldConfig.Proxy.Proxy, newConfig.Proxy.Proxy))
	}
	
	// 检查调试模式变更
	if oldConfig.DebugMode.Switch != newConfig.DebugMode.Switch {
		changes = append(changes, fmt.Sprintf("Debug mode: %t -> %t", oldConfig.DebugMode.Switch, newConfig.DebugMode.Switch))
	}
	
	// 检查翻译配置变更
	if oldConfig.Translate.Switch != newConfig.Translate.Switch {
		changes = append(changes, fmt.Sprintf("Translation enabled: %t -> %t", oldConfig.Translate.Switch, newConfig.Translate.Switch))
	}
	
	if oldConfig.Translate.Engine != newConfig.Translate.Engine {
		changes = append(changes, fmt.Sprintf("Translation engine: %s -> %s", oldConfig.Translate.Engine, newConfig.Translate.Engine))
	}
	
	// 检查人脸配置变更
	if oldConfig.Face.LocationsModel != newConfig.Face.LocationsModel {
		changes = append(changes, fmt.Sprintf("Face detection model: %s -> %s", oldConfig.Face.LocationsModel, newConfig.Face.LocationsModel))
	}
	
	if oldConfig.Face.AspectRatio != newConfig.Face.AspectRatio {
		changes = append(changes, fmt.Sprintf("Face aspect ratio: %.2f -> %.2f", oldConfig.Face.AspectRatio, newConfig.Face.AspectRatio))
	}
	
	// 检查水印配置变更
	if oldConfig.Watermark.Switch != newConfig.Watermark.Switch {
		changes = append(changes, fmt.Sprintf("Watermark enabled: %t -> %t", oldConfig.Watermark.Switch, newConfig.Watermark.Switch))
	}
	
	if len(changes) == 0 {
		changes = append(changes, "No significant changes detected")
	}
	
	return changes
}

// DirectoryConfigHandler 处理目录相关的配置变更
type DirectoryConfigHandler struct {
	createDirectories bool
}

// NewDirectoryConfigHandler 创建一个新的目录配置处理器
func NewDirectoryConfigHandler(createDirectories bool) *DirectoryConfigHandler {
	return &DirectoryConfigHandler{
		createDirectories: createDirectories,
	}
}

// OnConfigChange 处理目录相关的配置变更
func (h *DirectoryConfigHandler) OnConfigChange(oldConfig, newConfig *Config) error {
	// 处理输出目录变更
	if oldConfig.Common.SuccessOutputFolder != newConfig.Common.SuccessOutputFolder {
		if err := h.handleDirectoryChange("success output", oldConfig.Common.SuccessOutputFolder, newConfig.Common.SuccessOutputFolder); err != nil {
			return err
		}
	}
	
	if oldConfig.Common.FailedOutputFolder != newConfig.Common.FailedOutputFolder {
		if err := h.handleDirectoryChange("failed output", oldConfig.Common.FailedOutputFolder, newConfig.Common.FailedOutputFolder); err != nil {
			return err
		}
	}
	
	// 处理额外封面目录变更
	if oldConfig.Extrafanart.ExtrafanartFolder != newConfig.Extrafanart.ExtrafanartFolder {
		if err := h.handleDirectoryChange("extrafanart", oldConfig.Extrafanart.ExtrafanartFolder, newConfig.Extrafanart.ExtrafanartFolder); err != nil {
			return err
		}
	}
	
	return nil
}

// handleDirectoryChange 处理单个目录变更
func (h *DirectoryConfigHandler) handleDirectoryChange(dirType, oldPath, newPath string) error {
	if newPath == "" {
		return nil
	}
	
	// 将相对路径转换为绝对路径
	if !filepath.IsAbs(newPath) {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		newPath = filepath.Join(wd, newPath)
	}
	
	// 检查目录是否存在
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		if h.createDirectories {
			if err := os.MkdirAll(newPath, 0755); err != nil {
				return fmt.Errorf("failed to create %s directory %s: %w", dirType, newPath, err)
			}
			log.Printf("Created %s directory: %s", dirType, newPath)
		} else {
			log.Printf("Warning: %s directory does not exist: %s", dirType, newPath)
		}
	}
	
	return nil
}

// CacheConfigHandler 处理缓存相关的配置变更
type CacheConfigHandler struct {
	cacheManager interface{} // 缓存管理器接口
}

// NewCacheConfigHandler 创建一个新的缓存配置处理器
func NewCacheConfigHandler(cacheManager interface{}) *CacheConfigHandler {
	return &CacheConfigHandler{
		cacheManager: cacheManager,
	}
}

// OnConfigChange 处理缓存相关的配置变更
func (h *CacheConfigHandler) OnConfigChange(oldConfig, newConfig *Config) error {
	// 如果某些设置发生变更则清除缓存
	clearCache := false
	
	// 如果代理设置发生变更则清除缓存
	if oldConfig.Proxy.Switch != newConfig.Proxy.Switch ||
		oldConfig.Proxy.Proxy != newConfig.Proxy.Proxy ||
		oldConfig.Proxy.Type != newConfig.Proxy.Type {
		clearCache = true
	}
	
	// 如果翻译设置发生变更则清除缓存
	if oldConfig.Translate.Switch != newConfig.Translate.Switch ||
		oldConfig.Translate.Engine != newConfig.Translate.Engine ||
		oldConfig.Translate.TargetLang != newConfig.Translate.TargetLang {
		clearCache = true
	}
	
	// 如果优先网站发生变更则清除缓存
	if oldConfig.Priority.Website != newConfig.Priority.Website {
		clearCache = true
	}
	
	if clearCache {
		log.Printf("检测到配置变更，清除相关缓存")
		// 这里你可以调用缓存管理器方法来清除缓存
		// 这里保留为接口形式，因为实际的缓存实现
		// 取决于使用的具体缓存管理器
	}
	
	return nil
}

// SecurityConfigHandler 处理安全相关的配置变更
type SecurityConfigHandler struct{}

// NewSecurityConfigHandler 创建一个新的安全配置处理器
func NewSecurityConfigHandler() *SecurityConfigHandler {
	return &SecurityConfigHandler{}
}

// OnConfigChange 处理安全相关的配置变更
func (h *SecurityConfigHandler) OnConfigChange(oldConfig, newConfig *Config) error {
	// 记录安全敏感的变更
	if oldConfig.Proxy.Switch != newConfig.Proxy.Switch {
		log.Printf("Security: Proxy usage changed from %t to %t", oldConfig.Proxy.Switch, newConfig.Proxy.Switch)
	}
	
	if oldConfig.Proxy.Proxy != newConfig.Proxy.Proxy {
		log.Printf("Security: Proxy URL changed (details not logged for security)")
	}
	
	if oldConfig.Translate.Key != newConfig.Translate.Key {
		log.Printf("Security: Translation API key changed (details not logged for security)")
	}
	
	// 验证敏感信息不被记录
	if newConfig.Translate.Key != "" {
		log.Printf("Security: Translation API key is configured")
	}
	
	return nil
}

// PerformanceConfigHandler 处理性能相关的配置变更
type PerformanceConfigHandler struct{}

// NewPerformanceConfigHandler 创建一个新的性能配置处理器
func NewPerformanceConfigHandler() *PerformanceConfigHandler {
	return &PerformanceConfigHandler{}
}

// OnConfigChange 处理性能相关的配置变更
func (h *PerformanceConfigHandler) OnConfigChange(oldConfig, newConfig *Config) error {
	// 记录性能相关的变更
	if oldConfig.Common.MultiThreading != newConfig.Common.MultiThreading {
		log.Printf("Performance: Multi-threading changed from %d to %d", oldConfig.Common.MultiThreading, newConfig.Common.MultiThreading)
		
		// 警告潜在问题
		if newConfig.Common.MultiThreading > 10 {
			log.Printf("Performance Warning: High multi-threading value (%d) may cause system instability", newConfig.Common.MultiThreading)
		}
	}
	
	if oldConfig.Common.Sleep != newConfig.Common.Sleep {
		log.Printf("Performance: Sleep interval changed from %d to %d seconds", oldConfig.Common.Sleep, newConfig.Common.Sleep)
	}
	
	if oldConfig.Proxy.Timeout != newConfig.Proxy.Timeout {
		log.Printf("Performance: Proxy timeout changed from %d to %d seconds", oldConfig.Proxy.Timeout, newConfig.Proxy.Timeout)
	}
	
	if oldConfig.Extrafanart.ParallelDownload != newConfig.Extrafanart.ParallelDownload {
		log.Printf("Performance: Extrafanart parallel downloads changed from %d to %d", oldConfig.Extrafanart.ParallelDownload, newConfig.Extrafanart.ParallelDownload)
	}
	
	return nil
}

// CompositeConfigHandler 组合多个处理器
type CompositeConfigHandler struct {
	handlers []ConfigChangeHandler
}

// NewCompositeConfigHandler 创建一个新的组合配置处理器
func NewCompositeConfigHandler(handlers ...ConfigChangeHandler) *CompositeConfigHandler {
	return &CompositeConfigHandler{
		handlers: handlers,
	}
}

// OnConfigChange 调用所有注册的处理器
func (h *CompositeConfigHandler) OnConfigChange(oldConfig, newConfig *Config) error {
	for _, handler := range h.handlers {
		if err := handler.OnConfigChange(oldConfig, newConfig); err != nil {
			// 记录错误但继续执行其他处理器
			log.Printf("Config handler error: %v", err)
		}
	}
	return nil
}

// AddHandler 向组合处理器添加新的处理器
func (h *CompositeConfigHandler) AddHandler(handler ConfigChangeHandler) {
	h.handlers = append(h.handlers, handler)
}

// RemoveHandler 从组合处理器中移除处理器
func (h *CompositeConfigHandler) RemoveHandler(handler ConfigChangeHandler) {
	for i, existingHandler := range h.handlers {
		if existingHandler == handler {
			h.handlers = append(h.handlers[:i], h.handlers[i+1:]...)
			break
		}
	}
}