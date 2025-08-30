package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeJAV321 从JAV321抓取电影数据
func (s *Scraper) scrapeJAV321(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting JAV321 scraping for number: %s", number)
	
	// JAV321搜索URL
	searchURL := "https://www.jav321.com/search"
	
	// 创建表单数据
	formData := url.Values{}
	formData.Set("sn", number)
	
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Referer":      "https://www.jav321.com/",
	}
	
	resp, err := s.httpClient.Post(ctx, searchURL, strings.NewReader(formData.Encode()), headers)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}
	
	// 检查是否重定向到视频页面（如Python版本）
	if strings.Contains(resp.Request.URL.String(), "/video/") {
		return s.scrapeJAV321Page(ctx, resp.Request.URL.String())
	}
	
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	// 在搜索结果中查找第一个视频链接
	var detailURL string
	doc.Find("a[href*='/video/']").First().Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			if strings.HasPrefix(href, "/") {
				detailURL = "https://www.jav321.com" + href
			} else {
				detailURL = href
			}
		}
	})
	
	if detailURL == "" {
		return nil, fmt.Errorf("no detail page found for number: %s", number)
	}
	
	return s.scrapeJAV321Page(ctx, detailURL)
}

// scrapeJAV321Page 抓取特定的JAV321详情页面
func (s *Scraper) scrapeJAV321Page(ctx context.Context, url string) (*MovieData, error) {
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Referer":    "https://www.jav321.com/",
	}
	
	resp, err := s.httpClient.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	movieData := &MovieData{
		Website: url,
		Source:  "jav321",
	}
	
	// 提取标题（Python: expr_title = "/html/body/div[2]/div[1]/div[1]/div[1]/h3/text()"）
	// 尝试多个选择器来查找标题
	titleSelectors := []string{
		"body > div:nth-child(2) > div:nth-child(1) > div:nth-child(1) > div:nth-child(1) > h3",
		"div:nth-child(2) div:nth-child(1) div:nth-child(1) div:nth-child(1) h3",
		"h3", // 后备选择器
	}
	
	for _, selector := range titleSelectors {
		if title := doc.Find(selector).First().Text(); title != "" {
			movieData.Title = strings.TrimSpace(title)
			break
		}
	}
	
	// 调试：记录找到的内容
	logger.Debug("Title extraction - Found: '%s'", movieData.Title)
	
	// 提取封面（Python: expr_cover = "/html/body/div[2]/div[2]/div[1]/p/a/img/@src"）
	// 尝试多个选择器来查找封面图片
	coverSelectors := []string{
		"body > div:nth-child(2) > div:nth-child(2) > div:nth-child(1) > p > a > img",
		"div:nth-child(2) div:nth-child(2) div:nth-child(1) p a img",
		"img[src*='jacket']", // 封面图片的后备选择器
		"img[alt*='cover']",   // 封面图片的后备选择器
		"img", // 非常宽泛的后备选择器
	}
	
	for _, selector := range coverSelectors {
		if cover, exists := doc.Find(selector).First().Attr("src"); exists && cover != "" {
			if strings.HasPrefix(cover, "//") {
				cover = "https:" + cover
			} else if strings.HasPrefix(cover, "/") {
				cover = "https://www.jav321.com" + cover
			}
			movieData.Cover = cover
			break
		}
	}
	
	// 添加导演字段（JAV321可能没有导演信息，设置为空）
	movieData.Director = "" // JAV321不提供导演信息
	
	// 提取番号 - 尝试不同方法
	// 首先尝试从标题中提取，标题通常包含番号
	if movieData.Title != "" {
		// 使用正则表达式从标题中提取番号
		numberRegex := regexp.MustCompile(`([A-Z]+-\d+)`)
		if matches := numberRegex.FindString(strings.ToUpper(movieData.Title)); matches != "" {
			movieData.Number = matches
		}
	}
	
	// 如果在标题中未找到，尝试从整个页面文本中提取
	if movieData.Number == "" {
		// 获取页面所有文本并查找模式
		pageText := doc.Text()
		// 查找类似"品番: SSIS-001"或"品番:SSIS-001"的模式
		numberRegex := regexp.MustCompile(`品番:?\s*([A-Za-z]+-\d+)`)
		if matches := numberRegex.FindStringSubmatch(pageText); len(matches) > 1 {
			movieData.Number = strings.ToUpper(matches[1])
		}
	}
	
	// 提取演员（Python: expr_actor = '//b[contains(text(),"出演者")]/following-sibling::a[starts-with(@href,"/star")]/text()'）
	var actors []string
	doc.Find("b").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "出演者") {
			s.NextAll().Filter("a[href^='/star']").Each(func(j int, actor *goquery.Selection) {
				if actorName := strings.TrimSpace(actor.Text()); actorName != "" {
					actors = append(actors, actorName)
				}
			})
		}
	})
	movieData.ActorList = actors
	if len(actors) > 0 {
		movieData.Actor = strings.Join(actors, ",")
	}
	
	// 提取制作商/厂牌（Python: expr_studio = '//b[contains(text(),"メーカー")]/following-sibling::a[starts-with(@href,"/company")]/text()'）
	doc.Find("b").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "メーカー") {
			if studio := strings.TrimSpace(s.NextAll().Filter("a[href^='/company']").First().Text()); studio != "" {
				movieData.Studio = studio
				movieData.Label = studio // 与Python版本中的制作商相同
			}
		}
	})
	
	// 提取时长（Python: expr_runtime = '//b[contains(text(),"収録時間")]/following-sibling::node()'）
	doc.Find("b").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "収録時間") {
			// 查找b标签后的下一个文本节点或元素
			next := s.Next()
			if next.Length() > 0 {
				runtime := strings.TrimSpace(next.Text())
				if runtime != "" {
					// 仅提取数字部分
					re := regexp.MustCompile(`(\d+)`)
					if matches := re.FindString(runtime); matches != "" {
						movieData.Runtime = matches
						return
					}
				}
			}
			// 后备方案：尝试从父元素获取文本并在"収録時間:"后提取
			parentText := s.Parent().Text()
			if idx := strings.Index(parentText, "収録時間:"); idx != -1 {
				runtime := strings.TrimSpace(parentText[idx+len("収録時間:"):])
				// 仅提取数字部分
				re := regexp.MustCompile(`(\d+)`)
				if matches := re.FindString(runtime); matches != "" {
					movieData.Runtime = matches
				}
			}
		}
	})
	
	// 提取发布日期（Python: expr_release = '//b[contains(text(),"配信開始日")]/following-sibling::node()'）
	doc.Find("b").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "配信開始日") {
			// 查找b标签后的下一个文本节点或元素
			next := s.Next()
			if next.Length() > 0 {
				release := strings.TrimSpace(next.Text())
				if release != "" {
					movieData.Release = release
					movieData.Year = extractYear(release)
					return
				}
			}
			// 后备方案：尝试从父元素获取文本并在"配信開始日:"后提取
			parentText := s.Parent().Text()
			if idx := strings.Index(parentText, "配信開始日:"); idx != -1 {
				release := strings.TrimSpace(parentText[idx+len("配信開始日:"):])
				// 仅提取第一部分（在任何其他文本之前）
				if parts := strings.Fields(release); len(parts) > 0 {
					movieData.Release = parts[0]
					movieData.Year = extractYear(parts[0])
				}
			}
		}
	})
	
	// 提取标签（Python: expr_tags = '//b[contains(text(),"ジャンル")]/following-sibling::a[starts-with(@href,"/genre")]/text()'）
	var tags []string
	doc.Find("b").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "ジャンル") {
			s.NextAll().Filter("a[href^='/genre']").Each(func(j int, tag *goquery.Selection) {
				if tagName := strings.TrimSpace(tag.Text()); tagName != "" {
					tags = append(tags, tagName)
				}
			})
		}
	})
	movieData.Tag = tags
	
	// 提取系列（Python: expr_series = '//b[contains(text(),"シリーズ")]/following-sibling::node()'）
	doc.Find("b").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "シリーズ") {
			// 查找b标签后的下一个文本节点或元素
			next := s.Next()
			if next.Length() > 0 {
				series := strings.TrimSpace(next.Text())
				if series != "" {
					movieData.Series = series
					return
				}
			}
			// 后备方案：尝试从父元素获取文本并在"シリーズ:"后提取
			parentText := s.Parent().Text()
			if idx := strings.Index(parentText, "シリーズ:"); idx != -1 {
				series := strings.TrimSpace(parentText[idx+len("シリーズ:"):])
				// 仅提取第一部分（在任何其他文本之前）
				if parts := strings.Fields(series); len(parts) > 0 {
					movieData.Series = parts[0]
				}
			}
		}
	})
	
	// 提取简介（Python: expr_outline = "/html/body/div[2]/div[1]/div[1]/div[2]/div[3]/div/text()"）
	if outline := doc.Find("body > div:nth-child(2) > div:nth-child(1) > div:nth-child(1) > div:nth-child(2) > div:nth-child(3) > div").Text(); outline != "" {
		movieData.Outline = strings.TrimSpace(outline)
	}
	
	// Extract extra fanart (Python: expr_extrafanart = '//div[@class="col-md-3"]/div[@class="col-xs-12 col-md-12"]/p/a/img/@src')
	var extraFanart []string
	doc.Find("div.col-md-3 div.col-xs-12.col-md-12 p a img").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			if strings.HasPrefix(src, "//") {
				src = "https:" + src
			} else if strings.HasPrefix(src, "/") {
				src = "https://www.jav321.com" + src
			}
			extraFanart = append(extraFanart, src)
		}
	})
	movieData.Extrafanart = extraFanart
	
	// Extract trailer video URL (like Python version getTrailer method)
	if htmlContent, err := doc.Html(); err == nil {
		videoURLRegex := regexp.MustCompile(`<source src="(.*?)"`)
		if matches := videoURLRegex.FindStringSubmatch(htmlContent); len(matches) > 1 {
			videoURL := matches[1]
			// Replace domains like Python version
			videoURL = strings.ReplaceAll(videoURL, "awscc3001.r18.com", "cc3001.dmm.co.jp")
			videoURL = strings.ReplaceAll(videoURL, "cc3001.r18.com", "cc3001.dmm.co.jp")
			movieData.Trailer = videoURL
		}
	}
	
	logger.Debug("Successfully scraped JAV321 data - Number: %s, Title: %s", movieData.Number, movieData.Title)
	return movieData, nil
}