package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ANSI 颜色代码
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorGreen  = "\033[32m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
	ColorDim    = "\033[2m"
)

// 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// 日志级别字符串映射
var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

// 日志级别颜色映射
var levelColors = map[LogLevel]string{
	DEBUG: ColorCyan,
	INFO:  ColorGreen,
	WARN:  ColorYellow,
	ERROR: ColorRed,
}

// Logger 结构体
type Logger struct {
	mu          sync.RWMutex
	logFile     *os.File
	enableColor bool
	enableFile  bool
	minLevel    LogLevel
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// getDefaultLogger 获取默认日志实例
func getDefaultLogger() *Logger {
	once.Do(func() {
		defaultLogger = &Logger{
			enableColor: true,
			enableFile:  false,
			minLevel:    DEBUG,
		}
	})
	return defaultLogger
}

// InitConsoleLogger 初始化仅控制台日志记录
func InitConsoleLogger() {
	logger := getDefaultLogger()
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.enableColor = true
	logger.enableFile = false
}

// InitFileLogger 初始化文件和控制台日志记录
func InitFileLogger(logDir string) error {
	logger := getDefaultLogger()
	logger.mu.Lock()
	defer logger.mu.Unlock()
	
	// 如果日志目录不存在则创建
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// 创建带时间戳的日志文件
	timestamp := time.Now().Format("20060102T150405")
	logPath := filepath.Join(logDir, fmt.Sprintf("mdc_%s.log", timestamp))
	
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logger.logFile = file
	logger.enableColor = true
	logger.enableFile = true

	return nil
}

// getModuleName 从调用栈中提取模块名称
func getModuleName() string {
	_, file, _, ok := runtime.Caller(3) // 跳过3层调用栈
	if !ok {
		return "unknown"
	}
	
	// 提取文件名（不包含路径和扩展名）
	fileName := filepath.Base(file)
	if idx := strings.LastIndex(fileName, "."); idx != -1 {
		fileName = fileName[:idx]
	}
	
	return fileName
}

// formatLogMessage 格式化日志消息
func (l *Logger) formatLogMessage(level LogLevel, module, message string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelName := levelNames[level]
	
	// 对齐格式：时间戳(23) + 级别(5) + 模块(12) + 消息
	return fmt.Sprintf("%s [%-5s] [%-12s] %s", timestamp, levelName, module, message)
}

// log 核心日志方法
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	// 检查日志级别
	if level < l.minLevel {
		return
	}
	
	module := getModuleName()
	message := fmt.Sprintf(format, args...)
	logLine := l.formatLogMessage(level, module, message)
	
	// 控制台输出（带颜色）
	if l.enableColor {
		coloredLine := fmt.Sprintf("%s%s%s", levelColors[level], logLine, ColorReset)
		if level == ERROR {
			fmt.Fprintln(os.Stderr, coloredLine)
		} else {
			fmt.Fprintln(os.Stdout, coloredLine)
		}
	} else {
		if level == ERROR {
			fmt.Fprintln(os.Stderr, logLine)
		} else {
			fmt.Fprintln(os.Stdout, logLine)
		}
	}
	
	// File output (no color)
	if l.enableFile && l.logFile != nil {
		fmt.Fprintln(l.logFile, logLine)
	}
}

// Info 记录信息消息
func Info(format string, args ...interface{}) {
	getDefaultLogger().log(INFO, format, args...)
}

// Error 记录错误消息
func Error(format string, args ...interface{}) {
	getDefaultLogger().log(ERROR, format, args...)
}

// Debug 记录调试消息
func Debug(format string, args ...interface{}) {
	getDefaultLogger().log(DEBUG, format, args...)
}

// Warn 记录警告消息
func Warn(format string, args ...interface{}) {
	getDefaultLogger().log(WARN, format, args...)
}

// SetLevel 设置日志级别
func SetLevel(level LogLevel) {
	logger := getDefaultLogger()
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.minLevel = level
}

// SetColorEnabled 设置是否启用颜色
func SetColorEnabled(enabled bool) {
	logger := getDefaultLogger()
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.enableColor = enabled
}

// LogWithContext 带上下文信息的日志
func LogWithContext(level LogLevel, context map[string]interface{}, format string, args ...interface{}) {
	logger := getDefaultLogger()
	logger.mu.RLock()
	defer logger.mu.RUnlock()
	
	// 检查日志级别
	if level < logger.minLevel {
		return
	}
	
	module := getModuleName()
	message := fmt.Sprintf(format, args...)
	
	// 添加上下文信息
	if len(context) > 0 {
		var contextParts []string
		for k, v := range context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		message = fmt.Sprintf("%s [%s]", message, strings.Join(contextParts, ", "))
	}
	
	logLine := logger.formatLogMessage(level, module, message)
	
	// 控制台输出（带颜色）
	if logger.enableColor {
		coloredLine := fmt.Sprintf("%s%s%s", levelColors[level], logLine, ColorReset)
		if level == ERROR {
			fmt.Fprintln(os.Stderr, coloredLine)
		} else {
			fmt.Fprintln(os.Stdout, coloredLine)
		}
	} else {
		if level == ERROR {
			fmt.Fprintln(os.Stderr, logLine)
		} else {
			fmt.Fprintln(os.Stdout, logLine)
		}
	}
	
	// 文件输出（无颜色）
	if logger.enableFile && logger.logFile != nil {
		fmt.Fprintln(logger.logFile, logLine)
	}
}

// InfoWithContext 带上下文的Info日志
func InfoWithContext(context map[string]interface{}, format string, args ...interface{}) {
	LogWithContext(INFO, context, format, args...)
}

// ErrorWithContext 带上下文的Error日志
func ErrorWithContext(context map[string]interface{}, format string, args ...interface{}) {
	LogWithContext(ERROR, context, format, args...)
}

// DebugWithContext 带上下文的Debug日志
func DebugWithContext(context map[string]interface{}, format string, args ...interface{}) {
	LogWithContext(DEBUG, context, format, args...)
}

// WarnWithContext 带上下文的Warn日志
func WarnWithContext(context map[string]interface{}, format string, args ...interface{}) {
	LogWithContext(WARN, context, format, args...)
}

// Close 关闭日志文件
func Close() error {
	logger := getDefaultLogger()
	logger.mu.Lock()
	defer logger.mu.Unlock()
	
	if logger.logFile != nil {
		err := logger.logFile.Close()
		logger.logFile = nil
		logger.enableFile = false
		return err
	}
	return nil
}

// MultiLineLog 多行日志支持，自动缩进对齐
func MultiLineLog(level LogLevel, title string, lines []string) {
	logger := getDefaultLogger()
	logger.mu.RLock()
	defer logger.mu.RUnlock()
	
	// 检查日志级别
	if level < logger.minLevel {
		return
	}
	
	module := getModuleName()
	
	// Output title line
	logger.log(level, title)
	
	// Output indented content lines
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			indentedLine := "    " + line // 4 spaces indentation
			logLine := logger.formatLogMessage(level, module, indentedLine)
			
			// Console output
			if logger.enableColor {
				coloredLine := fmt.Sprintf("%s%s%s", ColorDim, logLine, ColorReset)
				if level == ERROR {
					fmt.Fprintln(os.Stderr, coloredLine)
				} else {
					fmt.Fprintln(os.Stdout, coloredLine)
				}
			} else {
				if level == ERROR {
					fmt.Fprintln(os.Stderr, logLine)
				} else {
					fmt.Fprintln(os.Stdout, logLine)
				}
			}
			
			// File output
			if logger.enableFile && logger.logFile != nil {
				fmt.Fprintln(logger.logFile, logLine)
			}
		}
	}
}

// HighlightLog 高亮显示关键信息的日志
func HighlightLog(level LogLevel, format string, args ...interface{}) {
	logger := getDefaultLogger()
	logger.mu.RLock()
	defer logger.mu.RUnlock()
	
	// 检查日志级别
	if level < logger.minLevel {
		return
	}
	
	module := getModuleName()
	message := fmt.Sprintf(format, args...)
	logLine := logger.formatLogMessage(level, module, message)
	
	// Console output (highlighted)
	if logger.enableColor {
		highlightedLine := fmt.Sprintf("%s%s%s%s%s", ColorBold, levelColors[level], logLine, ColorReset, ColorReset)
		if level == ERROR {
			fmt.Fprintln(os.Stderr, highlightedLine)
		} else {
			fmt.Fprintln(os.Stdout, highlightedLine)
		}
	} else {
		if level == ERROR {
			fmt.Fprintln(os.Stderr, logLine)
		} else {
			fmt.Fprintln(os.Stdout, logLine)
		}
	}
	
	// 文件输出（无颜色）
	if logger.enableFile && logger.logFile != nil {
		fmt.Fprintln(logger.logFile, logLine)
	}
}