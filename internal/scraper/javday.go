package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeJavDay 从JavDay抓取电影数据
func (s *Scraper) scrapeJavDay(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting JavDay scraping for number: %s", number)
	
	// JavDay基础URL
	baseURL := "https://javday.tv"
	
	// 首先尝试直接URL访问
	detailURL := fmt.Sprintf("%s/videos/%s/", baseURL, number)
	logger.Debug("Trying JavDay URL: %s", detailURL)
	
	return s.scrapeJavDayPage(ctx, detailURL, baseURL)
}

// scrapeJavDayPage 从JavDay详情页面抓取数据
func (s *Scraper) scrapeJavDayPage(ctx context.Context, url, baseURL string) (*MovieData, error) {
	logger.Debug("Scraping JavDay page: %s", url)
	
	headers := map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Connection":      "keep-alive",
		"Referer":         baseURL,
	}
	
	resp, err := s.httpClient.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JavDay page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("JavDay returned status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	// 检查页面是否存在（Python: "你似乎來到了沒有視頻存在的荒原"）
	if strings.Contains(doc.Text(), "你似乎來到了沒有視頻存在的荒原") {
		return nil, fmt.Errorf("video not found on JavDay")
	}
	
	movieData := &MovieData{}
	
	// 提取标题（Python: get_title）
	if titleElement := doc.Find("#videoInfo div h1").First(); titleElement.Length() > 0 {
		movieData.Title = strings.TrimSpace(titleElement.Text())
		logger.Debug("Extracted title: %s", movieData.Title)
	}
	
	if movieData.Title == "" {
		return nil, fmt.Errorf("failed to extract title from JavDay")
	}
	
	// 提取封面（Python: get_cover）
	if metaElement := doc.Find("html head meta").Eq(7); metaElement.Length() > 0 {
		if content, exists := metaElement.Attr("content"); exists && content != "" {
			if !strings.HasPrefix(content, "http") {
				content = baseURL + content
			}
			movieData.Cover = content
			logger.Debug("Extracted cover: %s", movieData.Cover)
		}
	}
	
	// 提取系列、标签和演员（Python: get_some_info）
	var series, tags, actors []string
	
	// 从p[3]/span[2]/a提取系列
	doc.Find("#videoInfo div div p:nth-child(3) span:nth-child(2) a").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			series = append(series, text)
		}
	})
	
	// 从p[1]/span[2]/a提取标签
	doc.Find("#videoInfo div div p:nth-child(1) span:nth-child(2) a").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			tags = append(tags, text)
		}
	})
	
	// 从与标签相同的选择器提取演员（在Python中它们共享相同的路径）
	doc.Find("#videoInfo div div p:nth-child(1) span:nth-child(2) a").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			// 过滤掉"未知"演员
			if !strings.Contains(text, "未知") {
				actors = append(actors, text)
			}
		}
	})
	
	// 从分类链接提取额外标签（Python: get_tag）
	doc.Find("div.category a[href*='/class/']").Each(func(i int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			tags = append(tags, text)
		}
	})
	
	// 从标签中移除重复项
	tags = removeDuplicates(tags)
	
	// 从演员中移除重复项
	actors = removeDuplicates(actors)
	
	// 设置提取的数据
	if len(series) > 0 {
		movieData.Series = series[0]
	}
	movieData.Tag = tags
	movieData.Actor = strings.Join(actors, ", ")
	movieData.ActorList = actors
	
	// 提取和处理番号和标题（Python的get_real_number_title的简化版本）
	// 目前使用原始番号并清理标题
	movieData.Number = extractNumberFromURL(url)
	if movieData.Number == "" {
		movieData.Number = movieData.Title
	}
	
	// 通过移除番号和其他元素来清理标题
	cleanedTitle := movieData.Title
	if movieData.Number != movieData.Title {
		cleanedTitle = strings.ReplaceAll(cleanedTitle, movieData.Number, "")
	}
	
	// 从标题中移除标签和演员
	for _, tag := range tags {
		cleanedTitle = strings.ReplaceAll(cleanedTitle, tag, "")
	}
	for _, actor := range actors {
		cleanedTitle = strings.ReplaceAll(cleanedTitle, actor, "")
	}
	
	// 从标题中移除系列
	if movieData.Series != "" {
		cleanedTitle = strings.TrimPrefix(cleanedTitle, movieData.Series+" ")
	}
	
	// 清理标题
	cleanedTitle = strings.ReplaceAll(cleanedTitle, "  ", " ")
	cleanedTitle = strings.ReplaceAll(cleanedTitle, "..", ".")
	cleanedTitle = strings.ReplaceAll(cleanedTitle, " x ", "")
	cleanedTitle = strings.ReplaceAll(cleanedTitle, " X ", "")
	cleanedTitle = strings.Trim(cleanedTitle, " -.")
	
	movieData.Title = cleanedTitle
	
	// 为JavDay中不可用的字段设置默认值
	movieData.Release = ""
	movieData.Year = ""
	movieData.Runtime = ""
	movieData.Director = ""
	movieData.Studio = ""
	movieData.Label = ""
	movieData.Outline = ""
	movieData.Extrafanart = []string{}
	movieData.Trailer = ""
	
	logger.Debug("Successfully extracted JavDay data for: %s", movieData.Number)
	return movieData, nil
}

// extractNumberFromURL 从JavDay URL中提取番号
func extractNumberFromURL(url string) string {
	// 从URL模式提取：/videos/{number}/
	re := regexp.MustCompile(`/videos/([^/]+)/?`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// removeDuplicates 从切片中移除重复的字符串
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	
	for _, item := range slice {
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}