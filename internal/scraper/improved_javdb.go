package scraper

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/httpclient"
	"movie-data-capture/pkg/logger"
)

// ImprovedJavDBScraper 表示改进的JavDB抓取器
type ImprovedJavDBScraper struct {
	client *httpclient.ImprovedClient
	config *config.Config
}

// NewImprovedJavDBScraper 创建新的改进JavDB抓取器
func (s *Scraper) NewImprovedJavDBScraper() *ImprovedJavDBScraper {
	return &ImprovedJavDBScraper{
		client: httpclient.NewImprovedClient(&s.config.Proxy),
		config: s.config,
	}
}

// ScrapeImprovedJavDB 使用改进方法从JavDB抓取电影数据
func (s *Scraper) ScrapeImprovedJavDB(ctx context.Context, number string) (*MovieData, error) {
	// 添加随机延迟以避免被检测为机器人
	delay := time.Duration(500+rand.Intn(1500)) * time.Millisecond
	time.Sleep(delay)
	
	scraper := s.NewImprovedJavDBScraper()
	
	// 从配置中获取JavDB站点，但以随机顺序尝试
	sites := strings.Split(s.config.Javdb.Sites, ",")
	
	// 随机化站点顺序以分散负载
	rand.Shuffle(len(sites), func(i, j int) {
		sites[i], sites[j] = sites[j], sites[i]
	})
	
	// 将主站点添加到列表中
	allSites := append(sites, "")
	
	for _, site := range allSites {
		site = strings.TrimSpace(site)
		
		var baseURL string
		if site == "" {
			baseURL = "https://javdb.com"
		} else {
			baseURL = fmt.Sprintf("https://javdb%s.com", site)
		}
		
		logger.Debug("Trying JavDB site: %s", baseURL)
		
		data, err := scraper.scrapeJavDBSiteImproved(ctx, baseURL, number)
		if err != nil {
			logger.Debug("Failed to scrape JavDB site %s: %v", baseURL, err)
			
			// 在尝试下一个站点之前添加延迟
			time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)
			continue
		}
		
		if data != nil {
			logger.Info("Successfully scraped from JavDB site: %s", baseURL)
			return data, nil
		}
	}
	
	return nil, fmt.Errorf("failed to scrape from any JavDB site")
}

// scrapeJavDBSiteImproved 使用改进方法从特定JavDB站点抓取
func (ijs *ImprovedJavDBScraper) scrapeJavDBSiteImproved(ctx context.Context, baseURL, number string) (*MovieData, error) {
	// 步骤1：首先访问主页初始化会话
	err := ijs.initializeSession(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}
	
	// 步骤2：使用正确的会话执行搜索
	searchURL := fmt.Sprintf("%s/search?q=%s&f=all", baseURL, number)
	logger.Debug("JavDB search URL: %s", searchURL)
	
	// 设置通常需要的cookies
	cookies := map[string]string{
		"over18": "1",
		"locale": "zh",
		"theme":  "auto",
	}
	ijs.client.SetCookies(baseURL, cookies)
	
	// 搜索请求的自定义请求头
	headers := map[string]string{
		"Referer":           baseURL,
		"Sec-Fetch-Site":    "same-origin",
		"Sec-Fetch-Mode":    "navigate",
		"Sec-Fetch-Dest":    "document",
		"Upgrade-Insecure-Requests": "1",
	}
	
	resp, err := ijs.client.GetWithSession(ctx, searchURL, headers)
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
	
	// Step 3: Enhanced search result parsing
	movieURL, err := ijs.findMovieURL(doc, baseURL, number)
	if err != nil {
		return nil, err
	}
	
	// Step 4: Get movie details
	return ijs.scrapeMoviePageImproved(ctx, movieURL, number)
}

// initializeSession initializes the session by visiting the main page
func (ijs *ImprovedJavDBScraper) initializeSession(ctx context.Context, baseURL string) error {
	logger.Debug("Initializing session with %s", baseURL)
	
	headers := map[string]string{
		"Sec-Fetch-Site": "none",
		"Sec-Fetch-Mode": "navigate",
		"Sec-Fetch-Dest": "document",
		"Sec-Fetch-User": "?1",
	}
	
	resp, err := ijs.client.GetWithSession(ctx, baseURL, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 200 {
		logger.Debug("Session initialized successfully")
		return nil
	}
	
	return fmt.Errorf("failed to initialize session: status %d", resp.StatusCode)
}

// findMovieURL finds the correct movie URL from search results
func (ijs *ImprovedJavDBScraper) findMovieURL(doc *goquery.Document, baseURL, number string) (string, error) {
	// Debug: log page title and basic info
	pageTitle := doc.Find("title").Text()
	logger.Debug("Search page title: %s", pageTitle)
	
	// Check for redirect or error pages
	if strings.Contains(pageTitle, "Redirecting") || 
	   strings.Contains(pageTitle, "Error") ||
	   strings.Contains(pageTitle, "Access Denied") {
		return "", fmt.Errorf("got redirect or error page: %s", pageTitle)
	}
	
	// Try multiple selectors for movie items (based on different JavDB layouts)
	selectors := []string{
		".movie-list .item",
		".grid .item", 
		".video-list .item",
		".grid-item",
		"[class*='movie-list'] [class*='item']",
		".box[class*='item']",
		"a[href*='/v/']", // Direct link selector
	}
	
	var movieItems *goquery.Selection
	var usedSelector string
	
	for _, selector := range selectors {
		items := doc.Find(selector)
		if items.Length() > 0 {
			movieItems = items
			usedSelector = selector
			logger.Debug("Found %d items using selector: %s", items.Length(), selector)
			break
		}
	}
	
	if movieItems == nil || movieItems.Length() == 0 {
		// Try to find any links that might be movie links
		allLinks := doc.Find("a[href*='/v/']")
		if allLinks.Length() > 0 {
			logger.Debug("Found %d potential movie links using fallback", allLinks.Length())
			movieItems = allLinks
			usedSelector = "a[href*='/v/'] (fallback)"
		} else {
			return "", fmt.Errorf("no search results found - tried %d selectors", len(selectors))
		}
	}
	
	// Extract URLs and IDs
	var urls []string
	var ids []string
	
	movieItems.Each(func(i int, s *goquery.Selection) {
		// Get URL
		var href string
		var exists bool
		
		// Try different ways to get the URL
		if href, exists = s.Attr("href"); !exists {
			href, exists = s.Find("a").Attr("href")
		}
		
		if exists && href != "" {
			urls = append(urls, href)
			logger.Debug("Found URL[%d]: %s", i, href)
		}
		
		// Get movie ID/number from various possible locations
		idSelectors := []string{
			".video-title strong",
			"strong",
			".title strong", 
			".video-title",
			".title",
			"[class*='title']",
			".meta strong",
		}
		
		var id string
		for _, idSel := range idSelectors {
			id = strings.TrimSpace(s.Find(idSel).Text())
			if id != "" {
				break
			}
		}
		
		// If no ID found in child elements, try the element itself
		if id == "" {
			id = strings.TrimSpace(s.Text())
			// Extract movie number pattern from text
			patterns := []string{
				`[A-Z]+-\d+`,
				`[A-Z]+\d+`,
				`\b\d{6,8}\b`, // For FC2 style numbers
			}
			
			for _, pattern := range patterns {
				re := regexp.MustCompile(pattern)
				if matches := re.FindString(id); matches != "" {
					id = matches
					break
				}
			}
		}
		
		if id != "" {
			ids = append(ids, id)
			logger.Debug("Found ID[%d]: %s (using selector: %s)", i, id, usedSelector)
		}
	})
	
	logger.Debug("Found %d URLs and %d IDs", len(urls), len(ids))
	
	if len(urls) == 0 {
		return "", fmt.Errorf("no URLs found in search results")
	}
	
	// Find exact match using similar logic to Python version
	var correctURL string
	
	// Check for western video pattern (like Python version)
	westernPattern := regexp.MustCompile(`[a-zA-Z]+\.\d{2}\.\d{2}\.\d{2}`)
	if westernPattern.MatchString(number) {
		logger.Debug("Western video detected, using first result")
		correctURL = urls[0]
	} else {
		// Find exact ID match
		found := false
		for i, id := range ids {
			if i < len(urls) && strings.EqualFold(strings.TrimSpace(id), strings.TrimSpace(number)) {
				correctURL = urls[i]
				found = true
				logger.Debug("Found exact match at index %d: %s", i, id)
				break
			}
		}
		
		if !found {
			if len(ids) == 0 {
				logger.Debug("No IDs found, using first URL")
				correctURL = urls[0]
			} else {
				// Check if first result is close enough
				firstID := strings.TrimSpace(ids[0])
				if strings.EqualFold(firstID, number) {
					correctURL = urls[0]
					logger.Debug("Using first result as fallback: %s", firstID)
				} else {
					return "", fmt.Errorf("expected %s, but found %s (no exact match)", number, firstID)
				}
			}
		}
	}
	
	// Build full URL
	var movieURL string
	if strings.HasPrefix(correctURL, "/") {
		movieURL = baseURL + correctURL
	} else if strings.HasPrefix(correctURL, "http") {
		movieURL = correctURL
	} else {
		movieURL = baseURL + "/" + correctURL
	}
	
	logger.Debug("Selected movie URL: %s", movieURL)
	return movieURL, nil
}

// scrapeMoviePageImproved scrapes movie details with improved parsing
func (ijs *ImprovedJavDBScraper) scrapeMoviePageImproved(ctx context.Context, movieURL, number string) (*MovieData, error) {
	// Use appropriate headers for movie page request
	headers := map[string]string{
		"Referer":        movieURL,
		"Sec-Fetch-Site": "same-origin",
		"Sec-Fetch-Mode": "navigate", 
		"Sec-Fetch-Dest": "document",
	}
	
	resp, err := ijs.client.GetWithSession(ctx, movieURL, headers)
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
	
	// Initialize movie data
	data := &MovieData{
		Number:  number,
		Source:  "javdb", 
		Website: movieURL,
	}
	
	// Parse all movie information using improved selectors
	ijs.parseMovieInfo(doc, data)
	
	return data, nil
}

// parseMovieInfo parses movie information from the document
func (ijs *ImprovedJavDBScraper) parseMovieInfo(doc *goquery.Document, data *MovieData) {
	// Extract title with multiple fallbacks
	titleSelectors := []string{
		"h2.title strong",
		"h2.title", 
		".movie-title strong",
		".movie-title",
		"h1 strong",
		"h1",
		".video-meta-panel .title strong",
	}
	
	for _, sel := range titleSelectors {
		if title := strings.TrimSpace(doc.Find(sel).Text()); title != "" {
			data.Title = title
			logger.Debug("Found title using selector '%s': %s", sel, title)
			break
		}
	}
	
	// Extract cover image with multiple fallbacks
	coverSelectors := []string{
		".video-cover img",
		".column-video-cover img",
		".movie-cover img", 
		"img[class*='cover']",
		".thumb img",
	}
	
	for _, sel := range coverSelectors {
		if cover, exists := doc.Find(sel).Attr("src"); exists && cover != "" {
			if strings.HasPrefix(cover, "//") {
				cover = "https:" + cover
			} else if strings.HasPrefix(cover, "/") {
				cover = "https://javdb.com" + cover
			}
			data.Cover = cover
			logger.Debug("Found cover using selector '%s': %s", sel, cover)
			break
		}
	}
	
	// Extract information from info panels
	doc.Find(".movie-panel-info .panel-block, .video-meta-panel .panel-block").Each(func(i int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("strong").Text())
		value := strings.TrimSpace(s.Find("span").Text())
		
		logger.Debug("Processing info block: '%s' = '%s'", label, value)
		
		switch {
		case strings.Contains(label, "番號") || strings.Contains(label, "番号"):
			if value != "" {
				data.Number = value
			}
		case strings.Contains(label, "日期"):
			data.Release = value
			if len(value) >= 4 {
				data.Year = value[:4]
			}
		case strings.Contains(label, "時長") || strings.Contains(label, "时长"):
			data.Runtime = value
		case strings.Contains(label, "導演") || strings.Contains(label, "导演"):
			data.Director = value
		case strings.Contains(label, "製作商") || strings.Contains(label, "制作商") || strings.Contains(label, "片商"):
			data.Studio = value
		case strings.Contains(label, "發行商") || strings.Contains(label, "发行商"):
			data.Label = value
		case strings.Contains(label, "系列"):
			data.Series = value
		case strings.Contains(label, "類別") || strings.Contains(label, "类别"):
			// Extract tags
			s.Find("span a, .value a").Each(func(j int, tag *goquery.Selection) {
				if tagText := strings.TrimSpace(tag.Text()); tagText != "" {
					data.Tag = append(data.Tag, tagText)
				}
			})
		case strings.Contains(label, "演員") || strings.Contains(label, "演员"):
			// Extract actors
			var actors []string
			actorPhotos := make(map[string]string)
			
			s.Find("span a, .value a").Each(func(j int, actor *goquery.Selection) {
				if actorName := strings.TrimSpace(actor.Text()); actorName != "" {
					actors = append(actors, actorName)
					
					// Try to get actor photo URL
					if href, exists := actor.Attr("href"); exists {
						// Store the actor page URL for potential future photo extraction
						actorPhotos[actorName] = href
					}
				}
			})
			
			data.ActorList = actors
			data.Actor = strings.Join(actors, ",")
			data.ActorPhoto = actorPhotos
		}
	})
	
	// Extract plot/outline with multiple selectors
	outlineSelectors := []string{
		".movie-panel-info .panel-block .value",
		".video-meta-panel .description",
		".plot",
		".synopsis", 
		".description",
	}
	
	for _, sel := range outlineSelectors {
		if outline := strings.TrimSpace(doc.Find(sel).Last().Text()); outline != "" && len(outline) > 20 {
			data.Outline = outline
			break
		}
	}
	
	// Extract rating
	ratingSelectors := []string{
		".score-stars",
		".rating",
		".vote-average",
	}
	
	for _, sel := range ratingSelectors {
		if ratingText := strings.TrimSpace(doc.Find(sel).Text()); ratingText != "" {
			if rating, err := parseRating(ratingText); err == nil {
				data.UserRating = rating
				break
			}
		}
	}
	
	// Extract extra fanart images
	fanartSelectors := []string{
		".preview-images a img",
		".tile-images a img",
		".sample-images img",
		".gallery img",
	}
	
	for _, sel := range fanartSelectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				} else if strings.HasPrefix(src, "/") {
					src = "https://javdb.com" + src
				}
				data.Extrafanart = append(data.Extrafanart, src)
			}
		})
		
		if len(data.Extrafanart) > 0 {
			break
		}
	}
	
	// Set image cut mode
	data.ImageCut = 1
	
	// Check if uncensored
	for _, tag := range data.Tag {
		tagLower := strings.ToLower(tag)
		if strings.Contains(tagLower, "無碼") || 
		   strings.Contains(tagLower, "无码") ||
		   strings.Contains(tagLower, "uncensored") ||
		   strings.Contains(tagLower, "western") {
			data.Uncensored = true
			break
		}
	}
	
	logger.Debug("Parsed movie data: Title=%s, Actor=%s, Studio=%s, Tags=%d, Extrafanart=%d", 
		data.Title, data.Actor, data.Studio, len(data.Tag), len(data.Extrafanart))
}