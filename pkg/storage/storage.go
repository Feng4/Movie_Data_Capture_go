package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"movie-data-capture/internal/config"
	"movie-data-capture/internal/scraper"
	"movie-data-capture/pkg/logger"
)

// Storage 处理文件操作和文件夹创建
type Storage struct {
	config *config.Config
}

// New 创建一个新的存储实例
func New(cfg *config.Config) *Storage {
	return &Storage{
		config: cfg,
	}
}

// CreateFolder 根据位置规则创建输出文件夹
func (s *Storage) CreateFolder(data *scraper.MovieData) (string, error) {
	successFolder := s.config.Common.SuccessOutputFolder
	
	// 评估位置规则
	locationRule := s.config.NameRule.LocationRule
	folderPath := s.evaluateLocationRule(locationRule, data)
	
	// 调试：打印评估后的文件夹路径
	logger.Debug("Evaluated folder path: %s", folderPath)
	
	// 处理过长的演员名称
	if strings.Contains(locationRule, "actor") && len(data.Actor) > 100 {
		// 对于多演员电影，将演员替换为"多人作品"
		folderPath = strings.ReplaceAll(folderPath, data.Actor, "多人作品")
	}
	
	// 处理过长的标题
	maxTitleLen := s.config.NameRule.MaxTitleLen
	if maxTitleLen > 0 && strings.Contains(locationRule, "title") && len(data.Title) > maxTitleLen {
		shortTitle := data.Title[:maxTitleLen]
		folderPath = strings.ReplaceAll(folderPath, data.Title, shortTitle)
	}
	
	// 确保相对路径（添加 ./ 前缀）
	if !strings.HasPrefix(folderPath, ".") && !strings.HasPrefix(folderPath, "/") {
		folderPath = "./" + folderPath
	}
	
	fullPath := filepath.Join(successFolder, folderPath)
	fullPath = filepath.Clean(fullPath)
	
	// 转义有问题的字符
	fullPath = s.escapePath(fullPath)
	
	// 创建目录
	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		// 回退：仅使用编号创建
		fallbackPath := filepath.Join(successFolder, data.Number)
		fallbackPath = s.escapePath(fallbackPath)
		err = os.MkdirAll(fallbackPath, 0755)
		if err != nil {
			return "", fmt.Errorf("创建目录失败: %w", err)
		}
		return fallbackPath, nil
	}
	
	return fullPath, nil
}

// evaluateLocationRule 评估位置规则模板
func (s *Storage) evaluateLocationRule(rule string, data *scraper.MovieData) string {
	result := rule
	
	// 定义字段映射
	fields := map[string]string{
		"number":   data.Number,
		"title":    data.Title,
		"actor":    data.Actor,
		"studio":   data.Studio,
		"director": data.Director,
		"release":  data.Release,
		"year":     data.Year,
		"series":   data.Series,
		"label":    data.Label,
	}
	
	// 处理Python风格的表达式，如 "actor + '/' + number"
	// 逐步解析表达式
	parts := strings.Split(result, " + ")
	var resultParts []string
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		// 处理单引号中的字面字符串
		if strings.HasPrefix(part, "'") && strings.HasSuffix(part, "'") {
			// 移除引号并添加字面字符串
			literal := part[1 : len(part)-1]
			resultParts = append(resultParts, literal)
		} else {
			// 替换字段占位符
			if value, exists := fields[part]; exists {
				resultParts = append(resultParts, value)
			} else {
				// 如果不是已知字段则保持原样
				resultParts = append(resultParts, part)
			}
		}
	}
	
	// 连接所有部分，将 '/' 视为路径分隔符
	var pathComponents []string
	currentComponent := ""
	
	for _, part := range resultParts {
		if part == "/" {
				// 遇到分隔符时，将当前组件添加到路径中
			if currentComponent != "" {
				pathComponents = append(pathComponents, currentComponent)
				currentComponent = ""
			}
		} else {
			// 累积非分隔符部分
			currentComponent += part
		}
	}
	
	// 添加最后一个组件
	if currentComponent != "" {
		pathComponents = append(pathComponents, currentComponent)
	}
	
	// 使用 filepath.Join 创建具有操作系统特定分隔符的正确路径
	if len(pathComponents) > 1 {
		result = filepath.Join(pathComponents...)
	} else if len(pathComponents) == 1 {
		result = pathComponents[0]
	} else {
		result = strings.Join(resultParts, "")
	}
	
	// 移除任何剩余的空格
	result = strings.TrimSpace(result)
	
	return result
}

// escapePath 转义文件路径中的有问题字符
func (s *Storage) escapePath(path string) string {
	literals := s.config.Escape.Literals
	
	result := path
	for _, char := range literals {
		// 不转义路径分隔符
		if char == '\\' || char == '/' {
			continue
		}
		result = strings.ReplaceAll(result, string(char), "")
	}
	
	return result
}

// MoveFile 移动或链接文件到目标位置
func (s *Storage) MoveFile(sourcePath, destPath string) error {
	// 检查目标文件是否已存在
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("destination file already exists: %s", destPath)
	}
	
	// 创建目标目录
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	
	linkMode := s.config.Common.LinkMode
	
	switch linkMode {
	case 0:
		// 移动文件
		return s.moveFile(sourcePath, destPath)
	case 1:
		// 创建软链接
		return s.createSoftLink(sourcePath, destPath)
	case 2:
		// 首先尝试硬链接，失败则回退到软链接
		err := s.createHardLink(sourcePath, destPath)
		if err != nil {
			logger.Debug("Hard link failed, trying soft link: %v", err)
			return s.createSoftLink(sourcePath, destPath)
		}
		return nil
	default:
		return s.moveFile(sourcePath, destPath)
	}
}

// moveFile 将文件从源位置移动到目标位置
func (s *Storage) moveFile(sourcePath, destPath string) error {
	err := os.Rename(sourcePath, destPath)
	if err != nil {
		// 如果重命名失败，尝试复制并删除
		return s.copyAndDelete(sourcePath, destPath)
	}
	
	logger.Info("Moved file: %s -> %s", sourcePath, destPath)
	return nil
}

// createSoftLink 创建符号链接
func (s *Storage) createSoftLink(sourcePath, destPath string) error {
	// 首先尝试相对路径
	destDir := filepath.Dir(destPath)
	relPath, err := filepath.Rel(destDir, sourcePath)
	if err == nil {
		err = os.Symlink(relPath, destPath)
		if err == nil {
			logger.Info("Created soft link: %s -> %s", destPath, relPath)
			return nil
		}
	}
	
	// 回退到绝对路径
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	err = os.Symlink(absPath, destPath)
	if err != nil {
		return fmt.Errorf("failed to create soft link: %w", err)
	}
	
	logger.Info("Created soft link: %s -> %s", destPath, absPath)
	return nil
}

// createHardLink 创建硬链接
func (s *Storage) createHardLink(sourcePath, destPath string) error {
	err := os.Link(sourcePath, destPath)
	if err != nil {
		return fmt.Errorf("failed to create hard link: %w", err)
	}
	
	logger.Info("Created hard link: %s -> %s", destPath, sourcePath)
	return nil
}

// copyAndDelete 复制文件并删除源文件
func (s *Storage) copyAndDelete(sourcePath, destPath string) error {
	// 打开源文件
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()
	
	// 创建目标文件
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()
	
	// 复制数据
	_, err = destFile.ReadFrom(srcFile)
	if err != nil {
		// 移除部分复制的文件
		os.Remove(destPath)
		return fmt.Errorf("failed to copy file: %w", err)
	}
	
	// 复制文件权限
	srcInfo, err := srcFile.Stat()
	if err == nil {
		os.Chmod(destPath, srcInfo.Mode())
	}
	
	// 删除源文件
	err = os.Remove(sourcePath)
	if err != nil {
		logger.Warn("Failed to delete source file %s: %v", sourcePath, err)
	}
	
	logger.Info("Copied and deleted: %s -> %s", sourcePath, destPath)
	return nil
}

// MoveToFailedFolder 将文件移动到失败文件夹
func (s *Storage) MoveToFailedFolder(filePath string) error {
	failedFolder := s.config.Common.FailedOutputFolder
	
	// 如果失败文件夹不存在则创建
	if err := os.MkdirAll(failedFolder, 0755); err != nil {
		return fmt.Errorf("failed to create failed folder: %w", err)
	}
	
	mainMode := s.config.Common.MainMode
	linkMode := s.config.Common.LinkMode
	
	// 模式3或链接模式：添加到失败列表而不是移动
	if mainMode == 3 || linkMode > 0 {
		return s.addToFailedList(filePath, failedFolder)
	}
	
	// 移动模式：如果配置了则实际移动文件
	if s.config.Common.FailedMove {
		return s.moveToFailedFolder(filePath, failedFolder)
	}
	
	return nil
}

// addToFailedList 将文件路径添加到失败列表
func (s *Storage) addToFailedList(filePath, failedFolder string) error {
	failedListPath := filepath.Join(failedFolder, "failed_list.txt")
	
	file, err := os.OpenFile(failedListPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open failed list: %w", err)
	}
	defer file.Close()
	
	_, err = file.WriteString(filePath + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to failed list: %w", err)
	}
	
	logger.Info("Added to failed list: %s", filePath)
	return nil
}

// moveToFailedFolder 将文件移动到失败文件夹
func (s *Storage) moveToFailedFolder(filePath, failedFolder string) error {
	fileName := filepath.Base(filePath)
	destPath := filepath.Join(failedFolder, fileName)
	
	// 检查目标是否存在
	if _, err := os.Stat(destPath); err == nil {
		logger.Warn("File already exists in failed folder: %s", fileName)
		return nil
	}
	
	// 记录移动操作
	recordPath := filepath.Join(failedFolder, "where_was_i_before_being_moved.txt")
	file, err := os.OpenFile(recordPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		timestamp := time.Now().Format("2006-01-02 15:04")
		record := fmt.Sprintf("%s FROM[%s]TO[%s]\n", timestamp, filePath, destPath)
		file.WriteString(record)
		file.Close()
	}
	
	// 移动文件
	err = os.Rename(filePath, destPath)
	if err != nil {
		return fmt.Errorf("failed to move file to failed folder: %w", err)
	}
	
	logger.Info("Moved to failed folder: %s", fileName)
	return nil
}

// RemoveEmptyFolders 移除空目录
func (s *Storage) RemoveEmptyFolders(rootPath string) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 出错时继续
		}
		
		if !info.IsDir() {
			return nil
		}
		
		// 不要移除根路径本身
		if path == rootPath {
			return nil
		}
		
		// 检查目录是否为空
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		
		if len(entries) == 0 {
			err = os.Remove(path)
			if err == nil {
				logger.Info("Removed empty folder: %s", path)
			}
		}
		
		return nil
	})
}