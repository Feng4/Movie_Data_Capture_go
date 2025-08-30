package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"movie-data-capture/internal/config"
	"movie-data-capture/internal/scraper"
	"movie-data-capture/pkg/logger"
	"movie-data-capture/pkg/parser"
)

// GetNumberFromFilename 从文件名中提取电影编号
func GetNumberFromFilename(filename string) string {
	// 移除文件扩展名
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	return GetNumberFromFilenameWithConfig(name, nil)
}

// GetNumberFromFilenameWithConfig 使用配置支持从文件名中提取电影编号
func GetNumberFromFilenameWithConfig(filename string, cfg *config.Config) string {
	// 使用增强的编号解析器
	numberParser := parser.NewNumberParser(cfg)
	return numberParser.GetNumber(filename)
}

// getNumberByBuiltinPatterns 使用内置模式提取编号（已弃用，请使用 parser 包）
func getNumberByBuiltinPatterns(name string) string {
	// 为向后兼容性回退到简单提取
	numberParser := parser.NewNumberParser(nil)
	return numberParser.GetNumber(name)
}

// GetMovieList 返回源文件夹中的电影文件列表
func GetMovieList(sourceFolder string, cfg *config.Config) ([]string, error) {
	var movieList []string
	
	// 获取支持的媒体类型
	mediaTypes := cfg.GetMediaTypes()
	
	// 获取要跳过的转义文件夹
	escapeFolders := strings.Split(cfg.Escape.Folders, ",")
	for i, folder := range escapeFolders {
		escapeFolders[i] = strings.TrimSpace(folder)
	}
	
	// 遍历源目录
	err := filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 出错时继续
		}
		
		// 跳过目录
		if info.IsDir() {
			// 检查是否应跳过此目录
			for _, escapeFolder := range escapeFolders {
				if escapeFolder != "" && strings.Contains(path, escapeFolder) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		
		// 检查文件扩展名
		ext := strings.ToLower(filepath.Ext(path))
		supported := false
		for _, mediaType := range mediaTypes {
			if ext == mediaType {
				supported = true
				break
			}
		}
		
		if !supported {
			return nil
		}
		
		// 跳过预告片文件
		if strings.Contains(strings.ToLower(filepath.Base(path)), "trailer") {
			return nil
		}
		
		// 跳过小文件（可能是广告）- 但允许大小为 0 的文件用于测试
		if info.Size() > 0 && info.Size() < 125829120 { // 120MB
			// 如果是调试/测试模式或明确配置则允许处理
			if !cfg.DebugMode.Switch {
				return nil
			}
		}
		
		// 检查文件是否在失败列表中（如果不忽略）
		if cfg.Common.MainMode == 3 || cfg.Common.LinkMode > 0 {
			if !cfg.Common.IgnoreFailedList {
				if isInFailedList(path, cfg.Common.FailedOutputFolder) {
					logger.Debug("跳过失败列表中的文件: %s", path)
					return nil
				}
			}
		}
		
		// 检查模式 3 的 NFO 跳过天数
		if cfg.Common.MainMode == 3 && cfg.Common.NFOSkipDays > 0 {
			nfoPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".nfo"
			if nfoInfo, err := os.Stat(nfoPath); err == nil {
				daysSince := int(time.Since(nfoInfo.ModTime()).Hours() / 24)
				if daysSince <= cfg.Common.NFOSkipDays {
					logger.Debug("Skipping file with recent NFO: %s", path)
					return nil
				}
			}
		}
		
		movieList = append(movieList, path)
		return nil
	})
	
	return movieList, err
}

// isInFailedList 检查文件路径是否在失败列表中
func isInFailedList(filePath, failedFolder string) bool {
	failedListPath := filepath.Join(failedFolder, "failed_list.txt")
	
	data, err := os.ReadFile(failedListPath)
	if err != nil {
		return false
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == filePath {
			return true
		}
	}
	
	return false
}

// IsUncensored 检查编号是否表示无码电影
func IsUncensored(number string, cfg *config.Config) bool {
	// 使用增强的编号解析器进行无码检测
	numberParser := parser.NewNumberParser(cfg)
	return numberParser.IsUncensored(number)
}

// DebugPrint 以调试格式打印电影数据
func DebugPrint(data *scraper.MovieData) {
	if data == nil {
		return
	}
	
	logger.Debug("------- DEBUG INFO -------")
	logger.Debug("number: %s", data.Number)
	logger.Debug("title: %s", data.Title)
	logger.Debug("actor: %s", data.Actor)
	logger.Debug("director: %s", data.Director)
	logger.Debug("studio: %s", data.Studio)
	logger.Debug("year: %s", data.Year)
	logger.Debug("release: %s", data.Release)
	logger.Debug("runtime: %s", data.Runtime)
	logger.Debug("series: %s", data.Series)
	logger.Debug("label: %s", data.Label)
	logger.Debug("cover: %s", data.Cover)
	logger.Debug("website: %s", data.Website)
	logger.Debug("source: %s", data.Source)
	
	if len(data.Tag) > 0 {
		logger.Debug("tags: %d items", len(data.Tag))
	}
	
	if len(data.Extrafanart) > 0 {
		logger.Debug("extrafanart: %d links", len(data.Extrafanart))
	}
	
	if len(data.Outline) > 0 {
		logger.Debug("outline: %d characters", len(data.Outline))
	}
	
	logger.Debug("------- DEBUG INFO -------")
}

// GetImageExtension 从 URL 确定图像扩展名
func GetImageExtension(url string) string {
	ext := strings.ToLower(filepath.Ext(url))
	
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp"}
	for _, validExt := range validExts {
		if ext == validExt {
			return ext
		}
	}
	
	// 默认为 .jpg
	return ".jpg"
}

// MovieFlags 表示电影文件的各种标志
type MovieFlags struct {
	Leak            bool   // 是否为泄露版本
	ChineseSubtitle bool   // 是否有中文字幕
	Hack            bool   // 是否为破解版本
	FourK           bool   // 是否为4K版本
	ISO             bool   // 是否为ISO格式
	Part            string // 分片标识（如 "-CD1"）
	IsMultiPart     bool   // 是否为多分片文件
}

// ParseMovieFlags 解析电影文件名中的各种标志
func ParseMovieFlags(filePath string) MovieFlags {
	var flags MovieFlags
	filename := filepath.Base(filePath)
	filenameUpper := strings.ToUpper(filename)
	
	// 检查多部分
	if match := regexp.MustCompile(`[-_]CD\d+`).FindString(filenameUpper); match != "" {
		flags.Part = match
		flags.IsMultiPart = true
	}
	
	// 检查中文字幕
	if regexp.MustCompile(`[-_]C(\.\w+$|-\w+)|\d+ch(\.\w+$|-\w+)`).MatchString(filename) ||
		strings.Contains(filename, "中文") ||
		strings.Contains(filename, "字幕") ||
		strings.Contains(filename, ".chs") ||
		strings.Contains(filename, ".cht") {
		flags.ChineseSubtitle = true
	}
	
	// 检查泄露
	if strings.Contains(filename, "流出") ||
		strings.Contains(strings.ToLower(filename), "uncensored") ||
		strings.Contains(strings.ToLower(filename), "leak") ||
		strings.Contains(filenameUpper, "-L") {
		flags.Leak = true
	}
	
	// 检查破解
	if strings.Contains(filenameUpper, "HACK") ||
		strings.Contains(filename, "破解") ||
		strings.Contains(filenameUpper, "-U") ||
		strings.Contains(filenameUpper, "-UC") {
		flags.Hack = true
	}
	
	// 检查 4K
	if strings.Contains(filenameUpper, "4K") {
		flags.FourK = true
	}
	
	// 检查 ISO
	if strings.Contains(filenameUpper, ".ISO") {
		flags.ISO = true
	}
	
	// 特殊组合
	if strings.Contains(filenameUpper, "-UC") {
		flags.Hack = true
		flags.ChineseSubtitle = true
	}
	
	if strings.Contains(filenameUpper, "-LC") {
		flags.Leak = true
		flags.ChineseSubtitle = true
	}
	
	return flags
}

// SanitizeFilename 移除或替换文件名中的无效字符
func SanitizeFilename(filename string) string {
	// 替换无效字符
	invalid := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	result := filename
	
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	
	// 移除前导/尾随空格和点
	result = strings.Trim(result, " .")
	
	return result
}

// FileExists 检查文件是否存在且不为空
func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// CreateDirectory 如果目录不存在则创建目录
func CreateDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}