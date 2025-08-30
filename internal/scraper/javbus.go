package scraper

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeJavBus 从JavBus抓取电影数据
func (s *Scraper) scrapeJavBus(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting JavBus scraping for number: %s", number)
	
	// 特殊番号映射（基于Python版本）
	specialMappings := map[string]string{
		"DV-1649": "DV-1649_2014-07-25",
		"DV-1195": "DV-1195_2010-10-08",
		"BKD-003": "BKD-003_2009-09-05",
	}

	searchNumber := number
	if mapped, exists := specialMappings[number]; exists {
		searchNumber = mapped
		logger.Debug("Using special mapping: %s -> %s", number, mapped)
	}

	// 首先尝试主站点
	detailURL := fmt.Sprintf("https://www.javbus.com/%s", searchNumber)
	logger.Debug("Trying main JavBus URL: %s", detailURL)
	movieData, err := s.scrapeJavBusPage(ctx, detailURL, false)
	if err == nil {
		return movieData, nil
	}

	logger.Debug("Main JavBus site failed: %v, trying mirror sites", err)

	// 尝试镜像站点
	mirrorSites := []string{"buscdn.art"}
	for _, mirror := range mirrorSites {
		mirrorURL := fmt.Sprintf("https://www.%s/%s", mirror, searchNumber)
		logger.Debug("Trying mirror site: %s", mirrorURL)
		movieData, err := s.scrapeJavBusPage(ctx, mirrorURL, false)
		if err == nil {
			return movieData, nil
		}
	}

	// 作为后备尝试无码搜索
	logger.Debug("Trying uncensored JavBus search")
	uncensoredNumber := strings.ReplaceAll(number, ".", "-")
	uncensoredURL := fmt.Sprintf("https://www.javbus.red/%s", uncensoredNumber)
	return s.scrapeJavBusPage(ctx, uncensoredURL, true)
}

// scrapeJavBusPage 抓取特定的JavBus页面
func (s *Scraper) scrapeJavBusPage(ctx context.Context, url string, uncensored bool) (*MovieData, error) {
	// 设置包含年龄验证cookie的请求头（基于Python版本增强）
	headers := map[string]string{
		"Cookie": "existmag=all; over18=18",
		"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8,ja;q=0.7",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"DNT": "1",
		"Connection": "keep-alive",
		"Upgrade-Insecure-Requests": "1",
	}
	
	resp, err := s.httpClient.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("page not found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 读取响应体以检查内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	logger.Debug("JavBus response body length: %d bytes", len(body))
	if len(body) > 0 {
		// 记录前500个字符用于调试
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		logger.Debug("JavBus response preview: %s", preview)
	}
	
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// 调试：检查页面内容
	pageTitle := doc.Find("title").Text()
	logger.Debug("JavBus page title: %s", pageTitle)
	
	// 检查是否为错误页面或重定向
	if strings.Contains(pageTitle, "404") || strings.Contains(pageTitle, "Not Found") {
		return nil, fmt.Errorf("movie not found (404)")
	}
	
	// 检查年龄验证或其他阻止页面
	if strings.Contains(pageTitle, "Age Verification") || strings.Contains(pageTitle, "Verification") {
		return nil, fmt.Errorf("age verification required")
	}

	// 提取电影数据
	movieData := &MovieData{
		Website: url,
		Source:  "javbus",
	}

	// 提取番号（Python: getWebNumber）
	if number := doc.Find("span.header:contains('識別碼:')").Parent().Find("span").Eq(1).Text(); number != "" {
		movieData.Number = strings.TrimSpace(number)
		logger.Debug("Extracted number: %s", movieData.Number)
	} else {
		// 后备方案：从URL提取
		if strings.Contains(url, "/") {
			urlParts := strings.Split(url, "/")
			if len(urlParts) > 0 {
				lastPart := urlParts[len(urlParts)-1]
				if lastPart != "" {
					movieData.Number = lastPart
					logger.Debug("Extracted number from URL: %s", movieData.Number)
				}
			}
		}
		logger.Debug("No number found in page")
	}

	// 提取标题（Python: get_title）
	if title := doc.Find("h3").First().Text(); title != "" {
		movieData.Title = strings.TrimSpace(title)
		logger.Debug("Extracted title: %s", movieData.Title)
	} else {
		logger.Debug("No title found")
	}

	// 提取封面图片（Python: getCover）
	if coverHref, exists := doc.Find("a.bigImage").Attr("href"); exists {
		if strings.HasPrefix(coverHref, "/") {
			// 处理相对URL
			baseURL := "https://www.javbus.com"
			if strings.Contains(url, "buscdn.art") {
				baseURL = "https://www.buscdn.art"
			} else if strings.Contains(url, "javbus.red") {
				baseURL = "https://www.javbus.red"
			}
			movieData.Cover = baseURL + coverHref
		} else if !strings.HasPrefix(coverHref, "http") {
			baseURL := "https://www.javbus.com"
			if strings.Contains(url, "buscdn.art") {
				baseURL = "https://www.buscdn.art"
			} else if strings.Contains(url, "javbus.red") {
				baseURL = "https://www.javbus.red"
			}
			movieData.Cover = baseURL + "/" + coverHref
		} else {
			movieData.Cover = coverHref
		}
		logger.Debug("Extracted cover: %s", movieData.Cover)
	}

	// 从信息部分提取发布日期和时长（基于Python版本增强）
	var infoTexts []string
	doc.Find("div.container div.row div.col-md-3 p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			infoTexts = append(infoTexts, text)
		}
	})
	
	// 处理收集的信息文本
	for i, text := range infoTexts {
		logger.Debug("Info text %d: %s", i, text)
		
		// 检查是否为日期（包含年份模式）
		if strings.Contains(text, "-") && len(text) >= 8 {
			// 可能是发布日期
			movieData.Release = text
			movieData.Year = extractYear(text)
			logger.Debug("Extracted release date: %s, year: %s", movieData.Release, movieData.Year)
		}
		
		// 检查是否为时长（包含分钟指示符）
		if strings.Contains(text, "分") || strings.Contains(text, "min") {
			// 移除各种分钟后缀
			runtime := text
			runtime = strings.TrimSuffix(runtime, "分鐘")
			runtime = strings.TrimSuffix(runtime, "分钟")
			runtime = strings.TrimSuffix(runtime, "分")
			runtime = strings.TrimSuffix(runtime, "min")
			runtime = strings.TrimSpace(runtime)
			if runtime != "" {
				movieData.Runtime = runtime
				logger.Debug("Extracted runtime: %s", movieData.Runtime)
			}
		}
	}

	// 提取制作商
	studioSelector := "span:contains('製作商:')"
	if uncensored {
		studioSelector = "span:contains('メーカー:')"
	}
	if studio := doc.Find(studioSelector).Parent().Find("a").Text(); studio != "" {
		movieData.Studio = strings.TrimSpace(studio)
	}

	// 提取导演
	directorSelector := "span:contains('導演:')"
	if uncensored {
		directorSelector = "span:contains('監督:')"
	}
	if director := doc.Find(directorSelector).Parent().Find("a").Text(); director != "" {
		movieData.Director = strings.TrimSpace(director)
	}

	// 提取系列
	seriesSelector := "span:contains('系列:')"
	if uncensored {
		seriesSelector = "span:contains('シリーズ:')"
	}
	if series := doc.Find(seriesSelector).Parent().Find("a").Text(); series != "" {
		movieData.Series = strings.TrimSpace(series)
	}

	// 提取厂牌（发行商）
	labelSelector := "span:contains('發行商:')"
	if uncensored {
		labelSelector = "span:contains('レーベル:')"
	}
	if label := doc.Find(labelSelector).Parent().Find("a").Text(); label != "" {
		movieData.Label = strings.TrimSpace(label)
	}

	// 提取演员（Python: getActor和getActorPhoto）
	var actors []string
	var actorPhotos = make(map[string]string)
	
	// 提取演员姓名
	doc.Find("div.star-name a").Each(func(i int, s *goquery.Selection) {
		actorName := strings.TrimSpace(s.Text())
		if actorName != "" && actorName != "?" {
			// 检查演员是否已存在
				alreadyExists := false
				for _, existing := range actors {
					if existing == actorName {
						alreadyExists = true
						break
					}
				}
				
			if !alreadyExists {
				actors = append(actors, actorName)
				logger.Debug("Found actor: %s", actorName)
			}
		}
	})
	
	// 提取演员照片
	doc.Find("div.star-name").Each(func(i int, s *goquery.Selection) {
		actorName := strings.TrimSpace(s.Find("a").Text())
		if actorName != "" && actorName != "?" {
			// 从父容器获取演员照片
			if img := s.Parent().Find("a img"); img.Length() > 0 {
				if src, exists := img.Attr("src"); exists && src != "" {
					if !strings.Contains(src, "nowprinting.gif") && !strings.Contains(src, "no-avatar") {
						if strings.HasPrefix(src, "/") {
							// 处理相对URL
							baseURL := "https://www.javbus.com"
							if strings.Contains(url, "buscdn.art") {
								baseURL = "https://www.buscdn.art"
							} else if strings.Contains(url, "javbus.red") {
								baseURL = "https://www.javbus.red"
							}
							actorPhotos[actorName] = baseURL + src
						} else if strings.HasPrefix(src, "http") {
							actorPhotos[actorName] = src
						}
						logger.Debug("Found actor photo for %s: %s", actorName, actorPhotos[actorName])
					}
				}
			}
		}
	})
	
	movieData.ActorList = actors
	movieData.ActorPhoto = actorPhotos
	// 将演员列表转换为逗号分隔的字符串用于Actor字段
	if len(actors) > 0 {
		movieData.Actor = strings.Join(actors, ",")
		logger.Debug("Total actors found: %d", len(actors))
	}

	// 提取标签（Python: getTag）
	var tags []string
	
	// 从类型链接提取标签
	doc.Find("span.genre a").Each(func(i int, s *goquery.Selection) {
		tag := strings.TrimSpace(s.Text())
		if tag != "" {
			tags = append(tags, tag)
		}
	})
	
	// 如果从类型链接没有获取到标签，尝试使用meta关键词作为后备
	if len(tags) == 0 {
		if keywords, exists := doc.Find("meta[name='keywords']").Attr("content"); exists {
			tagList := strings.Split(keywords, ",")
			for _, tag := range tagList {
				tag = strings.TrimSpace(tag)
				if tag != "" && tag != movieData.Number && !strings.Contains(tag, "javbus") {
					tags = append(tags, tag)
				}
			}
		}
	}
	
	movieData.Tag = tags
	if len(tags) > 0 {
		logger.Debug("Extracted %d tags: %v", len(tags), tags)
	}

	// 提取额外剧照（Python: getExtraFanart）
	var extraFanart []string
	
	// 从样本瀑布流提取
	doc.Find("#sample-waterfall a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			if strings.HasPrefix(href, "/") {
				// 处理相对URL
				baseURL := "https://www.javbus.com"
				if strings.Contains(url, "buscdn.art") {
					baseURL = "https://www.buscdn.art"
				} else if strings.Contains(url, "javbus.red") {
					baseURL = "https://www.javbus.red"
				}
				extraFanart = append(extraFanart, baseURL+href)
			} else if strings.HasPrefix(href, "http") {
				extraFanart = append(extraFanart, href)
			}
		}
	})
	
	movieData.Extrafanart = extraFanart
	if len(extraFanart) > 0 {
		logger.Debug("Extracted %d extra fanart images", len(extraFanart))
	}

	// 检查是否为无码
	if doc.Find("#navbar ul li.active a[href*='uncensored']").Length() > 0 {
		movieData.Uncensored = true
	}

	// 为图片下载设置请求头以防止403错误
	movieData.Headers = map[string]string{
		"Referer": url,
	}

	// 设置图片裁剪模式用于封面裁剪
	movieData.ImageCut = 1

	logger.Debug("Successfully scraped JavBus data for: %s", movieData.Number)
	return movieData, nil
}