package scraper

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)



// scrapeJavDB 从JavDB抓取电影数据
func (s *Scraper) scrapeJavDB(ctx context.Context, number string) (*MovieData, error) {
	// 添加小延迟以模拟人类行为
	time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
	
	// 从配置中获取JavDB站点
	sites := strings.Split(s.config.Javdb.Sites, ",")
	
	for _, site := range sites {
		site = strings.TrimSpace(site)
		if site == "" {
			continue
		}
		
		baseURL := fmt.Sprintf("https://javdb%s.com", site)
		data, err := s.scrapeJavDBSite(ctx, baseURL, number)
		if err != nil {
			logger.Debug("Failed to scrape JavDB site %s: %v", site, err)
			continue
		}
		
		if data != nil {
			return data, nil
		}
	}
	
	// 如果配置的站点失败，尝试主站点
	return s.scrapeJavDBSite(ctx, "https://javdb.com", number)
}

// scrapeJavDBSite 从特定的JavDB站点抓取
func (s *Scraper) scrapeJavDBSite(ctx context.Context, baseURL, number string) (*MovieData, error) {
	// 搜索电影
	searchURL := fmt.Sprintf("%s/search?q=%s&f=all", baseURL, number)
	logger.Debug("JavDB search URL: %s", searchURL)
	
	// 为JavDB设置请求头以模拟真实浏览器并绕过反机器人保护
	headers := map[string]string{
		"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Cookie":          "over18=1; locale=zh",
		"Accept":           "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":  "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		// 不要手动设置Accept-Encoding - 让Go自动处理压缩
		"DNT":              "1",
		"Connection":       "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":   "document",
		"Sec-Fetch-Mode":   "navigate",
		"Sec-Fetch-Site":   "none",
		"Sec-Fetch-User":   "?1",
		"Cache-Control":    "max-age=0",
		"sec-ch-ua":        "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"",
		"sec-ch-ua-mobile": "?0",
		"sec-ch-ua-platform": "\"Windows\"",
	}
	
	resp, err := s.httpClient.Get(ctx, searchURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}
	
	// 调试：记录页面标题和一些内容
	pageTitle := doc.Find("title").Text()
	logger.Debug("Page title: %s", pageTitle)
	
	// 检查不同的可能选择器
	movieListItems := doc.Find(".movie-list .item")
	logger.Debug("Found %d items with .movie-list .item", movieListItems.Length())
	
	// 尝试替代选择器
	if movieListItems.Length() == 0 {
		// 尝试其他常见选择器
		alternatives := []string{
			".grid .item",
			".video-list .item",
			".movie-list .movie-item",
			".grid-item",
			"[class*='item']",
		}
		
		for _, selector := range alternatives {
			items := doc.Find(selector)
			logger.Debug("Trying selector '%s': found %d items", selector, items.Length())
			if items.Length() > 0 {
				movieListItems = items
				break
			}
		}
	}
	
	// 查找所有电影结果及其ID（遵循Python逻辑）
	var urls []string
	var ids []string
	
	movieListItems.Each(func(i int, s *goquery.Selection) {
		// 获取URL
		if href, exists := s.Find("a").Attr("href"); exists {
			urls = append(urls, href)
			logger.Debug("Found URL[%d]: %s", i, href)
		}
		
		// 尝试不同的ID选择器
		idSelectors := []string{
			".video-title strong",
			"strong",
			".title strong",
			".video-title",
			".title",
		}
		
		var id string
		for _, selector := range idSelectors {
			id = strings.TrimSpace(s.Find(selector).Text())
			if id != "" {
				logger.Debug("Found ID[%d] with selector '%s': %s", i, selector, id)
				break
			}
		}
		
		if id != "" {
			ids = append(ids, id)
		}
	})
	
	logger.Debug("Found %d results with IDs: %v", len(ids), ids)
	
	if len(urls) == 0 {
		return nil, fmt.Errorf("no search results found in javdb")
	}
	
	// 查找精确匹配（遵循Python逻辑）
	var correctURL string
	
	// 检查西方视频模式
	westernPattern := regexp.MustCompile(`[a-zA-Z]+\.\d{2}\.\d{2}\.\d{2}`)
	if westernPattern.MatchString(number) {
		logger.Debug("Western video detected")
		correctURL = urls[0]
	} else {
		// 查找精确ID匹配
		found := false
		for i, id := range ids {
			if strings.EqualFold(id, number) {
				correctURL = urls[i]
				found = true
				break
			}
		}
		
		if !found {
			if len(ids) == 0 {
				return nil, fmt.Errorf("no IDs found in search results")
			}
			
			// 检查第一个结果是否匹配
			if !strings.EqualFold(ids[0], number) {
				return nil, fmt.Errorf("expected %s, but found %s", number, ids[0])
			}
			
			correctURL = urls[0]
		}
	}
	
	// 构建完整URL
	var movieURL string
	if strings.HasPrefix(correctURL, "/") {
		movieURL = baseURL + correctURL
	} else {
		movieURL = correctURL
	}
	
	logger.Debug("Selected movie URL: %s", movieURL)
	
	// 获取电影详情
	return s.scrapeJavDBMoviePage(ctx, movieURL, number)
}

// scrapeJavDBMoviePage 从JavDB电影页面抓取电影详情
func (s *Scraper) scrapeJavDBMoviePage(ctx context.Context, movieURL, number string) (*MovieData, error) {
	// 使用与搜索相同的请求头以保持会话一致性
	headers := map[string]string{
		"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Cookie":          "over18=1; theme=auto; locale=zh",
		"Accept":           "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
		// 不要手动设置Accept-Encoding - 让Go自动处理压缩
		"DNT":              "1",
		"Connection":       "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":   "document",
		"Sec-Fetch-Mode":   "navigate",
		"Sec-Fetch-Site":   "same-origin",
		"Sec-Fetch-User":   "?1",
		"Referer":          movieURL,
		"sec-ch-ua":        "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"",
		"sec-ch-ua-mobile": "?0",
		"sec-ch-ua-platform": "\"Windows\"",
	}
	
	resp, err := s.httpClient.Get(ctx, movieURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("movie page returned status %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse movie page: %w", err)
	}
	
	data := &MovieData{
		Number:  number,
		Source:  "javdb",
		Website: movieURL,
	}
	
	// 提取标题
	if title := doc.Find("h2.title strong").Text(); title != "" {
		data.Title = strings.TrimSpace(title)
	}
	
	// Extract cover image
	if cover, exists := doc.Find(".video-cover img").Attr("src"); exists {
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		} else if strings.HasPrefix(cover, "/") {
			cover = "https://javdb.com" + cover
		}
		data.Cover = cover
	}
	
	// Extract movie info from panels
	doc.Find(".movie-panel-info .panel-block").Each(func(i int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("strong").Text())
		value := strings.TrimSpace(s.Find("span").Text())
		
		switch {
		case strings.Contains(label, "番號"):
			data.Number = value
		case strings.Contains(label, "日期"):
			data.Release = value
			if len(value) >= 4 {
				data.Year = value[:4]
			}
		case strings.Contains(label, "時長"):
			data.Runtime = value
		case strings.Contains(label, "導演"):
			data.Director = value
		case strings.Contains(label, "製作商"):
			data.Studio = value
		case strings.Contains(label, "發行商"):
			data.Label = value
		case strings.Contains(label, "系列"):
			data.Series = value
		case strings.Contains(label, "類別"):
			// Extract tags
			s.Find("span a").Each(func(j int, tag *goquery.Selection) {
				if tagText := strings.TrimSpace(tag.Text()); tagText != "" {
					data.Tag = append(data.Tag, tagText)
				}
			})
		case strings.Contains(label, "演員"):
			// Extract actors
			var actors []string
			actorPhotos := make(map[string]string)
			
			s.Find("span a").Each(func(j int, actor *goquery.Selection) {
				if actorName := strings.TrimSpace(actor.Text()); actorName != "" {
					actors = append(actors, actorName)
					
					// Try to get actor photo
					if _, exists := actor.Attr("href"); exists {
						// This would require another request to get actor photo
						// For now, just store empty
						actorPhotos[actorName] = ""
					}
				}
			})
			
			data.ActorList = actors
			data.Actor = strings.Join(actors, ",")
			data.ActorPhoto = actorPhotos
		}
	})
	
	// Extract plot/outline
	if outline := doc.Find(".movie-panel-info .panel-block .value").Last().Text(); outline != "" {
		data.Outline = strings.TrimSpace(outline)
	}
	
	// Extract rating
	if ratingText := doc.Find(".score-stars").Text(); ratingText != "" {
		if rating, err := parseRating(ratingText); err == nil {
			data.UserRating = rating
		}
	}
	
	// Extract extra fanart images
	doc.Find(".preview-images a img").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			if strings.HasPrefix(src, "//") {
				src = "https:" + src
			} else if strings.HasPrefix(src, "/") {
				src = "https://javdb.com" + src
			}
			data.Extrafanart = append(data.Extrafanart, src)
		}
	})
	
	// Set image cut mode (default to 1 for JavDB)
	data.ImageCut = 1
	
	// Check if uncensored
	for _, tag := range data.Tag {
		if strings.Contains(strings.ToLower(tag), "無碼") || 
		   strings.Contains(strings.ToLower(tag), "uncensored") {
			data.Uncensored = true
			break
		}
	}
	
	return data, nil
}

// parseRating parses rating from text
func parseRating(text string) (float64, error) {
	// Extract number from rating text
	re := regexp.MustCompile(`[\d.]+`)
	match := re.FindString(text)
	if match == "" {
		return 0, fmt.Errorf("no rating found")
	}
	
	return strconv.ParseFloat(match, 64)
}