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

// GetchuScraper handles scraping from Getchu
type GetchuScraper struct {
	httpClient *httpclient.Client
}

// NewGetchuScraper creates a new Getchu scraper
func NewGetchuScraper(client *httpclient.Client) *GetchuScraper {
	return &GetchuScraper{
		httpClient: client,
	}
}

// CleanNumber cleans and validates the number for Getchu
func (g *GetchuScraper) CleanNumber(number string) string {
	// Remove common prefixes and clean
	number = strings.ToUpper(strings.TrimSpace(number))
	number = regexp.MustCompile(`^(GETCHU[-_]?)`).ReplaceAllString(number, "")
	return number
}

// ScrapeByNumber scrapes movie data by number
func (g *GetchuScraper) ScrapeByNumber(ctx context.Context, number string) (*MovieData, error) {
	cleanedNumber := g.CleanNumber(number)
	logger.Debug("Scraping Getchu with cleaned number: %s", cleanedNumber)

	// Search for the movie
	searchURL := fmt.Sprintf("http://www.getchu.com/php/search.phtml?search_keyword=%s", cleanedNumber)
	resp, err := g.httpClient.Get(ctx, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	// Find the first result link
	var movieURL string
	doc.Find("a[href*='item.phtml']").Each(func(i int, s *goquery.Selection) {
		if movieURL == "" {
			href, exists := s.Attr("href")
			if exists {
				if strings.HasPrefix(href, "/") {
					movieURL = "http://www.getchu.com" + href
				} else if strings.HasPrefix(href, "http") {
					movieURL = href
				}
			}
		}
	})

	if movieURL == "" {
		return nil, fmt.Errorf("movie not found for number: %s", number)
	}

	return g.ScrapeByURL(ctx, movieURL)
}

// ScrapeByURL scrapes movie data from a specific URL
func (g *GetchuScraper) ScrapeByURL(ctx context.Context, url string) (*MovieData, error) {
	resp, err := g.httpClient.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse movie page: %w", err)
	}

	data := &MovieData{
		Website: url,
		Source:  "getchu",
	}

	// Extract basic information
	g.extractTitle(doc, data)
	g.extractCover(doc, data)
	g.extractReleaseDate(doc, data)
	g.extractRuntime(doc, data)
	g.extractDirector(doc, data)
	g.extractStudio(doc, data)
	g.extractActors(doc, data)
	g.extractTags(doc, data)
	g.extractOutline(doc, data)
	g.extractExtrafanart(doc, data)

	// Extract number from URL or title
	g.extractNumber(doc, data, url)

	return data, nil
}

// extractTitle extracts the movie title
func (g *GetchuScraper) extractTitle(doc *goquery.Document, data *MovieData) {
	// Try different selectors for title
	selectors := []string{
		"h1",
		".item_title",
		"title",
	}

	for _, selector := range selectors {
		if title := strings.TrimSpace(doc.Find(selector).First().Text()); title != "" {
			// Clean up title
			title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
			data.Title = title
			break
		}
	}
}

// extractCover extracts the cover image URL
func (g *GetchuScraper) extractCover(doc *goquery.Document, data *MovieData) {
	// Try different selectors for cover image
	selectors := []string{
		".item_image img",
		".package_image img",
		"img[src*='package']",
		"img[src*='item']",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if data.Cover == "" {
				if src, exists := s.Attr("src"); exists {
					if strings.HasPrefix(src, "/") {
						data.Cover = "http://www.getchu.com" + src
					} else if strings.HasPrefix(src, "http") {
						data.Cover = src
					}
				}
			}
		})
		if data.Cover != "" {
			break
		}
	}
}

// extractReleaseDate extracts the release date
func (g *GetchuScraper) extractReleaseDate(doc *goquery.Document, data *MovieData) {
	// Look for release date in various formats
	doc.Find("td, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "発売日") || strings.Contains(text, "配信開始日") {
			// Extract date from the text
			dateRegex := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
			if matches := dateRegex.FindStringSubmatch(text); len(matches) == 4 {
				year := matches[1]
				month := fmt.Sprintf("%02s", matches[2])
				day := fmt.Sprintf("%02s", matches[3])
				data.Release = fmt.Sprintf("%s-%s-%s", year, month, day)
			}
		}
	})
}

// extractRuntime extracts the runtime
func (g *GetchuScraper) extractRuntime(doc *goquery.Document, data *MovieData) {
	// Look for runtime information
	doc.Find("td, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "時間") || strings.Contains(text, "分") {
			// Extract runtime
			runtimeRegex := regexp.MustCompile(`(\d+)分`)
			if matches := runtimeRegex.FindStringSubmatch(text); len(matches) == 2 {
				data.Runtime = matches[1]
			}
		}
	})
}

// extractDirector extracts the director
func (g *GetchuScraper) extractDirector(doc *goquery.Document, data *MovieData) {
	// Look for director information
	doc.Find("td, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "監督") || strings.Contains(text, "ディレクター") {
			// Extract director name
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "監督") && !strings.Contains(line, "ディレクター") && line != "" {
					data.Director = line
					break
				}
			}
		}
	})
}

// extractStudio extracts the studio/publisher
func (g *GetchuScraper) extractStudio(doc *goquery.Document, data *MovieData) {
	// Look for studio/publisher information
	doc.Find("td, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "ブランド") || strings.Contains(text, "メーカー") {
			// Extract studio name
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "ブランド") && !strings.Contains(line, "メーカー") && line != "" {
					data.Studio = line
					break
				}
			}
		}
	})
}

// extractActors extracts the actors/voice actors
func (g *GetchuScraper) extractActors(doc *goquery.Document, data *MovieData) {
	var actors []string

	// Look for voice actor information
	doc.Find("td, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "声優") || strings.Contains(text, "CV") {
			// Extract actor names
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "声優") && !strings.Contains(line, "CV") && line != "" {
					// Split by common separators
					names := regexp.MustCompile(`[、,/]`).Split(line, -1)
					for _, name := range names {
						name = strings.TrimSpace(name)
						if name != "" {
							actors = append(actors, name)
						}
					}
				}
			}
		}
	})

	data.ActorList = actors
	if len(actors) > 0 {
		data.Actor = strings.Join(actors, ", ")
	}
}

// extractTags extracts tags/genres
func (g *GetchuScraper) extractTags(doc *goquery.Document, data *MovieData) {
	var tags []string

	// Look for genre/tag information
	doc.Find("td, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "ジャンル") || strings.Contains(text, "カテゴリ") {
			// Extract tags
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "ジャンル") && !strings.Contains(line, "カテゴリ") && line != "" {
					// Split by common separators
					tagNames := regexp.MustCompile(`[、,/]`).Split(line, -1)
					for _, tag := range tagNames {
						tag = strings.TrimSpace(tag)
						if tag != "" {
							tags = append(tags, tag)
						}
					}
				}
			}
		}
	})

	data.Tag = tags
}

// extractOutline extracts the plot/description
func (g *GetchuScraper) extractOutline(doc *goquery.Document, data *MovieData) {
	// Look for description in various selectors
	selectors := []string{
		".item_description",
		".story",
		".outline",
		"[class*='description']",
	}

	for _, selector := range selectors {
		if outline := strings.TrimSpace(doc.Find(selector).First().Text()); outline != "" {
			// Clean up outline
			outline = regexp.MustCompile(`\s+`).ReplaceAllString(outline, " ")
			data.Outline = outline
			break
		}
	}
}

// extractExtrafanart extracts additional images
func (g *GetchuScraper) extractExtrafanart(doc *goquery.Document, data *MovieData) {
	var images []string

	// Look for sample images
	doc.Find("img[src*='sample'], img[src*='screen']").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			if strings.HasPrefix(src, "/") {
				src = "http://www.getchu.com" + src
			}
			if strings.HasPrefix(src, "http") {
				images = append(images, src)
			}
		}
	})

	data.Extrafanart = images
}

// extractNumber extracts the product number
func (g *GetchuScraper) extractNumber(doc *goquery.Document, data *MovieData, url string) {
	// Try to extract from URL
	urlRegex := regexp.MustCompile(`item\.phtml\?id=(\d+)`)
	if matches := urlRegex.FindStringSubmatch(url); len(matches) == 2 {
		data.Number = "GETCHU-" + matches[1]
		return
	}

	// Try to extract from page content
	doc.Find("td, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "品番") || strings.Contains(text, "商品番号") {
			// Extract product number
			numberRegex := regexp.MustCompile(`([A-Z0-9-]+)`)
			if matches := numberRegex.FindStringSubmatch(text); len(matches) == 2 {
				data.Number = matches[1]
			}
		}
	})

	// Fallback: use a generic number
	if data.Number == "" {
		data.Number = "GETCHU-UNKNOWN"
	}
}

// parseDate parses Japanese date format
func (g *GetchuScraper) parseDate(dateStr string) string {
	// Handle various Japanese date formats
	dateRegex := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
	if matches := dateRegex.FindStringSubmatch(dateStr); len(matches) == 4 {
		year := matches[1]
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		return fmt.Sprintf("%s-%02d-%02d", year, month, day)
	}
	return dateStr
}

// parseRuntime parses runtime from Japanese text
func (g *GetchuScraper) parseRuntime(runtimeStr string) string {
	// Extract minutes from Japanese text
	runtimeRegex := regexp.MustCompile(`(\d+)分`)
	if matches := runtimeRegex.FindStringSubmatch(runtimeStr); len(matches) == 2 {
		return matches[1]
	}
	return runtimeStr
}

// IsValidNumber checks if the number format is valid for Getchu
func (g *GetchuScraper) IsValidNumber(number string) bool {
	cleanedNumber := g.CleanNumber(number)
	// Getchu typically uses numeric IDs or specific patterns
	return regexp.MustCompile(`^(\d+|[A-Z0-9-]+)$`).MatchString(cleanedNumber)
}