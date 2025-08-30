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

// MadouScraper handles scraping from Madou (麻豆传媒)
type MadouScraper struct {
	httpClient *httpclient.Client
}

// NewMadouScraper creates a new Madou scraper
func NewMadouScraper(client *httpclient.Client) *MadouScraper {
	return &MadouScraper{
		httpClient: client,
	}
}

// CleanNumber cleans and validates the number for Madou
func (m *MadouScraper) CleanNumber(number string) string {
	// Remove common prefixes and clean
	number = strings.ToUpper(strings.TrimSpace(number))
	number = regexp.MustCompile(`^(MADOU[-_]?|MD[-_]?|麻豆[-_]?)`).ReplaceAllString(number, "")
	return number
}

// ScrapeByNumber scrapes movie data by number
func (m *MadouScraper) ScrapeByNumber(ctx context.Context, number string) (*MovieData, error) {
	cleanedNumber := m.CleanNumber(number)
	logger.Debug("Scraping Madou with cleaned number: %s", cleanedNumber)

	// Try different search URLs
	searchURLs := []string{
		fmt.Sprintf("https://madou.club/search?q=%s", cleanedNumber),
		fmt.Sprintf("https://madou.tv/search?keyword=%s", cleanedNumber),
		fmt.Sprintf("https://www.madou.club/video/%s", cleanedNumber),
	}

	for _, searchURL := range searchURLs {
		resp, err := m.httpClient.Get(ctx, searchURL, nil)
		if err != nil {
			logger.Debug("Failed to access %s: %v", searchURL, err)
			continue
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			logger.Debug("Failed to parse %s: %v", searchURL, err)
			continue
		}

		// Check if this is a direct video page
		if strings.Contains(searchURL, "/video/") {
			// Try to extract data directly
			data := m.extractFromVideoPage(doc, searchURL)
			if data != nil {
				return data, nil
			}
		} else {
			// Find the first result link
			var movieURL string
			doc.Find("a[href*='/video/']").Each(func(i int, s *goquery.Selection) {
				if movieURL == "" {
					href, exists := s.Attr("href")
					if exists {
						if strings.HasPrefix(href, "/") {
							movieURL = "https://madou.club" + href
						} else if strings.HasPrefix(href, "http") {
							movieURL = href
						}
					}
				}
			})

			if movieURL != "" {
				return m.ScrapeByURL(ctx, movieURL)
			}
		}
	}

	return nil, fmt.Errorf("movie not found for number: %s", number)
}

// ScrapeByURL scrapes movie data from a specific URL
func (m *MadouScraper) ScrapeByURL(ctx context.Context, url string) (*MovieData, error) {
	resp, err := m.httpClient.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse movie page: %w", err)
	}

	return m.extractFromVideoPage(doc, url), nil
}

// extractFromVideoPage extracts data from a video page
func (m *MadouScraper) extractFromVideoPage(doc *goquery.Document, url string) *MovieData {
	data := &MovieData{
		Website: url,
		Source:  "madou",
	}

	// Extract basic information
	m.extractTitle(doc, data)
	m.extractNumber(doc, data, url)
	m.extractCover(doc, data)
	m.extractReleaseDate(doc, data)
	m.extractRuntime(doc, data)
	m.extractDirector(doc, data)
	m.extractStudio(doc, data)
	m.extractActors(doc, data)
	m.extractTags(doc, data)
	m.extractOutline(doc, data)
	m.extractExtrafanart(doc, data)

	// Validate that we have essential data
	if data.Title == "" && data.Number == "" {
		return nil
	}

	return data
}

// extractTitle extracts the movie title
func (m *MadouScraper) extractTitle(doc *goquery.Document, data *MovieData) {
	// Try different selectors for title
	selectors := []string{
		"h1.video-title",
		".video-info h1",
		"h1",
		".title",
		"title",
	}

	for _, selector := range selectors {
		if title := strings.TrimSpace(doc.Find(selector).First().Text()); title != "" {
			// Clean up title
			title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
			// Remove site name from title
			title = regexp.MustCompile(`(?i)\s*[-|]\s*(madou|麻豆).*$`).ReplaceAllString(title, "")
			data.Title = title
			break
		}
	}
}

// extractNumber extracts the movie number
func (m *MadouScraper) extractNumber(doc *goquery.Document, data *MovieData, url string) {
	// Try to extract from URL
	urlRegex := regexp.MustCompile(`/video/([A-Z0-9-]+)`)
	if matches := urlRegex.FindStringSubmatch(url); len(matches) == 2 {
		data.Number = "MD-" + matches[1]
		return
	}

	// Try to extract from page content
	doc.Find(".video-info, .info-table").Find("tr, div").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "番号") || strings.Contains(text, "编号") || strings.Contains(strings.ToLower(text), "number") {
			// Extract number from the text
			numberRegex := regexp.MustCompile(`([A-Z0-9-]+)`)
			if matches := numberRegex.FindStringSubmatch(text); len(matches) == 2 {
				data.Number = matches[1]
			}
		}
	})

	// Fallback: extract from title
	if data.Number == "" && data.Title != "" {
		numberRegex := regexp.MustCompile(`(MD[-_]?\d+|麻豆[-_]?\d+)`)
		if matches := numberRegex.FindStringSubmatch(data.Title); len(matches) == 2 {
			data.Number = strings.ToUpper(matches[1])
		}
	}

	// Final fallback: generate a number
	if data.Number == "" {
		data.Number = "MD-UNKNOWN"
	}
}

// extractCover extracts the cover image URL
func (m *MadouScraper) extractCover(doc *goquery.Document, data *MovieData) {
	// Try different selectors for cover image
	selectors := []string{
		".video-poster img",
		".poster img",
		".cover img",
		"video[poster]",
		"img[src*='poster']",
		"img[src*='cover']",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if data.Cover == "" {
				// Check for poster attribute first (video element)
				if poster, exists := s.Attr("poster"); exists {
					if strings.HasPrefix(poster, "/") {
						data.Cover = "https://madou.club" + poster
					} else if strings.HasPrefix(poster, "http") {
						data.Cover = poster
					}
				} else if src, exists := s.Attr("src"); exists {
					if strings.HasPrefix(src, "/") {
						data.Cover = "https://madou.club" + src
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
func (m *MadouScraper) extractReleaseDate(doc *goquery.Document, data *MovieData) {
	// Look for release date in various formats
	doc.Find(".video-info, .info-table").Find("tr, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "发布") || strings.Contains(text, "上传") || strings.Contains(text, "时间") || strings.Contains(strings.ToLower(text), "date") {
			// Extract date from the text
			dateRegex := regexp.MustCompile(`(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})`)
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
func (m *MadouScraper) extractRuntime(doc *goquery.Document, data *MovieData) {
	// Look for runtime information
	doc.Find(".video-info, .info-table").Find("tr, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "时长") || strings.Contains(text, "分钟") || strings.Contains(strings.ToLower(text), "duration") {
			// Extract runtime
			runtimeRegex := regexp.MustCompile(`(\d+)\s*分`)
			if matches := runtimeRegex.FindStringSubmatch(text); len(matches) == 2 {
				data.Runtime = matches[1]
			}
		}
	})

	// Also check video element duration
	doc.Find("video").Each(func(i int, s *goquery.Selection) {
		if duration, exists := s.Attr("duration"); exists {
			if durationFloat, err := strconv.ParseFloat(duration, 64); err == nil {
				minutes := int(durationFloat / 60)
				data.Runtime = strconv.Itoa(minutes)
			}
		}
	})
}

// extractDirector extracts the director
func (m *MadouScraper) extractDirector(doc *goquery.Document, data *MovieData) {
	// Look for director information
	doc.Find(".video-info, .info-table").Find("tr, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "导演") || strings.Contains(text, "監督") || strings.Contains(strings.ToLower(text), "director") {
			// Extract director name
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "导演") && !strings.Contains(line, "監督") && !strings.Contains(strings.ToLower(line), "director") && line != "" {
					data.Director = line
					break
				}
			}
		}
	})
}

// extractStudio extracts the studio/publisher
func (m *MadouScraper) extractStudio(doc *goquery.Document, data *MovieData) {
	// Madou is typically the studio
	data.Studio = "麻豆传媒 (Madou Media)"

	// Look for more specific studio information
	doc.Find(".video-info, .info-table").Find("tr, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "制作") || strings.Contains(text, "出品") || strings.Contains(strings.ToLower(text), "studio") {
			// Extract studio name
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "制作") && !strings.Contains(line, "出品") && !strings.Contains(strings.ToLower(line), "studio") && line != "" {
					data.Studio = line
					break
				}
			}
		}
	})
}

// extractActors extracts the actors
func (m *MadouScraper) extractActors(doc *goquery.Document, data *MovieData) {
	var actors []string

	// Look for actors in video info
	doc.Find(".video-info, .info-table").Find("tr, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "演员") || strings.Contains(text, "主演") || strings.Contains(strings.ToLower(text), "actress") {
			// Extract actor names
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "演员") && !strings.Contains(line, "主演") && !strings.Contains(strings.ToLower(line), "actress") && line != "" {
					// Split by common separators
					names := regexp.MustCompile(`[,、/]`).Split(line, -1)
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

	// Also look for actor links
	doc.Find("a[href*='/actress/'], a[href*='/actor/']").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Text())
		if name != "" {
			// Check if not already added
			found := false
			for _, existing := range actors {
				if strings.EqualFold(existing, name) {
					found = true
					break
				}
			}
			if !found {
				actors = append(actors, name)
			}
		}
	})

	data.ActorList = actors
	if len(actors) > 0 {
		data.Actor = strings.Join(actors, ", ")
	}
}

// extractTags extracts tags/genres
func (m *MadouScraper) extractTags(doc *goquery.Document, data *MovieData) {
	var tags []string

	// Add default tags for Madou content
	tags = append(tags, "国产", "中文", "麻豆")

	// Look for additional tags
	doc.Find(".video-info, .info-table").Find("tr, div, span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "标签") || strings.Contains(text, "分类") || strings.Contains(strings.ToLower(text), "tag") {
			// Extract tags
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, "标签") && !strings.Contains(line, "分类") && !strings.Contains(strings.ToLower(line), "tag") && line != "" {
					// Split by common separators
					tagNames := regexp.MustCompile(`[,、/]`).Split(line, -1)
					for _, tag := range tagNames {
						tag = strings.TrimSpace(tag)
						if tag != "" {
							// Check if not already added
							found := false
							for _, existing := range tags {
								if strings.EqualFold(existing, tag) {
									found = true
									break
								}
							}
							if !found {
								tags = append(tags, tag)
							}
						}
					}
				}
			}
		}
	})

	// Also look for tag links
	doc.Find("a[href*='/tag/'], a[href*='/category/']").Each(func(i int, s *goquery.Selection) {
		tag := strings.TrimSpace(s.Text())
		if tag != "" {
			// Check if not already added
			found := false
			for _, existing := range tags {
				if strings.EqualFold(existing, tag) {
					found = true
					break
				}
			}
			if !found {
				tags = append(tags, tag)
			}
		}
	})

	data.Tag = tags
}

// extractOutline extracts the plot/description
func (m *MadouScraper) extractOutline(doc *goquery.Document, data *MovieData) {
	// Look for description in various selectors
	selectors := []string{
		".video-description",
		".description",
		".plot",
		".summary",
		".intro",
	}

	for _, selector := range selectors {
		if outline := strings.TrimSpace(doc.Find(selector).First().Text()); outline != "" {
			// Clean up outline
			outline = regexp.MustCompile(`\s+`).ReplaceAllString(outline, " ")
			data.Outline = outline
			break
		}
	}

	// Also check in video info table
	if data.Outline == "" {
		doc.Find(".video-info, .info-table").Find("tr, div").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if strings.Contains(text, "简介") || strings.Contains(text, "描述") || strings.Contains(strings.ToLower(text), "description") {
				// Extract description
				lines := strings.Split(text, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if !strings.Contains(line, "简介") && !strings.Contains(line, "描述") && !strings.Contains(strings.ToLower(line), "description") && len(line) > 10 {
						data.Outline = line
						break
					}
				}
			}
		})
	}
}

// extractExtrafanart extracts additional images
func (m *MadouScraper) extractExtrafanart(doc *goquery.Document, data *MovieData) {
	var images []string

	// Look for sample images
	doc.Find(".sample-images img, .screenshots img, img[src*='sample'], img[src*='thumb']").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			if strings.HasPrefix(src, "/") {
				src = "https://madou.club" + src
			}
			if strings.HasPrefix(src, "http") && !strings.Contains(src, data.Cover) {
				images = append(images, src)
			}
		}
	})

	data.Extrafanart = images
}

// IsValidNumber checks if the number format is valid for Madou
func (m *MadouScraper) IsValidNumber(number string) bool {
	cleanedNumber := m.CleanNumber(number)
	// Madou typically uses MD- prefix or numeric IDs
	return regexp.MustCompile(`^(MD[-_]?\d+|\d+|[A-Z0-9-]+)$`).MatchString(cleanedNumber)
}