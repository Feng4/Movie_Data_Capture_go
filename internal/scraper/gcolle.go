package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/httpclient"
)

// GColleScraper implements the Scraper interface for GColle
type GColleScraper struct {
	BaseURL    string
	httpClient *httpclient.Client
}

// NewGColleScraper creates a new GColleScraper instance
func NewGColleScraper(httpClient *httpclient.Client) *GColleScraper {
	return &GColleScraper{
		BaseURL:    "https://gcolle.net",
		httpClient: httpClient,
	}
}

// GetName returns the scraper name
func (g *GColleScraper) GetName() string {
	return "gcolle"
}

// ScrapeByNumber scrapes movie data by number
func (g *GColleScraper) ScrapeByNumber(ctx context.Context, number string) (*MovieData, error) {
	return g.Search(number)
}

// Search searches for movie data by number
func (g *GColleScraper) Search(number string) (*MovieData, error) {
	ctx := context.Background()
	// Clean the number for GColle format
	cleanNumber := g.cleanNumber(number)
	if cleanNumber == "" {
		return nil, fmt.Errorf("invalid number format for gcolle: %s", number)
	}

	// Build search URL
	searchURL := fmt.Sprintf("%s/search?keyword=%s", g.BaseURL, url.QueryEscape(cleanNumber))

	// Fetch the page
	doc, err := fetchDocument(ctx, g.httpClient, searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gcolle page: %w", err)
	}

	// Check if page exists
	if doc.Find(".error-page").Length() > 0 || doc.Find(".not-found").Length() > 0 {
		return nil, fmt.Errorf("product not found: %s", number)
	}

	// Parse the movie data
	movieData, err := g.parseMovieData(doc, cleanNumber, searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gcolle movie data: %w", err)
	}

	return movieData, nil
}

// cleanNumber cleans and formats the number for GColle
func (g *GColleScraper) cleanNumber(number string) string {
	// Remove common prefixes
	number = strings.ToUpper(number)
	number = strings.TrimPrefix(number, "GCOLLE-")
	number = strings.TrimPrefix(number, "GC-")

	// GColle numbers are typically numeric IDs
	re := regexp.MustCompile(`^(\d+)$`)
	if re.MatchString(number) {
		return number
	}

	// Try to extract number from mixed format
	re2 := regexp.MustCompile(`(\d+)`)
	matches := re2.FindStringSubmatch(number)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// parseMovieData parses movie data from the HTML document
func (g *GColleScraper) parseMovieData(doc *goquery.Document, number, searchURL string) (*MovieData, error) {
	movieData := &MovieData{
		Number:  number,
		Website: searchURL,
	}

	// Extract title
	title := doc.Find(".product-title h1").Text()
	if title == "" {
		title = doc.Find("h1.title").Text()
	}
	if title == "" {
		title = doc.Find(".title h1").Text()
	}
	movieData.Title = strings.TrimSpace(title)

	// Extract cover image
	coverImg := doc.Find(".product-image img").AttrOr("src", "")
	if coverImg == "" {
		coverImg = doc.Find(".main-image img").AttrOr("src", "")
	}
	if coverImg == "" {
		coverImg = doc.Find(".product-thumb img").AttrOr("src", "")
	}
	if coverImg != "" && !strings.HasPrefix(coverImg, "http") {
		movieData.Cover = g.BaseURL + coverImg
	} else {
		movieData.Cover = coverImg
	}

	// Extract release date
	releaseText := doc.Find(".product-info").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Text(), "販売日") || strings.Contains(s.Text(), "Release")
	}).Text()
	
	if releaseText == "" {
		releaseText = doc.Find(".release-date").Text()
	}
	
	if releaseText != "" {
		if releaseDate, err := g.parseDate(releaseText); err == nil {
			movieData.Release = releaseDate.Format("2006-01-02")
			movieData.Year = strconv.Itoa(releaseDate.Year())
		}
	}

	// Extract studio/seller
	studio := doc.Find(".seller-name a").Text()
	if studio == "" {
		studio = doc.Find(".product-seller a").Text()
	}
	if studio == "" {
		studio = doc.Find(".seller a").Text()
	}
	movieData.Studio = strings.TrimSpace(studio)

	// Extract series (if available)
	series := doc.Find(".product-series a").Text()
	if series == "" {
		series = doc.Find(".series a").Text()
	}
	movieData.Series = strings.TrimSpace(series)

	// Extract tags/categories
	tags := []string{}
	doc.Find(".product-tags a, .tags a, .categories a").Each(func(i int, s *goquery.Selection) {
		tagName := strings.TrimSpace(s.Text())
		if tagName != "" {
			tags = append(tags, tagName)
		}
	})
	movieData.Tag = tags

	// Extract outline/description
	outline := doc.Find(".product-description").Text()
	if outline == "" {
		outline = doc.Find(".description").Text()
	}
	if outline == "" {
		outline = doc.Find(".product-detail").Text()
	}
	movieData.Outline = strings.TrimSpace(outline)

	// Extract extrafanart images
	extrafanart := []string{}
	doc.Find(".product-gallery img, .gallery img, .preview-images img").Each(func(i int, s *goquery.Selection) {
		imgSrc := s.AttrOr("src", "")
		if imgSrc == "" {
			imgSrc = s.AttrOr("data-src", "")
		}
		if imgSrc == "" {
			imgSrc = s.AttrOr("data-lazy", "")
		}
		if imgSrc != "" {
			if !strings.HasPrefix(imgSrc, "http") {
				imgSrc = g.BaseURL + imgSrc
			}
			// Skip if it's the same as cover
			if imgSrc != movieData.Cover {
				extrafanart = append(extrafanart, imgSrc)
			}
		}
	})
	movieData.Extrafanart = extrafanart

	// Extract runtime (if available)
	runtimeText := doc.Find(".product-info").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Text(), "時間") || strings.Contains(s.Text(), "Duration") || strings.Contains(s.Text(), "再生時間")
	}).Text()
	
	if runtime := g.parseRuntime(runtimeText); runtime > 0 {
		movieData.Runtime = strconv.Itoa(runtime)
	}

	// Extract file size (if available)
	fileSizeText := doc.Find(".product-info").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Text(), "ファイルサイズ") || strings.Contains(s.Text(), "File Size")
	}).Text()
	
	// Store file size in outline if available
	if fileSizeText != "" && movieData.Outline != "" {
		movieData.Outline += "\n\nFile Size: " + strings.TrimSpace(fileSizeText)
	}

	// Set label
	movieData.Label = "GColle"

	// Set default studio if empty
	if movieData.Studio == "" {
		movieData.Studio = "GColle"
	}

	// Validate required fields
	if movieData.Title == "" {
		return nil, fmt.Errorf("no title found for number: %s", number)
	}

	return movieData, nil
}

// parseDate parses date string in various formats
func (g *GColleScraper) parseDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	
	// Extract date from text
	re := regexp.MustCompile(`(\d{4})[年/-](\d{1,2})[月/-](\d{1,2})`)
	matches := re.FindStringSubmatch(dateStr)
	if len(matches) == 4 {
		year, _ := strconv.Atoi(matches[1])
		month, _ := strconv.Atoi(matches[2])
		day, _ := strconv.Atoi(matches[3])
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
	}
	
	// Common date formats for GColle
	formats := []string{
		"2006年01月02日",
		"2006/01/02",
		"2006-01-02",
		"Jan 02, 2006",
		"January 2, 2006",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// parseRuntime extracts runtime in minutes from text
func (g *GColleScraper) parseRuntime(runtimeText string) int {
	if runtimeText == "" {
		return 0
	}

	// Extract minutes
	re := regexp.MustCompile(`(\d+)\s*分`)
	matches := re.FindStringSubmatch(runtimeText)
	if len(matches) > 1 {
		if runtime, err := strconv.Atoi(matches[1]); err == nil {
			return runtime
		}
	}

	// Extract hours and minutes
	re2 := regexp.MustCompile(`(\d+)\s*時間\s*(\d+)\s*分`)
	matches2 := re2.FindStringSubmatch(runtimeText)
	if len(matches2) > 2 {
		hours, _ := strconv.Atoi(matches2[1])
		minutes, _ := strconv.Atoi(matches2[2])
		return hours*60 + minutes
	}

	// Extract just numbers
	re3 := regexp.MustCompile(`(\d+)`)
	matches3 := re3.FindStringSubmatch(runtimeText)
	if len(matches3) > 1 {
		if runtime, err := strconv.Atoi(matches3[1]); err == nil {
			return runtime
		}
	}

	return 0
}

// GetMovieDataByURL gets movie data by direct URL
func (g *GColleScraper) GetMovieDataByURL(rawURL string) (*MovieData, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Extract number from URL path
	re := regexp.MustCompile(`/product/(\d+)`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) < 2 {
		return nil, fmt.Errorf("unable to extract number from URL: %s", rawURL)
	}

	return g.Search(matches[1])
}

// IsValidNumber checks if the number format is valid for this scraper
func (g *GColleScraper) IsValidNumber(number string) bool {
	return g.cleanNumber(number) != ""
}

// GetSearchURL returns the search URL for a given number
func (g *GColleScraper) GetSearchURL(number string) string {
	cleanNumber := g.cleanNumber(number)
	if cleanNumber == "" {
		return ""
	}
	return fmt.Sprintf("%s/product/%s", g.BaseURL, cleanNumber)
}

// MarshalJSON implements json.Marshaler interface
func (g *GColleScraper) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":     g.GetName(),
		"base_url": g.BaseURL,
	})
}