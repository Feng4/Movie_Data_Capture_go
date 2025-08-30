package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/httpclient"
	"movie-data-capture/pkg/logger"
)

// MovieData 表示抓取的电影信息
type MovieData struct {
	Number          string            `json:"number"`
	Title           string            `json:"title"`
	OriginalTitle   string            `json:"original_title"`
	Actor           string            `json:"actor"`
	ActorList       []string          `json:"actor_list"`
	ActorPhoto      map[string]string `json:"actor_photo"`
	Release         string            `json:"release"`
	Year            string            `json:"year"`
	Runtime         string            `json:"runtime"`
	Director        string            `json:"director"`
	Studio          string            `json:"studio"`
	Label           string            `json:"label"`
	Series          string            `json:"series"`
	Tag             []string          `json:"tag"`
	Outline         string            `json:"outline"`
	Cover           string            `json:"cover"`
	CoverSmall      string            `json:"cover_small"`
	Trailer         string            `json:"trailer"`
	Extrafanart     []string          `json:"extrafanart"`
	Website         string            `json:"website"`
	Source          string            `json:"source"`
	ImageCut        int               `json:"imagecut"`
	Uncensored      bool              `json:"uncensored"`
	UserRating      float64           `json:"userrating"`
	UserVotes       int               `json:"uservotes"`
	NamingRule      string            `json:"naming_rule"`
	OriginalNaming  string            `json:"original_naming_rule"`
	Headers         map[string]string `json:"headers,omitempty"`
}

// Scraper 处理从各种来源抓取电影数据
type Scraper struct {
	config     *config.Config
	httpClient *httpclient.Client
	sources    []string
}

// New 创建新的抓取器实例
func New(cfg *config.Config) *Scraper {
	return &Scraper{
		config:     cfg,
		httpClient: httpclient.NewClient(&cfg.Proxy),
		sources:    cfg.GetSources(),
	}
}

// GetDataFromNumber 根据番号抓取电影数据
func (s *Scraper) GetDataFromNumber(number, specifiedSource, specifiedURL string) (*MovieData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.Info("Searching for movie data: %s", number)

	// 如果提供了指定来源，则只使用该来源
	sources := s.sources
	if specifiedSource != "" {
		sources = []string{specifiedSource}
	}

	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}

		logger.Debug("Trying source: %s", source)

		data, err := s.scrapeFromSource(ctx, source, number, specifiedURL)
		if err != nil {
			logger.Debug("Failed to scrape from %s: %v", source, err)
			continue
		}

		if data != nil {
			// 验证数据
			if data.Number == "" || data.Title == "" {
				logger.Debug("Invalid data from %s: missing number or title", source)
				continue
			}

			// 检查番号是否匹配（不区分大小写）
			if !strings.EqualFold(data.Number, number) {
				logger.Warn("Number mismatch: requested=%s, got=%s from %s", number, data.Number, source)
				// 某些来源可能会规范化番号，所以我们可能允许这种情况
			}

			// 处理数据
			s.processMovieData(data)
			
			logger.Info("Successfully found data from source: %s", source)
			return data, nil
		}
	}

	return nil, fmt.Errorf("no data found for number: %s", number)
}

// scrapeFromSource 从特定来源抓取数据
func (s *Scraper) scrapeFromSource(ctx context.Context, source, number, specifiedURL string) (*MovieData, error) {
	switch strings.ToLower(source) {
	case "javdb":
		// Use improved JavDB scraper
		return s.ScrapeImprovedJavDB(ctx, number)
	case "javbus":
		return s.scrapeJavBus(ctx, number)
	case "fanza":
		return s.scrapeFanza(ctx, number)
	case "dmm":
		return s.scrapeDMM(ctx, number)
	case "xcity":
		return s.scrapeXCity(ctx, number)
	case "mgstage":
		return s.scrapeMGStage(ctx, number)
	case "fc2", "fc2club":
		return s.scrapeFC2Club(ctx, number)
	case "jav321":
		return s.scrapeJAV321(ctx, number)
	case "javlibrary":
		return s.scrapeJavLibrary(ctx, number)

	case "cableav":
		return s.scrapeCableAV(ctx, number)
	case "cnmdb":
		return s.scrapeCNMDB(ctx, number)
	case "dahlia":
		return s.scrapeDahlia(ctx, number)
	case "faleno":
		return s.scrapeFaleno(ctx, number)
	case "fantastica":
		return s.scrapeFantastica(ctx, number)
	case "carib", "caribbeancom":
		return s.scrapeCarib(ctx, number)
	case "caribpr", "caribbeancompr":
		return s.scrapeCaribPR(ctx, number)
	case "dlsite":
		return s.scrapeDLSite(ctx, number)
	case "gcolle":
		return s.scrapeGColle(ctx, number)
	case "getchu":
		return s.scrapeGetchu(ctx, number)
	case "javmenu":
		return s.scrapeJavMenu(ctx, number)
	case "javday":
		return s.scrapeJavDay(ctx, number)
	case "freejavbt":
		return scrapeFreeJavBT(number)
	case "madou", "md":
		return s.scrapeMadou(ctx, number)
	default:
		return nil, fmt.Errorf("unsupported source: %s", source)
	}
}

// processMovieData 处理和规范化抓取的数据
func (s *Scraper) processMovieData(data *MovieData) {
	// 清理特殊字符
	data.Title = s.cleanSpecialCharacters(data.Title)
	data.Outline = s.cleanSpecialCharacters(data.Outline)
	data.Studio = s.cleanSpecialCharacters(data.Studio)
	data.Director = s.cleanSpecialCharacters(data.Director)
	data.Label = s.cleanSpecialCharacters(data.Label)
	data.Series = s.cleanSpecialCharacters(data.Series)

	// 处理演员列表
	for i, actor := range data.ActorList {
		data.ActorList[i] = s.cleanSpecialCharacters(actor)
	}

	// 处理标签
	for i, tag := range data.Tag {
		data.Tag[i] = s.cleanSpecialCharacters(tag)
	}

	// 移除空的/无效的标签
	data.Tag = s.cleanTags(data.Tag)

	// 规范化发布日期
	data.Release = s.normalizeDate(data.Release)

	// 如果未设置则设置原始标题
	if data.OriginalTitle == "" {
		data.OriginalTitle = data.Title
	}

	// 生成命名规则
	data.NamingRule = s.generateNamingRule(data)
	data.OriginalNaming = s.generateOriginalNamingRule(data)

	// 处理番号大写设置
	if s.config.NameRule.NumberUppercase {
		data.Number = strings.ToUpper(data.Number)
	}
}

// cleanSpecialCharacters 移除或替换在文件系统中引起问题的特殊字符
func (s *Scraper) cleanSpecialCharacters(text string) string {
	if text == "" {
		return text
	}

	// 用安全的替代字符替换有问题的字符
	replacements := map[string]string{
		"\\": "∖", // U+2216 SET MINUS
		"/":  "∕", // U+2215 DIVISION SLASH  
		":":  "꞉", // U+A789 MODIFIER LETTER COLON
		"*":  "∗", // U+2217 ASTERISK OPERATOR
		"?":  "？", // U+FF1F FULLWIDTH QUESTION MARK
		"\"": "＂", // U+FF02 FULLWIDTH QUOTATION MARK
		"<":  "ᐸ", // U+1438 CANADIAN SYLLABICS PA
		">":  "ᐳ", // U+1433 CANADIAN SYLLABICS PO
		"|":  "ǀ", // U+01C0 LATIN LETTER DENTAL CLICK
		"&":  "＆", // U+FF06 FULLWIDTH AMPERSAND
	}

	result := text
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	// 处理HTML实体
	result = strings.ReplaceAll(result, "&lsquo;", "'")
	result = strings.ReplaceAll(result, "&rsquo;", "'")
	result = strings.ReplaceAll(result, "&hellip;", "…")
	result = strings.ReplaceAll(result, "&amp;", "＆")

	return result
}

// cleanTags 移除无效或不需要的标签
func (s *Scraper) cleanTags(tags []string) []string {
	var cleaned []string
	
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || tag == "XXXX" || tag == "xxx" {
			continue
		}
		cleaned = append(cleaned, tag)
	}
	
	return cleaned
}

// normalizeDate 将日期格式规范化为YYYY-MM-DD
func (s *Scraper) normalizeDate(date string) string {
	if date == "" {
		return date
	}

	// 将 / 替换为 -
	date = strings.ReplaceAll(date, "/", "-")
	
	// TODO: 如果需要，添加更多日期格式规范化
	return date
}

// generateNamingRule 根据配置生成文件命名规则
func (s *Scraper) generateNamingRule(data *MovieData) string {
	rule := s.config.NameRule.NamingRule
	return s.applyNamingRule(rule, data, false)
}

// generateOriginalNamingRule 生成原始命名规则（翻译前）
func (s *Scraper) generateOriginalNamingRule(data *MovieData) string {
	rule := s.config.NameRule.NamingRule
	return s.applyNamingRule(rule, data, true)
}

// applyNamingRule 将命名规则模板应用到电影数据
func (s *Scraper) applyNamingRule(rule string, data *MovieData, useOriginal bool) string {
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

	// 如果需要则使用原始标题
	if useOriginal {
		fields["title"] = data.OriginalTitle
	}

	// 处理标签（数组字段）
	if strings.Contains(result, "tag") && len(data.Tag) > 0 {
		tagStr := strings.Join(data.Tag, "&")
		fields["tag"] = tagStr
	}

	// 处理演员列表（数组字段）  
	if strings.Contains(result, "actor") && len(data.ActorList) > 0 {
		actorStr := strings.Join(data.ActorList, "&")
		fields["actor"] = actorStr
	}

	// 替换占位符
	for field, value := range fields {
		placeholder := field
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// 处理单引号中的字面字符串
	re := regexp.MustCompile(`'([^']*)'`)
	result = re.ReplaceAllString(result, "$1")

	return result
}

// Close 关闭抓取器并清理资源
// 用于其他数据源的新抓取器方法
func (s *Scraper) scrapeCarib(ctx context.Context, number string) (*MovieData, error) {
	scraper := NewCaribScraper(s.httpClient)
	return scraper.ScrapeByNumber(ctx, number)
}

func (s *Scraper) scrapeCaribPR(ctx context.Context, number string) (*MovieData, error) {
	scraper := NewCaribPRScraper(s.httpClient)
	return scraper.ScrapeByNumber(ctx, number)
}

func (s *Scraper) scrapeDLSite(ctx context.Context, number string) (*MovieData, error) {
	scraper := NewDLSiteScraper(s.httpClient)
	return scraper.ScrapeByNumber(ctx, number)
}

func (s *Scraper) scrapeGColle(ctx context.Context, number string) (*MovieData, error) {
	scraper := NewGColleScraper(s.httpClient)
	return scraper.ScrapeByNumber(ctx, number)
}

func (s *Scraper) scrapeGetchu(ctx context.Context, number string) (*MovieData, error) {
	scraper := NewGetchuScraper(s.httpClient)
	return scraper.ScrapeByNumber(ctx, number)
}

func (s *Scraper) scrapeJavMenu(ctx context.Context, number string) (*MovieData, error) {
	scraper := NewJavMenuScraper(s.httpClient)
	return scraper.ScrapeByNumber(ctx, number)
}

func (s *Scraper) scrapeMadou(ctx context.Context, number string) (*MovieData, error) {
	scraper := NewMadouScraper(s.httpClient)
	return scraper.ScrapeByNumber(ctx, number)
}

func (s *Scraper) Close() error {
	if s.httpClient != nil {
		return s.httpClient.Close()
	}
	return nil
}