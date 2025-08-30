package fragment

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"movie-data-capture/pkg/logger"
)

// FragmentInfo 表示分片文件信息
type FragmentInfo struct {
	FilePath   string // 完整文件路径
	BaseName   string // 基础文件名（不含分片标识）
	PartNumber int    // 分片编号
	PartSuffix string // 分片后缀（如 "-cd1", "-CD2"）
	Extension  string // 文件扩展名
}

// FragmentGroup 表示同一影片的分片组
type FragmentGroup struct {
	BaseName  string         // 基础文件名
	Fragments []FragmentInfo // 分片列表
	MainFile  string         // 主文件路径（通常是第一个分片）
}

// FragmentManager 分片文件管理器
type FragmentManager struct {
	// 分片检测正则表达式
	cdRegex     *regexp.Regexp // 匹配 -cd1, -CD2 等格式
	partRegex   *regexp.Regexp // 匹配 -part1, -PART2 等格式
	discRegex   *regexp.Regexp // 匹配 -disc1, -DISC2 等格式
	customRegex *regexp.Regexp // 自定义分片格式
}

// NewFragmentManager 创建新的分片文件管理器
func NewFragmentManager() *FragmentManager {
	return &FragmentManager{
		cdRegex:   regexp.MustCompile(`(?i)[-_.](cd)(\d+)$`),
		partRegex: regexp.MustCompile(`(?i)[-_](part)(\d+)$`),
		discRegex: regexp.MustCompile(`(?i)[-_](disc)(\d+)$`),
		// 支持更多格式：-1, -2, -A, -B 等
		customRegex: regexp.MustCompile(`(?i)[-_]([a-z]*)(\d+)$`),
	}
}

// IsFragmentFile 检查文件是否为分片文件
func (fm *FragmentManager) IsFragmentFile(filename string) bool {
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// 检查各种分片格式
	if fm.cdRegex.MatchString(baseName) {
		return true
	}
	if fm.partRegex.MatchString(baseName) {
		return true
	}
	if fm.discRegex.MatchString(baseName) {
		return true
	}
	
	// 检查方括号格式 [1], [2] 等
	if regexp.MustCompile(`\[(\d+)\]$`).MatchString(baseName) {
		return true
	}
	
	// 检查字母后缀格式 -A, -B 等
	if regexp.MustCompile(`(?i)[-_]([A-Z])$`).MatchString(baseName) {
		return true
	}
	
	// 检查简单数字后缀（如 movie-1, movie-2），但只有当数字是1-9的小数字时才认为是分片
	if match := regexp.MustCompile(`[-_](\d+)$`).FindStringSubmatch(baseName); match != nil {
		// 只有当数字是1-9之间的小数字时才认为是分片文件
		if len(match[1]) == 1 && match[1] >= "1" && match[1] <= "9" {
			return true
		}
	}
	
	return false
}

// ParseFragmentInfo 解析分片文件信息
func (fm *FragmentManager) ParseFragmentInfo(filePath string) (*FragmentInfo, error) {
	filename := filepath.Base(filePath)
	extension := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, extension)
	
	info := &FragmentInfo{
		FilePath:  filePath,
		Extension: extension,
	}
	
	// 尝试匹配CD格式
	if matches := fm.cdRegex.FindStringSubmatch(baseName); len(matches) >= 3 {
		partNum := parsePartNumber(matches[2])
		info.PartNumber = partNum
		info.PartSuffix = matches[0] // 完整匹配的后缀
		info.BaseName = strings.TrimSuffix(baseName, matches[0])
		return info, nil
	}
	
	// 尝试匹配PART格式
	if matches := fm.partRegex.FindStringSubmatch(baseName); len(matches) >= 3 {
		partNum := parsePartNumber(matches[2])
		info.PartNumber = partNum
		info.PartSuffix = matches[0]
		info.BaseName = strings.TrimSuffix(baseName, matches[0])
		return info, nil
	}
	
	// 特殊处理 _part_1, _part_2 格式
	if matches := regexp.MustCompile(`(.+)_part_(\d+)$`).FindStringSubmatch(baseName); len(matches) >= 3 {
		partNum := parsePartNumber(matches[2])
		info.PartNumber = partNum
		info.PartSuffix = fmt.Sprintf("_part_%d", partNum)
		info.BaseName = matches[1]
		return info, nil
	}
	
	// 尝试匹配DISC格式
	if matches := fm.discRegex.FindStringSubmatch(baseName); len(matches) >= 3 {
		partNum := parsePartNumber(matches[2])
		info.PartNumber = partNum
		info.PartSuffix = matches[0]
		info.BaseName = strings.TrimSuffix(baseName, matches[0])
		return info, nil
	}
	
	// 尝试匹配方括号格式 [1], [2] 等
	if matches := regexp.MustCompile(`\[(\d+)\]$`).FindStringSubmatch(baseName); len(matches) >= 2 {
		partNum := parsePartNumber(matches[1])
		info.PartNumber = partNum
		info.PartSuffix = matches[0]
		info.BaseName = strings.TrimSuffix(baseName, matches[0])
		return info, nil
	}
	
	// 尝试匹配字母后缀格式 -A, -B 等
	if matches := regexp.MustCompile(`(?i)([-_])([A-Z])$`).FindStringSubmatch(baseName); len(matches) >= 3 {
		// 将字母转换为数字：A=1, B=2, C=3...
		letter := strings.ToUpper(matches[2])
		partNum := int(letter[0] - 'A' + 1)
		info.PartNumber = partNum
		info.PartSuffix = matches[0]
		info.BaseName = strings.TrimSuffix(baseName, matches[0])
		return info, nil
	}
	
	// 尝试匹配简单数字后缀
	if matches := regexp.MustCompile(`([-_])(\d+)$`).FindStringSubmatch(baseName); len(matches) >= 3 {
		partNum := parsePartNumber(matches[2])
		info.PartNumber = partNum
		info.PartSuffix = matches[0]
		info.BaseName = strings.TrimSuffix(baseName, matches[0])
		return info, nil
	}
	
	// 如果不是分片文件，返回原始信息
	info.BaseName = baseName
	info.PartNumber = 0
	info.PartSuffix = ""
	return info, nil
}

// GroupFragmentFiles 将文件列表按分片进行分组
func (fm *FragmentManager) GroupFragmentFiles(filePaths []string) ([]FragmentGroup, []string) {
	fragmentMap := make(map[string][]FragmentInfo)
	nonFragmentFiles := []string{}
	
	for _, filePath := range filePaths {
		if !fm.IsFragmentFile(filepath.Base(filePath)) {
			nonFragmentFiles = append(nonFragmentFiles, filePath)
			continue
		}
		
		info, err := fm.ParseFragmentInfo(filePath)
		if err != nil {
			logger.Warn("Failed to parse fragment info for %s: %v", filePath, err)
			nonFragmentFiles = append(nonFragmentFiles, filePath)
			continue
		}
		
		// 如果不是分片文件，加入非分片列表
		if info.PartNumber == 0 {
			nonFragmentFiles = append(nonFragmentFiles, filePath)
			continue
		}
		
		// 按基础文件名分组
		key := strings.ToLower(info.BaseName + info.Extension)
		fragmentMap[key] = append(fragmentMap[key], *info)
	}
	
	// 创建分片组
	var fragmentGroups []FragmentGroup
	for baseName, fragments := range fragmentMap {
		// 按分片编号排序
		sort.Slice(fragments, func(i, j int) bool {
			return fragments[i].PartNumber < fragments[j].PartNumber
		})
		
		group := FragmentGroup{
			BaseName:  baseName,
			Fragments: fragments,
			MainFile:  fragments[0].FilePath, // 第一个分片作为主文件
		}
		
		fragmentGroups = append(fragmentGroups, group)
		
		logger.Info("Found fragment group '%s' with %d parts", baseName, len(fragments))
		for _, frag := range fragments {
			logger.Debug("  Part %d: %s", frag.PartNumber, filepath.Base(frag.FilePath))
		}
	}
	
	return fragmentGroups, nonFragmentFiles
}

// GetMainFileFromGroup 获取分片组的主文件路径
func (fg *FragmentGroup) GetMainFileFromGroup() string {
	return fg.MainFile
}

// GetAllFragmentPaths 获取分片组中所有文件的路径
func (fg *FragmentGroup) GetAllFragmentPaths() []string {
	paths := make([]string, len(fg.Fragments))
	for i, frag := range fg.Fragments {
		paths[i] = frag.FilePath
	}
	return paths
}

// GetFragmentCount 获取分片数量
func (fg *FragmentGroup) GetFragmentCount() int {
	return len(fg.Fragments)
}

// HasMissingParts 检查是否有缺失的分片
func (fg *FragmentGroup) HasMissingParts() bool {
	if len(fg.Fragments) == 0 {
		return false
	}
	
	// 检查分片编号是否连续
	for i, frag := range fg.Fragments {
		expectedPart := i + 1
		if frag.PartNumber != expectedPart {
			return true
		}
	}
	
	return false
}

// parsePartNumber 解析分片编号
func parsePartNumber(partStr string) int {
	if partStr == "" {
		return 0
	}
	
	// 简单的字符串到数字转换
	var num int
	for _, char := range partStr {
		if char >= '0' && char <= '9' {
			num = num*10 + int(char-'0')
		}
	}
	
	return num
}