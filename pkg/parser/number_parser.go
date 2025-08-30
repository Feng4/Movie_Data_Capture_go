package parser

import (
	"regexp"
	"strings"
	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/logger"
)

// NumberParser 处理从文件名中提取电影编号
type NumberParser struct {
	config *config.Config
	cleanupRegex *regexp.Regexp
	takeNumRules map[string]func(string) string
}

// NewNumberParser 创建一个新的编号解析器实例
func NewNumberParser(cfg *config.Config) *NumberParser {
	p := &NumberParser{
		config: cfg,
		cleanupRegex: regexp.MustCompile(`(?i)^\w+\.(cc|com|net|me|club|jp|tv|xyz|biz|wiki|info|tw|us|de)@|^22-sht\.me|^(fhd|hd|sd|1080p|720p|4K)(-|_)|(-|_)(fhd|hd|sd|1080p|720p|4K|x264|x265|uncensored|hack|leak)`),
	}
	
	// 初始化提取编号规则（类似于Python的G_TAKE_NUM_RULES）
	p.initTakeNumRules()
	
	return p
}

// initTakeNumRules 初始化特殊编号提取规则
func (p *NumberParser) initTakeNumRules() {
	p.takeNumRules = map[string]func(string) string{
		"tokyo.*hot": func(filename string) string {
			re := regexp.MustCompile(`(?i)(cz|gedo|k|n|red-|se)\d{2,4}`)
			if match := re.FindString(filename); match != "" {
				return match
			}
			return ""
		},
		"carib": func(filename string) string {
			re := regexp.MustCompile(`(?i)\d{6}(-|_)\d{3}`)
			if match := re.FindString(filename); match != "" {
				return strings.ReplaceAll(match, "_", "-")
			}
			return ""
		},
		"1pon|mura|paco": func(filename string) string {
			re := regexp.MustCompile(`(?i)\d{6}(-|_)\d{3}`)
			if match := re.FindString(filename); match != "" {
				return strings.ReplaceAll(match, "-", "_")
			}
			return ""
		},
		"10mu": func(filename string) string {
			re := regexp.MustCompile(`(?i)\d{6}(-|_)\d{2}`)
			if match := re.FindString(filename); match != "" {
				return strings.ReplaceAll(match, "-", "_")
			}
			return ""
		},
		"x-art": func(filename string) string {
			re := regexp.MustCompile(`(?i)x-art\.\d{2}\.\d{2}\.\d{2}`)
			return re.FindString(filename)
		},
		"xxx-av": func(filename string) string {
			re := regexp.MustCompile(`(?i)xxx-av[^\d]*(\d{3,5})[^\d]*`)
			if matches := re.FindStringSubmatch(filename); len(matches) > 1 {
				return "xxx-av-" + matches[1]
			}
			return ""
		},
		"heydouga": func(filename string) string {
			re := regexp.MustCompile(`(?i)(\d{4})[\-_](\d{3,4})[^\d]*`)
			if matches := re.FindStringSubmatch(filename); len(matches) > 2 {
				return "heydouga-" + matches[1] + "-" + matches[2]
			}
			return ""
		},
		"heyzo": func(filename string) string {
			re := regexp.MustCompile(`(?i)heyzo[^\d]*(\d{4})`)
			if matches := re.FindStringSubmatch(filename); len(matches) > 1 {
				return "HEYZO-" + matches[1]
			}
			return ""
		},
		"mdbk": func(filename string) string {
			re := regexp.MustCompile(`(?i)mdbk(-|_)(\d{4})`)
			return re.FindString(filename)
		},
		"mdtm": func(filename string) string {
			re := regexp.MustCompile(`(?i)mdtm(-|_)(\d{4})`)
			return re.FindString(filename)
		},
		"caribpr": func(filename string) string {
			re := regexp.MustCompile(`(?i)\d{6}(-|_)\d{3}`)
			if match := re.FindString(filename); match != "" {
				return strings.ReplaceAll(match, "_", "-")
			}
			return ""
		},
	}
}

// GetNumber 使用增强逻辑从文件名中提取电影编号
func (p *NumberParser) GetNumber(filename string) string {
	// 移除路径并获取基础文件名
	basename := strings.TrimSuffix(filename, getFileExtension(filename))
	
	// 首先尝试自定义正则表达式模式
	if p.config != nil && p.config.NameRule.NumberRegexs != "" {
		customPatterns := strings.Fields(p.config.NameRule.NumberRegexs)
		for _, pattern := range customPatterns {
			if pattern == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				logger.Warn("自定义正则表达式异常: %v [%s]", err, pattern)
				continue
			}
			
			if matches := re.FindStringSubmatch(basename); len(matches) > 1 {
				// 返回第一个捕获组
				result := matches[1]
				if result != "" {
					logger.Debug("自定义正则表达式匹配: %s -> %s", pattern, result)
					return strings.ToUpper(result)
				}
			} else if len(matches) == 1 {
				// 如果没有捕获组则返回整个匹配
				result := matches[0]
				if result != "" {
					logger.Debug("自定义正则表达式匹配: %s -> %s", pattern, result)
					return strings.ToUpper(result)
				}
			}
		}
	}
	
	// 尝试特殊字典规则
	if number := p.getNumberByDict(basename); number != "" {
		return number
	}
	
	// 处理字幕组或特殊格式
	if strings.Contains(basename, "字幕组") || strings.Contains(strings.ToUpper(basename), "SUB") || p.containsJapaneseKatakana(basename) {
		cleanName := p.cleanupRegex.ReplaceAllString(basename, "")
		// 移除括号内容
		bracketRegex := regexp.MustCompile(`\[.*?\]`)
		cleanName = bracketRegex.ReplaceAllString(cleanName, "")
		// 移除字幕后缀
		cleanName = strings.ReplaceAll(cleanName, ".chs", "")
		cleanName = strings.ReplaceAll(cleanName, ".cht", "")
		// 提取第一个点之前的内容
		dotRegex := regexp.MustCompile(`(.+?)\.`)
		if matches := dotRegex.FindStringSubmatch(cleanName); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	
	// 处理带有破折号或下划线的正常提取
	if strings.Contains(basename, "-") || strings.Contains(basename, "_") {
		return p.extractNormalNumber(basename)
	}
	
	// 处理没有破折号/下划线的编号（FANZA CID，欧洲编号）
	return p.extractNumberWithoutDash(basename)
}

// getNumberByDict 尝试将文件名与特殊规则字典匹配
func (p *NumberParser) getNumberByDict(filename string) string {
	for pattern, extractor := range p.takeNumRules {
		re := regexp.MustCompile(`(?i)` + pattern)
		if re.MatchString(filename) {
			if result := extractor(filename); result != "" {
				return result
			}
		}
	}
	return ""
}

// extractNormalNumber 处理带有破折号/下划线的正常编号提取
func (p *NumberParser) extractNormalNumber(filename string) string {
	// 清理文件名
	cleanName := p.cleanupRegex.ReplaceAllString(filename, "")
	
	// 移除日期模式
	dateRegex := regexp.MustCompile(`\[\d{4}-\d{1,2}-\d{1,2}\] - `)
	cleanName = dateRegex.ReplaceAllString(cleanName, "")
	
	// 处理FC2特殊情况
	lowerCheck := strings.ToLower(cleanName)
	if strings.Contains(lowerCheck, "fc2") {
		cleanName = strings.ReplaceAll(cleanName, "--", "-")
		cleanName = strings.ReplaceAll(cleanName, "_", "-")
		cleanName = strings.ToUpper(cleanName)
		
		// 专门提取FC2编号
		fc2Regex := regexp.MustCompile(`(?i)FC2[-_]?(?:PPV[-_]?)?(\d+)`)
		if matches := fc2Regex.FindStringSubmatch(cleanName); len(matches) > 1 {
			return "FC2-" + matches[1]
		}
	}
	
	// 移除CD后缀
	cdRegex := regexp.MustCompile(`(?i)[-_]cd\d{1,2}`)
	cleanName = cdRegex.ReplaceAllString(cleanName, "")
	
	// 如果移除CD后没有破折号/下划线，提取第一个单词
	if !strings.Contains(cleanName, "-") && !strings.Contains(cleanName, "_") {
		wordRegex := regexp.MustCompile(`\w+`)
		if match := wordRegex.FindString(cleanName); match != "" {
			dotIndex := strings.Index(cleanName, ".")
			if dotIndex > 0 {
				return match
			}
		}
	}
	
	// 提取标准格式（字母 + 破折号/下划线 + 数字）
	// 首先尝试复杂格式如MKY-NS-001
	complexRegex := regexp.MustCompile(`(?i)([a-z]+[-_][a-z]+)[-_](\d+)`)
	if matches := complexRegex.FindStringSubmatch(cleanName); len(matches) > 2 {
		prefix := strings.ToUpper(strings.ReplaceAll(matches[1], "_", "-"))
		return prefix + "-" + matches[2]
	}
	
	// 然后尝试简单格式如ABC-123
	standardRegex := regexp.MustCompile(`(?i)([a-z]+)[-_](\d+)`)
	if matches := standardRegex.FindStringSubmatch(cleanName); len(matches) > 2 {
		return strings.ToUpper(matches[1]) + "-" + matches[2]
	}
	
	// 回退：返回第一个字母数字序列
	fallbackRegex := regexp.MustCompile(`([a-zA-Z0-9-_]+)`)
	if match := fallbackRegex.FindString(cleanName); match != "" {
		return strings.ToUpper(match)
	}
	
	return ""
}

// extractNumberWithoutDash 处理没有破折号/下划线的编号
func (p *NumberParser) extractNumberWithoutDash(filename string) string {
	// 清理文件名
	cleanName := p.cleanupRegex.ReplaceAllString(filename, "")
	
	// 移除CD后缀
	cdRegex := regexp.MustCompile(`(?i)[-_]cd\d{1,2}`)
	cleanName = cdRegex.ReplaceAllString(cleanName, "")
	
	// 尝试提取FANZA CID格式（纯数字）
	numberRegex := regexp.MustCompile(`^(\d{6,})$`)
	if matches := numberRegex.FindStringSubmatch(cleanName); len(matches) > 1 {
		return matches[1]
	}
	
	// 尝试提取欧洲格式（字母 + 数字无分隔符）
	europeanRegex := regexp.MustCompile(`^([a-zA-Z]+)(\d+)$`)
	if matches := europeanRegex.FindStringSubmatch(cleanName); len(matches) > 2 {
		return strings.ToUpper(matches[1]) + "-" + matches[2]
	}
	
	// 回退：返回点之前的第一个单词
	dotIndex := strings.Index(cleanName, ".")
	if dotIndex > 0 {
		wordRegex := regexp.MustCompile(`\w+`)
		if match := wordRegex.FindString(cleanName[:dotIndex]); match != "" {
			return strings.ToUpper(match)
		}
	}
	
	return strings.ToUpper(cleanName)
}

// containsJapaneseKatakana 检查字符串是否包含日文片假名字符
func (p *NumberParser) containsJapaneseKatakana(text string) bool {
	katakanaRegex := regexp.MustCompile(`[ァ-ヿ]+`)
	return katakanaRegex.MatchString(text)
}

// IsUncensored 检查编号是否代表无码电影
func (p *NumberParser) IsUncensored(number string) bool {
	// 内置无码模式
	// 纯数字（6位以上）或特定模式
	pureNumberRegex := regexp.MustCompile(`^\d{6,}$`)
	if pureNumberRegex.MatchString(number) {
		return true
	}
	
	// 加勒比海格式（6位数字 + 下划线/破折号 + 2-3位数字）
	caribbeanRegex := regexp.MustCompile(`^\d{6}[-_]\d{2,3}$`)
	if caribbeanRegex.MatchString(number) {
		return true
	}
	
	// Tokyo Hot模式
	tokyoHotRegex := regexp.MustCompile(`(?i)^(cz|gedo|k|n|red-|se)\d{2,4}$`)
	if tokyoHotRegex.MatchString(number) {
		return true
	}
	
	// 其他无码模式
	otherUncensoredRegex := regexp.MustCompile(`(?i)^(heyzo-.+|xxx-av-.+|heydouga-.+|x-art\.\d{2}\.\d{2}\.\d{2})$`)
	if otherUncensoredRegex.MatchString(number) {
		return true
	}
	
	// 检查基于配置的无码前缀
	if p.config != nil && p.config.Uncensored.UncensoredPrefix != "" {
		prefixes := strings.Split(p.config.Uncensored.UncensoredPrefix, ",")
		numberUpper := strings.ToUpper(number)
		
		for _, prefix := range prefixes {
			prefix = strings.TrimSpace(strings.ToUpper(prefix))
			if prefix != "" && strings.HasPrefix(numberUpper, prefix) {
				return true
			}
		}
	}
	
	return false
}

// getFileExtension 返回包含点的文件扩展名
func getFileExtension(filename string) string {
	if idx := strings.LastIndex(filename, "."); idx != -1 {
		return filename[idx:]
	}
	return ""
}