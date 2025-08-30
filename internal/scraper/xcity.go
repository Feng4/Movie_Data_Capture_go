package scraper

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeXCity 从XCity抓取电影数据
func (s *Scraper) scrapeXCity(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting XCity scraping for number: %s", number)
	
	// XCity搜索需要表单提交
	xcityNumber := strings.ReplaceAll(number, "-", "")
	searchURL := "https://xcity.jp/main/"
	
	// 创建表单数据
	formData := url.Values{}
	formData.Set("q", strings.ToLower(xcityNumber))
	
	// 将表单数据转换为POST请求的正确格式
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}
	
	resp, err := s.httpClient.Post(ctx, searchURL, strings.NewReader(formData.Encode()), headers)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}
	
	// 读取响应体用于调试
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	logger.Debug("XCity response body length: %d", len(body))
	if len(body) > 500 {
		logger.Debug("XCity response body preview: %s", string(body[:500]))
	} else {
		logger.Debug("XCity response body preview: %s", string(body))
	}
	
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	// 查找详情页面链接
	var detailURL string
	doc.Find("a[href*='/avod/detail/']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			if strings.Contains(href, "/avod/detail/") {
				if strings.HasPrefix(href, "/") {
					detailURL = "https://xcity.jp" + href
				} else {
					detailURL = href
				}
				return
			}
		}
	})
	
	if detailURL == "" {
		return nil, fmt.Errorf("no detail page found for number: %s", number)
	}
	
	return s.scrapeXCityPage(ctx, detailURL)
}

// scrapeXCityPage 抓取特定的XCity详情页面
func (s *Scraper) scrapeXCityPage(ctx context.Context, url string) (*MovieData, error) {
	resp, err := s.httpClient.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// 读取响应体以检查内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	logger.Debug("XCity response body length: %d bytes", len(body))
	if len(body) > 0 {
		// 记录前500个字符用于调试
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		logger.Debug("XCity response preview: %s", preview)
	}
	
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	movieData := &MovieData{
		Website: url,
		Source:  "xcity",
	}
	
	// 提取番号
	if number := doc.Find("#hinban").Text(); number != "" {
		movieData.Number = strings.TrimSpace(number)
	}
	
	// 提取标题
	if title := doc.Find("#program_detail_title").Text(); title != "" {
		movieData.Title = strings.TrimSpace(title)
	}
	
	// 提取封面
	if cover, exists := doc.Find("#avodDetails div.frame p a").Attr("href"); exists {
		movieData.Cover = cover
	}
	
	// 提取演员
	var actors []string
	doc.Find("ul li.credit-links a").Each(func(i int, s *goquery.Selection) {
		if actor := strings.TrimSpace(s.Text()); actor != "" {
			actors = append(actors, actor)
		}
	})
	movieData.ActorList = actors
	// 将演员列表转换为逗号分隔的字符串用于Actor字段
	if len(actors) > 0 {
		movieData.Actor = strings.Join(actors, ",")
	}
	
	// 提取制作商
	if studio := doc.Find("strong:contains('片商')").Parent().Next().Find("a").Text(); studio != "" {
		movieData.Studio = strings.TrimSpace(studio)
	}
	
	// 提取时长
	if runtime := doc.Find("span.koumoku:contains('収録時間')").Parent().Text(); runtime != "" {
		// 仅提取时间部分
		re := regexp.MustCompile(`\d+`)
		if matches := re.FindString(runtime); matches != "" {
			movieData.Runtime = matches
		}
	}
	
	// 提取发布日期
	if release := doc.Find("#avodDetails ul li:nth-child(2)").Text(); release != "" {
		movieData.Release = strings.TrimSpace(release)
		movieData.Year = extractYear(release)
	}
	
	// 提取标签
	var tags []string
	doc.Find("span.koumoku:contains('ジャンル')").Parent().Find("a[href*='/avod/genre/']").Each(func(i int, s *goquery.Selection) {
		if tag := strings.TrimSpace(s.Text()); tag != "" {
			tags = append(tags, tag)
		}
	})
	movieData.Tag = tags
	
	// 提取导演
	if director := doc.Find("#program_detail_director").Text(); director != "" {
		movieData.Director = strings.TrimSpace(director)
	}
	
	// 提取系列
	if series := doc.Find("span:contains('シリーズ')").Parent().Find("a span").Text(); series != "" {
		movieData.Series = strings.TrimSpace(series)
	} else if series := doc.Find("span:contains('シリーズ')").Parent().Find("span").Text(); series != "" {
		movieData.Series = strings.TrimSpace(series)
	}
	
	// 提取额外剧照
	var extraFanart []string
	doc.Find("div#sample_images div a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			extraFanart = append(extraFanart, href)
		}
	})
	movieData.Extrafanart = extraFanart
	
	// 从og:description提取简介
	if outline, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		movieData.Outline = strings.TrimSpace(outline)
	}
	
	logger.Debug("Successfully scraped XCity data for: %s", movieData.Number)
	return movieData, nil
}