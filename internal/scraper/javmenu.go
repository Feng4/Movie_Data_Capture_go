package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"movie-data-capture/pkg/httpclient"
	"movie-data-capture/pkg/logger"

	"github.com/PuerkitoBio/goquery"
)

// JavMenuScraper handles scraping from JavMenu
type JavMenuScraper struct {
	httpClient *httpclient.Client
}

// NewJavMenuScraper creates a new JavMenu scraper
func NewJavMenuScraper(client *httpclient.Client) *JavMenuScraper {
	return &JavMenuScraper{
		httpClient: client,
	}
}

// CleanNumber cleans and validates the number for JavMenu
func (j *JavMenuScraper) CleanNumber(number string) string {
	// Remove common prefixes and clean
	number = strings.ToUpper(strings.TrimSpace(number))
	number = regexp.MustCompile(`^(JAVMENU[-_]?)`).ReplaceAllString(number, "")
	return number
}

// ScrapeByNumber scrapes movie data by number
func (j *JavMenuScraper) ScrapeByNumber(ctx context.Context, number string) (*MovieData, error) {
	cleanedNumber := j.CleanNumber(number)
	logger.Debug("Scraping JavMenu with cleaned number: %s", cleanedNumber)

	// Search for the movie
	searchURL := fmt.Sprintf("https://javmenu.com/search?q=%s", cleanedNumber)
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
	}
	
	resp, err := j.httpClient.Get(ctx, searchURL, headers)
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

	// Find the best matching result link
	var movieURL string
	numberLower := strings.ToLower(strings.ReplaceAll(cleanedNumber, "-", ""))
	
	doc.Find("a[href*='/movie/']").Each(func(i int, s *goquery.Selection) {
		if movieURL == "" {
			href, exists := s.Attr("href")
			if exists {
				// Check if this link matches our number (case insensitive, ignore dashes)
				hrefLower := strings.ToLower(strings.ReplaceAll(href, "-", ""))
				if strings.Contains(hrefLower, numberLower) {
					if strings.HasPrefix(href, "/") {
						movieURL = "https://javmenu.com" + href
					} else if strings.HasPrefix(href, "http") {
						movieURL = href
					}
				}
			}
		}
	})

	if movieURL == "" {
		return nil, fmt.Errorf("movie not found for number: %s", number)
	}

	return j.ScrapeByURL(ctx, movieURL)
}

// ScrapeByURL scrapes movie data from a specific URL
func (j *JavMenuScraper) ScrapeByURL(ctx context.Context, url string) (*MovieData, error) {
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
	}
	
	resp, err := j.httpClient.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse movie page: %w", err)
	}

	data := &MovieData{
		Website: url,
	}

	// Extract basic information
	j.extractTitle(doc, data)
	if data.Title == "" {
		return nil, fmt.Errorf("no title found on page")
	}
	
	j.extractNumber(doc, data, "")
	j.extractCover(doc, data)
	j.extractReleaseDate(doc, data)
	j.extractRuntime(doc, data)
	j.extractDirector(doc, data)
	j.extractStudio(doc, data)
	j.extractActors(doc, data)
	j.extractTags(doc, data)
	j.extractOutline(doc, data)
	j.extractExtrafanart(doc, data)

	logger.Debug("Successfully scraped JavMenu data - Number: %s, Title: %s", data.Number, data.Title)
	return data, nil
}

// extractTitle extracts the movie title
func (j *JavMenuScraper) extractTitle(doc *goquery.Document, data *MovieData) {
	// Try multiple selectors for title (matching Python version)
	selectors := []string{
		"h1.movie-title",
		"h1",
		"title",
	}
	
	for _, selector := range selectors {
		if title := strings.TrimSpace(doc.Find(selector).First().Text()); title != "" {
			// Clean the title by removing number prefix (matching Python regex)
			numberRegex := regexp.MustCompile(`^[A-Z0-9-]+\s*`)
			data.Title = strings.TrimSpace(numberRegex.ReplaceAllString(title, ""))
			if data.Title != "" {
				break
			}
		}
	}
}

// extractNumber extracts the movie number
func (j *JavMenuScraper) extractNumber(doc *goquery.Document, data *MovieData, originalNumber string) {
	// Try to extract from page first (matching Python selectors)
	selectors := []string{
		"span.movie-code",
		"div.movie-info strong:contains('番号')",
	}
	
	for _, selector := range selectors {
		if selector == "div.movie-info strong:contains('番号')" {
			// For this selector, get the following sibling text
			if number := strings.TrimSpace(doc.Find(selector).Next().Text()); number != "" {
				data.Number = strings.ToUpper(number)
				return
			}
		} else {
			if number := strings.TrimSpace(doc.Find(selector).First().Text()); number != "" {
				data.Number = strings.ToUpper(number)
				return
			}
		}
	}
	
	// Fallback: use original number
	data.Number = strings.ToUpper(originalNumber)
}

// extractCover extracts the movie cover image
func (j *JavMenuScraper) extractCover(doc *goquery.Document, data *MovieData) {
	// Use selectors matching Python version
	selectors := []string{
		"img.movie-poster",
		"div.poster img",
		"img[class*='cover']",
	}
	
	for _, selector := range selectors {
		if src, exists := doc.Find(selector).First().Attr("src"); exists && src != "" {
			if strings.HasPrefix(src, "//") {
				data.Cover = "https:" + src
			} else if strings.HasPrefix(src, "/") {
				data.Cover = "https://javmenu.com" + src
			} else {
				data.Cover = src
			}
			break
		}
	}
}

// extractReleaseDate extracts the release date
func (j *JavMenuScraper) extractReleaseDate(doc *goquery.Document, data *MovieData) {
	// Use selectors matching Python version
	selectors := []string{
		"span.release-date",
		"div.movie-info strong:contains('发行')",
		"div.movie-info strong:contains('發行')",
		"div.movie-info strong:contains('Release')",
	}
	
	for _, selector := range selectors {
		var dateText string
		if selector == "span.release-date" {
			dateText = strings.TrimSpace(doc.Find(selector).First().Text())
		} else {
			// For strong elements, get the following sibling text
			dateText = strings.TrimSpace(doc.Find(selector).Next().Text())
		}
		
		if dateText != "" {
			// Parse date format (matching Python regex: YYYY[-/年]MM[-/月]DD)
			dateRegex := regexp.MustCompile(`(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})`)
			if matches := dateRegex.FindStringSubmatch(dateText); len(matches) > 3 {
				year, month, day := matches[1], matches[2], matches[3]
				if len(month) == 1 {
					month = "0" + month
				}
				if len(day) == 1 {
					day = "0" + day
				}
				data.Release = fmt.Sprintf("%s-%s-%s", year, month, day)
				break
			}
		}
	}
}

// extractRuntime extracts the movie runtime
func (j *JavMenuScraper) extractRuntime(doc *goquery.Document, data *MovieData) {
	// Use selectors matching Python version
	selectors := []string{
		"span.duration",
		"div.movie-info strong:contains('时长')",
		"div.movie-info strong:contains('時長')",
		"div.movie-info strong:contains('Duration')",
	}
	
	for _, selector := range selectors {
		var runtimeText string
		if selector == "span.duration" {
			runtimeText = strings.TrimSpace(doc.Find(selector).First().Text())
		} else {
			// For strong elements, get the following sibling text
			runtimeText = strings.TrimSpace(doc.Find(selector).Next().Text())
		}
		
		if runtimeText != "" {
			// Extract number from runtime text (matching Python regex)
			runtimeRegex := regexp.MustCompile(`(\d+)`)
			if matches := runtimeRegex.FindString(runtimeText); matches != "" {
				data.Runtime = matches
				break
			}
		}
	}
}

// extractDirector extracts the movie director
func (j *JavMenuScraper) extractDirector(doc *goquery.Document, data *MovieData) {
	// Use selectors matching Python version
	selectors := []string{
		"span.director a",
		"div.movie-info strong:contains('导演')",
		"div.movie-info strong:contains('導演')",
		"div.movie-info strong:contains('Director')",
	}
	
	for _, selector := range selectors {
		if selector == "span.director a" {
			if director := strings.TrimSpace(doc.Find(selector).First().Text()); director != "" {
				data.Director = director
				break
			}
		} else {
			// For strong elements, get the following sibling a element
			if director := strings.TrimSpace(doc.Find(selector).Next().Filter("a").Text()); director != "" {
				data.Director = director
				break
			}
		}
	}
}

// extractStudio extracts the movie studio
func (j *JavMenuScraper) extractStudio(doc *goquery.Document, data *MovieData) {
	// Use selectors matching Python version
	selectors := []string{
		"span.studio a",
		"div.movie-info strong:contains('制片')",
		"div.movie-info strong:contains('製片')",
		"div.movie-info strong:contains('Studio')",
	}
	
	for _, selector := range selectors {
		if selector == "span.studio a" {
			if studio := strings.TrimSpace(doc.Find(selector).First().Text()); studio != "" {
				data.Studio = studio
				break
			}
		} else {
			// For strong elements, get the following sibling a element
			if studio := strings.TrimSpace(doc.Find(selector).Next().Filter("a").Text()); studio != "" {
				data.Studio = studio
				break
			}
		}
	}
}

// extractActors extracts the actors
func (j *JavMenuScraper) extractActors(doc *goquery.Document, data *MovieData) {
	actors := j.extractActorsList(doc)
	data.ActorList = actors
	if len(actors) > 0 {
		data.Actor = strings.Join(actors, ", ")
	}
}

// extractActorsList extracts the movie actors
func (j *JavMenuScraper) extractActorsList(doc *goquery.Document) []string {
	var actors []string
	
	// Use selectors matching Python version
	selectors := []string{
		"div.actors a",
		"span.actress a",
		"div[class*='cast'] a",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if actor := strings.TrimSpace(s.Text()); actor != "" {
				actors = append(actors, actor)
			}
		})
		if len(actors) > 0 {
			break
		}
	}
	
	return actors
}

// extractTags extracts tags/genres
func (j *JavMenuScraper) extractTags(doc *goquery.Document, data *MovieData) {
	tags := j.extractTagsList(doc)
	data.Tag = tags
}

// extractTagsList extracts the movie tags
func (j *JavMenuScraper) extractTagsList(doc *goquery.Document) []string {
	var tags []string
	
	// Use selectors matching Python version
	selectors := []string{
		"div.tags a",
		"span.genre a",
		"div.movie-info strong:contains('类型')",
		"div.movie-info strong:contains('類型')",
		"div.movie-info strong:contains('Genre')",
	}
	
	for _, selector := range selectors {
		if strings.HasPrefix(selector, "div.movie-info strong") {
			// For strong elements, get all following sibling a elements
			doc.Find(selector).NextAll().Filter("a").Each(func(i int, s *goquery.Selection) {
				if tag := strings.TrimSpace(s.Text()); tag != "" {
					tags = append(tags, tag)
				}
			})
		} else {
			doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				if tag := strings.TrimSpace(s.Text()); tag != "" {
					tags = append(tags, tag)
				}
			})
		}
		if len(tags) > 0 {
			break
		}
	}
	
	return tags
}

// extractOutline extracts the movie outline/plot
func (j *JavMenuScraper) extractOutline(doc *goquery.Document, data *MovieData) {
	// Use selectors matching Python version
	selectors := []string{
		"div.plot",
		"div.summary",
		"div.description",
		"p.outline",
	}
	
	for _, selector := range selectors {
		if outline := strings.TrimSpace(doc.Find(selector).First().Text()); outline != "" {
			data.Outline = outline
			break
		}
	}
}

// extractExtrafanart extracts additional movie images
func (j *JavMenuScraper) extractExtrafanart(doc *goquery.Document, data *MovieData) {
	var extrafanart []string
	
	// Use selectors matching Python version
	selectors := []string{
		"div.screenshots img",
		"div.gallery img",
		"div.images img",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists && src != "" {
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				} else if strings.HasPrefix(src, "/") {
					src = "https://javmenu.com" + src
				}
				extrafanart = append(extrafanart, src)
			}
		})
		if len(extrafanart) > 0 {
			break
		}
	}
	
	data.Extrafanart = extrafanart
}

// parseDate parses various date formats
func (j *JavMenuScraper) parseDate(dateStr string) string {
	// Handle various date formats
	dateStr = strings.TrimSpace(dateStr)

	// YYYY-MM-DD format
	if regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(dateStr) {
		return dateStr
	}

	// YYYY/MM/DD format
	if regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`).MatchString(dateStr) {
		return strings.ReplaceAll(dateStr, "/", "-")
	}

	// Japanese format: YYYY年MM月DD日
	dateRegex := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
	if matches := dateRegex.FindStringSubmatch(dateStr); len(matches) == 4 {
		year := matches[1]
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		return fmt.Sprintf("%s-%02d-%02d", year, month, day)
	}

	return dateStr
}

// parseRuntime parses runtime from various formats
func (j *JavMenuScraper) parseRuntime(runtimeStr string) string {
	// Extract minutes from various formats
	runtimeStr = strings.TrimSpace(runtimeStr)

	// "120 min" format
	if regexp.MustCompile(`^\d+\s*min`).MatchString(runtimeStr) {
		runtimeRegex := regexp.MustCompile(`^(\d+)`)
		if matches := runtimeRegex.FindStringSubmatch(runtimeStr); len(matches) == 2 {
			return matches[1]
		}
	}

	// "120分" format
	if regexp.MustCompile(`^\d+分`).MatchString(runtimeStr) {
		runtimeRegex := regexp.MustCompile(`^(\d+)`)
		if matches := runtimeRegex.FindStringSubmatch(runtimeStr); len(matches) == 2 {
			return matches[1]
		}
	}

	// "2:00:00" format (convert to minutes)
	if regexp.MustCompile(`^\d+:\d{2}:\d{2}$`).MatchString(runtimeStr) {
		parts := strings.Split(runtimeStr, ":")
		if len(parts) == 3 {
			hours, _ := strconv.Atoi(parts[0])
			minutes, _ := strconv.Atoi(parts[1])
			totalMinutes := hours*60 + minutes
			return strconv.Itoa(totalMinutes)
		}
	}

	return runtimeStr
}

// IsValidNumber checks if the number format is valid for JavMenu
func (j *JavMenuScraper) IsValidNumber(number string) bool {
	cleanedNumber := j.CleanNumber(number)
	// JavMenu typically uses standard JAV number formats
	return regexp.MustCompile(`^[A-Z]{2,}-\d{3,}$`).MatchString(cleanedNumber)
}