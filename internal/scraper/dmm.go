package scraper

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/httpclient"
	"movie-data-capture/pkg/logger"
)

// scrapeDMM scrapes movie data from DMM website using scraper's HTTP client
func (s *Scraper) scrapeDMM(ctx context.Context, number string) (*MovieData, error) {
	// Clean number for search
	searchNumber := cleanNumberForDMMSearch(number)
	logger.Debug("Original number: %s, Cleaned number: %s", number, searchNumber)
	
	// Try multiple URL formats
	urlFormats := []string{
		fmt.Sprintf("https://www.dmm.co.jp/mono/dvd/-/detail/=/cid=%s/", searchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/videoa/-/detail/=/cid=%s/", searchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/anime/-/detail/=/cid=%s/", searchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/mono/anime/-/detail/=/cid=%s/", searchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/videoc/-/detail/=/cid=%s/", searchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/nikkatsu/-/detail/=/cid=%s/", searchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/rental/-/detail/=/cid=%s/", searchNumber),
	}
	
	for i, url := range urlFormats {
		logger.Debug("Trying URL %d/%d: %s", i+1, len(urlFormats), url)
		movieInfo, err := s.scrapeDMMPage(ctx, url, number)
		if err != nil {
			logger.Debug("URL %d failed: %v", i+1, err)
		} else if movieInfo.Title != "" {
			logger.Debug("URL %d succeeded, found title: %s", i+1, movieInfo.Title)
			return movieInfo, nil
		} else {
			logger.Debug("URL %d returned empty title", i+1)
		}
	}
	
	return nil, fmt.Errorf("failed to scrape DMM data for number: %s", number)
}

// scrapeDMMPage scrapes a specific DMM page using scraper's HTTP client
func (s *Scraper) scrapeDMMPage(ctx context.Context, url, originalNumber string) (*MovieData, error) {
	// Set age verification cookies for DMM
	cookies := map[string]string{
		"age_check_done": "1",
		"ckcy":          "1", 
		"cklg":          "ja",
	}
	
	// Use the scraper's HTTP client (which has proper proxy configuration)
	client := httpclient.NewImprovedClient(&s.config.Proxy)
	
	err := client.SetCookies(url, cookies)
	if err != nil {
		logger.Debug("Failed to set cookies: %v", err)
	}
	
	// Prepare headers specifically for DMM
	headers := map[string]string{
		"Accept-Language": "ja-JP,ja;q=0.9,en-US;q=0.8,en;q=0.7",
		"Referer":         "https://www.dmm.co.jp/",
		"Accept-Encoding": "gzip, deflate, br", // Explicitly request compression
	}
	
	resp, err := client.GetWithSession(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Handle gzip decompression manually if needed
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	// Check for region restriction first
	body := doc.Text()
	logger.Debug("Response body length: %d", len(body))
	
	previewLen := 500
	if len(body) < previewLen {
		previewLen = len(body)
	}
	logger.Debug("Response body preview: %s", body[:previewLen])
	
	// Check for region restriction
	if strings.Contains(body, "このページはお住まいの地域からご利用になれません") ||
	   strings.Contains(body, "Sorry! This content is not available in your region") ||
	   strings.Contains(body, "not-available-in-your-region") {
		return nil, fmt.Errorf("region restriction detected - DMM/FANZA blocks access from your location")
	}
	
	// Check for age verification
	if strings.Contains(body, "年齢認証") || strings.Contains(body, "Age Verification") {
		return nil, fmt.Errorf("age verification required")
	}
	
	// Check if page has valid content - use meta tag approach like Python version
	hasValidContent := false
	doc.Find("meta[property='og:title']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists && content != "" {
			hasValidContent = true
		}
	})
	
	if !hasValidContent {
		// Try other indicators
		if doc.Find("h1#title").Length() > 0 || doc.Find(".product-title").Length() > 0 {
			hasValidContent = true
		}
	}
	
	if !hasValidContent {
		return nil, fmt.Errorf("no valid content found")
	}
	
	movieInfo := &MovieData{
		Source:  "dmm",
		Website: url,
		ImageCut: 0, // Default image cut setting
		Uncensored: false, // DMM is censored content
	}
	
	// Extract title
	movieInfo.Title = extractDMMTitle(doc)
	if movieInfo.Title == "" {
		return nil, fmt.Errorf("title not found")
	}
	movieInfo.OriginalTitle = movieInfo.Title
	
	// Extract other information
	movieInfo.Number = extractDMMNumber(doc, originalNumber)
	actorList := extractDMMActor(doc)
	if len(actorList) > 0 {
		movieInfo.Actor = strings.Join(actorList, ", ")
		movieInfo.ActorList = actorList
	}
	movieInfo.ActorPhoto = extractDMMActorPhoto(actorList)
	movieInfo.Cover = extractDMMCover(doc)
	movieInfo.CoverSmall = movieInfo.Cover // Use same image for both cover and cover_small
	movieInfo.Release = extractDMMRelease(doc)
	movieInfo.Year = extractDMMYear(movieInfo.Release)
	movieInfo.Runtime = extractDMMRuntime(doc)
	directorList := extractDMMDirector(doc)
	if len(directorList) > 0 {
		movieInfo.Director = strings.Join(directorList, ", ")
	}
	movieInfo.Studio = extractDMMStudio(doc)
	movieInfo.Label = extractDMMPublisher(doc)
	movieInfo.Series = extractDMMSeries(doc)
	movieInfo.Tag = extractDMMTag(doc)
	movieInfo.Outline = extractDMMOutline(doc)
	movieInfo.Extrafanart = extractDMMExtraFanart(doc)
	movieInfo.Trailer = extractDMMTrailer(doc)
	
	logger.Info("Successfully scraped DMM data for: %s", movieInfo.Number)
	return movieInfo, nil
}

// cleanNumberForDMMSearch cleans the number for DMM search
func cleanNumberForDMMSearch(number string) string {
	// Convert to lowercase and trim
	searchNumber := strings.ToLower(strings.TrimSpace(number))
	if strings.HasPrefix(searchNumber, "h-") {
		searchNumber = strings.Replace(searchNumber, "h-", "h_", 1)
	}
	// Keep only alphanumeric and underscore (same as Python version)
	re := regexp.MustCompile(`[^0-9a-zA-Z_]`)
	searchNumber = re.ReplaceAllString(searchNumber, "")
	return searchNumber
}

// extractDMMTitle extracts title from DMM page
func extractDMMTitle(doc *goquery.Document) string {
	// First try to get from meta og:title (like Python version)
	if content, exists := doc.Find("meta[property='og:title']").First().Attr("content"); exists && content != "" {
		title := strings.TrimSpace(content)
		// Clean title like Python version
		title = regexp.MustCompile(`^[A-Z0-9-]+\s*`).ReplaceAllString(title, "")
		title = regexp.MustCompile(`(?i)\s*-\s*FANZA.*$`).ReplaceAllString(title, "")
		title = strings.ReplaceAll(title, "\n", " ")
		title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
		return strings.TrimSpace(title)
	}
	
	// Try other selectors as fallback
	selectors := []string{
		"h1#title",
		"h1.product-title",
		"h1",
	}
	
	for _, selector := range selectors {
		title := strings.TrimSpace(doc.Find(selector).First().Text())
		if title != "" {
			// Clean title like Python version
			title = regexp.MustCompile(`^[A-Z0-9-]+\s*`).ReplaceAllString(title, "")
			title = regexp.MustCompile(`(?i)\s*-\s*FANZA.*$`).ReplaceAllString(title, "")
			title = strings.ReplaceAll(title, "\n", " ")
			title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
			return strings.TrimSpace(title)
		}
	}
	return ""
}

// extractDMMNumber extracts number from DMM page
func extractDMMNumber(doc *goquery.Document, originalNumber string) string {
	// Try to extract from page
	selectors := []string{
		"td:contains('品番') + td",
		"td:contains('商品番号') + td",
		".product-details td:contains('品番') + td",
	}
	
	for _, selector := range selectors {
		number := strings.TrimSpace(doc.Find(selector).First().Text())
		if number != "" {
			return number
		}
	}
	return originalNumber
}

// extractDMMActor extracts actors from DMM page
func extractDMMActor(doc *goquery.Document) []string {
	var actors []string
	selectors := []string{
		"td:contains('出演者') + td a",
		"td:contains('女優') + td a",
		".product-details td:contains('出演者') + td a",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			actor := strings.TrimSpace(s.Text())
			if actor != "" {
				actors = append(actors, actor)
			}
		})
		if len(actors) > 0 {
			break
		}
	}
	return actors
}

// extractDMMActorPhoto extracts actor photos (placeholder implementation)
func extractDMMActorPhoto(actors []string) map[string]string {
	actorPhoto := make(map[string]string)
	for _, actor := range actors {
		actorPhoto[actor] = ""
	}
	return actorPhoto
}

// extractDMMCover extracts cover image from DMM page
func extractDMMCover(doc *goquery.Document) string {
	// First try meta og:image (like Python version)
	if content, exists := doc.Find("meta[property='og:image']").First().Attr("content"); exists && content != "" {
		return normalizeImageURL(content)
	}
	
	// Try other selectors as fallback
	selectors := []string{
		"img[name='package-image']",
		"#sample-video img",
		".product-image img",
		"#package-image img",
		".package-image img",
	}
	
	for _, selector := range selectors {
		if img, exists := doc.Find(selector).First().Attr("src"); exists && img != "" {
			return normalizeImageURL(img)
		}
	}
	return ""
}

// normalizeImageURL normalizes image URL
func normalizeImageURL(img string) string {
	if strings.HasPrefix(img, "//") {
		return "https:" + img
	} else if strings.HasPrefix(img, "/") {
		return "https://www.dmm.co.jp" + img
	}
	return img
}

// extractDMMRelease extracts release date from DMM page
func extractDMMRelease(doc *goquery.Document) string {
	selectors := []string{
		"td:contains('発売日') + td",
		"td:contains('配信開始日') + td",
		".product-details td:contains('発売日') + td",
	}
	
	for _, selector := range selectors {
		date := strings.TrimSpace(doc.Find(selector).First().Text())
		if date != "" {
			// Convert date format if needed
			return convertDMMDate(date)
		}
	}
	return ""
}

// convertDMMDate converts DMM date format to standard format
func convertDMMDate(dateStr string) string {
	// DMM uses format like "2023/12/25" or "2023年12月25日"
	re := regexp.MustCompile(`(\d{4})[年/](\d{1,2})[月/](\d{1,2})`)
	matches := re.FindStringSubmatch(dateStr)
	if len(matches) == 4 {
		year := matches[1]
		month := fmt.Sprintf("%02s", matches[2])
		day := fmt.Sprintf("%02s", matches[3])
		return fmt.Sprintf("%s-%s-%s", year, month, day)
	}
	return dateStr
}

// extractDMMYear extracts year from release date
func extractDMMYear(release string) string {
	if len(release) >= 4 {
		return release[:4]
	}
	return ""
}

// extractDMMRuntime extracts runtime from DMM page
func extractDMMRuntime(doc *goquery.Document) string {
	selectors := []string{
		"td:contains('収録時間') + td",
		"td:contains('再生時間') + td",
		".product-details td:contains('収録時間') + td",
	}
	
	for _, selector := range selectors {
		runtime := strings.TrimSpace(doc.Find(selector).First().Text())
		if runtime != "" {
			// Extract minutes from text like "120分" or "2時間"
			re := regexp.MustCompile(`(\d+)分`)
			matches := re.FindStringSubmatch(runtime)
			if len(matches) == 2 {
				return matches[1]
			}
			// Handle hour format like "2時間"
			re = regexp.MustCompile(`(\d+)時間`)
			matches = re.FindStringSubmatch(runtime)
			if len(matches) == 2 {
				hours, _ := strconv.Atoi(matches[1])
				return strconv.Itoa(hours * 60)
			}
		}
	}
	return ""
}

// extractDMMDirector extracts director from DMM page
func extractDMMDirector(doc *goquery.Document) []string {
	var directors []string
	selectors := []string{
		"td:contains('監督') + td a",
		"td:contains('ディレクター') + td a",
		".product-details td:contains('監督') + td a",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			director := strings.TrimSpace(s.Text())
			if director != "" {
				directors = append(directors, director)
			}
		})
		if len(directors) > 0 {
			break
		}
	}
	return directors
}

// extractDMMStudio extracts studio from DMM page
func extractDMMStudio(doc *goquery.Document) string {
	selectors := []string{
		"td:contains('メーカー') + td a",
		"td:contains('スタジオ') + td a",
		".product-details td:contains('メーカー') + td a",
	}
	
	for _, selector := range selectors {
		studio := strings.TrimSpace(doc.Find(selector).First().Text())
		if studio != "" {
			return studio
		}
	}
	return ""
}

// extractDMMPublisher extracts publisher from DMM page
func extractDMMPublisher(doc *goquery.Document) string {
	selectors := []string{
		"td:contains('レーベル') + td a",
		"td:contains('ブランド') + td a",
		".product-details td:contains('レーベル') + td a",
	}
	
	for _, selector := range selectors {
		publisher := strings.TrimSpace(doc.Find(selector).First().Text())
		if publisher != "" {
			return publisher
		}
	}
	return ""
}

// extractDMMSeries extracts series from DMM page
func extractDMMSeries(doc *goquery.Document) string {
	selectors := []string{
		"td:contains('シリーズ') + td a",
		"td:contains('作品シリーズ') + td a",
		".product-details td:contains('シリーズ') + td a",
	}
	
	for _, selector := range selectors {
		series := strings.TrimSpace(doc.Find(selector).First().Text())
		if series != "" {
			return series
		}
	}
	return ""
}

// extractDMMTag extracts tags from DMM page
func extractDMMTag(doc *goquery.Document) []string {
	var tags []string
	selectors := []string{
		"td:contains('ジャンル') + td a",
		"td:contains('カテゴリ') + td a",
		".product-details td:contains('ジャンル') + td a",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			tag := strings.TrimSpace(s.Text())
			if tag != "" {
				tags = append(tags, tag)
			}
		})
		if len(tags) > 0 {
			break
		}
	}
	return tags
}

// extractDMMOutline extracts outline from DMM page
func extractDMMOutline(doc *goquery.Document) string {
	// Try selectors similar to Python version
	selectors := []string{
		".mg-b20.lh4",
		"p.mg-b20", 
		".summary",
		".product-description",
		"td:contains('商品紹介') + td",
		".product-details .description",
	}
	
	for _, selector := range selectors {
		outline := strings.TrimSpace(doc.Find(selector).First().Text())
		if outline != "" {
			// Clean outline like Python version
			outline = strings.ReplaceAll(outline, "\n", " ")
			outline = regexp.MustCompile(`\s+`).ReplaceAllString(outline, " ")
			return strings.TrimSpace(outline)
		}
	}
	return ""
}

// extractDMMExtraFanart extracts extra fanart from DMM page
func extractDMMExtraFanart(doc *goquery.Document) []string {
	var fanart []string
	selectors := []string{
		"#sample-image-block img",
		".sample-image-block img",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			img, exists := s.Attr("src")
			if exists && img != "" {
				img = normalizeImageURL(img)
				// Get large image like Python version - replace '-' with 'jp-'
				img = strings.Replace(img, "-", "jp-", 1)
				fanart = append(fanart, img)
			}
		})
		if len(fanart) > 0 {
			break
		}
	}
	return fanart
}

// extractDMMTrailer extracts trailer from DMM page
func extractDMMTrailer(doc *goquery.Document) string {
	selectors := []string{
		"#sample-video a",
		"video",
	}
	
	for _, selector := range selectors {
		var trailer string
		if selector == "#sample-video a" {
			trailer, _ = doc.Find(selector).First().Attr("href")
		} else {
			trailer, _ = doc.Find(selector).First().Attr("src")
		}
		
		if trailer != "" {
			return normalizeImageURL(trailer)
		}
	}
	return ""
}