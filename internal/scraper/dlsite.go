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

// DLSiteScraper implements the Scraper interface for DLSite
type DLSiteScraper struct {
	BaseURL    string
	httpClient *httpclient.Client
}

// NewDLSiteScraper creates a new DLSiteScraper instance
func NewDLSiteScraper(httpClient *httpclient.Client) *DLSiteScraper {
	return &DLSiteScraper{
		BaseURL:    "https://www.dlsite.com",
		httpClient: httpClient,
	}
}

// GetName returns the scraper name
func (d *DLSiteScraper) GetName() string {
	return "dlsite"
}

// ScrapeByNumber scrapes movie data by number
func (d *DLSiteScraper) ScrapeByNumber(ctx context.Context, number string) (*MovieData, error) {
	return d.Search(number)
}

// Search searches for movie data by number
func (d *DLSiteScraper) Search(number string) (*MovieData, error) {
	ctx := context.Background()
	// Clean the number for DLSite format
	cleanNumber := d.cleanNumber(number)
	if cleanNumber == "" {
		return nil, fmt.Errorf("invalid number format for dlsite: %s", number)
	}

	// Build search URL - try different language versions
	searchURLs := []string{
		fmt.Sprintf("%s/maniax/work/=/product_id/%s.html", d.BaseURL, cleanNumber),
		fmt.Sprintf("%s/home/work/=/product_id/%s.html", d.BaseURL, cleanNumber),
		fmt.Sprintf("%s/pro/work/=/product_id/%s.html", d.BaseURL, cleanNumber),
	}

	var lastErr error
	for _, searchURL := range searchURLs {
		// Fetch the page
		doc, err := fetchDocument(ctx, d.httpClient, searchURL)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if page exists (not 404)
		if doc.Find(".error_404").Length() > 0 {
			continue
		}

		// Parse the movie data
		movieData, err := d.parseMovieData(doc, cleanNumber, searchURL)
		if err != nil {
			lastErr = err
			continue
		}

		return movieData, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to fetch dlsite page: %w", lastErr)
	}
	return nil, fmt.Errorf("no valid page found for number: %s", number)
}

// cleanNumber cleans and formats the number for DLSite
func (d *DLSiteScraper) cleanNumber(number string) string {
	// Remove common prefixes
	number = strings.ToUpper(number)
	number = strings.TrimPrefix(number, "DLSITE-")
	number = strings.TrimPrefix(number, "DL-")

	// DLSite numbers are typically in format: RJ123456 or VJ123456
	re := regexp.MustCompile(`^([A-Z]{2})(\d{6,8})$`)
	matches := re.FindStringSubmatch(number)
	if len(matches) == 3 {
		return fmt.Sprintf("%s%s", matches[1], matches[2])
	}

	// Try without prefix
	re2 := regexp.MustCompile(`^(\d{6,8})$`)
	if re2.MatchString(number) {
		return "RJ" + number
	}

	return ""
}

// parseMovieData parses movie data from the HTML document
func (d *DLSiteScraper) parseMovieData(doc *goquery.Document, number, searchURL string) (*MovieData, error) {
	movieData := &MovieData{
		Number:  number,
		Website: searchURL,
	}

	// Extract title
	title := doc.Find("#work_name a").Text()
	if title == "" {
		title = doc.Find(".work_name").Text()
	}
	if title == "" {
		title = doc.Find("h1#work_name").Text()
	}
	movieData.Title = strings.TrimSpace(title)

	// Extract cover image
	coverImg := doc.Find(".product-slider-data img").AttrOr("src", "")
	if coverImg == "" {
		coverImg = doc.Find(".slider_item img").AttrOr("src", "")
	}
	if coverImg == "" {
		coverImg = doc.Find(".work_img img").AttrOr("src", "")
	}
	if coverImg != "" && !strings.HasPrefix(coverImg, "http") {
		movieData.Cover = d.BaseURL + coverImg
	} else {
		movieData.Cover = coverImg
	}

	// Extract release date
	releaseText := doc.Find("#work_outline tr").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Find("th").Text(), "販売日") || strings.Contains(s.Find("th").Text(), "Release")
	}).Find("td").Text()
	
	if releaseText != "" {
		if releaseDate, err := d.parseDate(releaseText); err == nil {
			movieData.Release = releaseDate.Format("2006-01-02")
			movieData.Year = strconv.Itoa(releaseDate.Year())
		}
	}

	// Extract studio/circle
	studio := doc.Find("#work_outline tr").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Find("th").Text(), "サークル名") || strings.Contains(s.Find("th").Text(), "Circle")
	}).Find("td a").Text()
	
	if studio == "" {
		studio = doc.Find(".maker_name a").Text()
	}
	movieData.Studio = strings.TrimSpace(studio)

	// Extract series
	series := doc.Find("#work_outline tr").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Find("th").Text(), "シリーズ名") || strings.Contains(s.Find("th").Text(), "Series")
	}).Find("td a").Text()
	movieData.Series = strings.TrimSpace(series)

	// Extract voice actors
	actors := []string{}
	doc.Find("#work_outline tr").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Find("th").Text(), "声優") || strings.Contains(s.Find("th").Text(), "Voice")
	}).Find("td a").Each(func(i int, s *goquery.Selection) {
		actorName := strings.TrimSpace(s.Text())
		if actorName != "" {
			actors = append(actors, actorName)
		}
	})
	movieData.Actor = joinActors(actors)
	movieData.ActorList = actors

	// Extract tags/genres
	tags := []string{}
	doc.Find("#work_outline tr").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Find("th").Text(), "ジャンル") || strings.Contains(s.Find("th").Text(), "Genre")
	}).Find("td a").Each(func(i int, s *goquery.Selection) {
		tagName := strings.TrimSpace(s.Text())
		if tagName != "" {
			tags = append(tags, tagName)
		}
	})
	movieData.Tag = tags

	// Extract outline/description
	outline := doc.Find(".work_parts_container").Text()
	if outline == "" {
		outline = doc.Find(".work_article").Text()
	}
	if outline == "" {
		outline = doc.Find(".summary").Text()
	}
	movieData.Outline = strings.TrimSpace(outline)

	// Extract extrafanart images
	extrafanart := []string{}
	doc.Find(".product-slider-data img, .slider_item img").Each(func(i int, s *goquery.Selection) {
		imgSrc := s.AttrOr("src", "")
		if imgSrc == "" {
			imgSrc = s.AttrOr("data-src", "")
		}
		if imgSrc != "" {
			if !strings.HasPrefix(imgSrc, "http") {
				imgSrc = d.BaseURL + imgSrc
			}
			// Skip if it's the same as cover
			if imgSrc != movieData.Cover {
				extrafanart = append(extrafanart, imgSrc)
			}
		}
	})
	movieData.Extrafanart = extrafanart

	// Extract runtime (if available)
	runtimeText := doc.Find("#work_outline tr").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(s.Find("th").Text(), "時間") || strings.Contains(s.Find("th").Text(), "Duration")
	}).Find("td").Text()
	
	if runtime := d.parseRuntime(runtimeText); runtime > 0 {
		movieData.Runtime = strconv.Itoa(runtime)
	}

	// Set label
	movieData.Label = "DLSite"

	// Validate required fields
	if movieData.Title == "" {
		return nil, fmt.Errorf("no title found for number: %s", number)
	}

	return movieData, nil
}

// parseDate parses date string in various formats
func (d *DLSiteScraper) parseDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	
	// Common date formats for DLSite
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
func (d *DLSiteScraper) parseRuntime(runtimeText string) int {
	if runtimeText == "" {
		return 0
	}

	// Extract numbers from runtime text
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindStringSubmatch(runtimeText)
	if len(matches) > 1 {
		if runtime, err := strconv.Atoi(matches[1]); err == nil {
			return runtime
		}
	}

	return 0
}

// GetMovieDataByURL gets movie data by direct URL
func (d *DLSiteScraper) GetMovieDataByURL(rawURL string) (*MovieData, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Extract number from URL path
	re := regexp.MustCompile(`/product_id/([A-Z]{2}\d{6,8})\.html`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) < 2 {
		return nil, fmt.Errorf("unable to extract number from URL: %s", rawURL)
	}

	return d.Search(matches[1])
}

// IsValidNumber checks if the number format is valid for this scraper
func (d *DLSiteScraper) IsValidNumber(number string) bool {
	return d.cleanNumber(number) != ""
}

// GetSearchURL returns the search URL for a given number
func (d *DLSiteScraper) GetSearchURL(number string) string {
	cleanNumber := d.cleanNumber(number)
	if cleanNumber == "" {
		return ""
	}
	return fmt.Sprintf("%s/maniax/work/=/product_id/%s.html", d.BaseURL, cleanNumber)
}

// MarshalJSON implements json.Marshaler interface
func (d *DLSiteScraper) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":     d.GetName(),
		"base_url": d.BaseURL,
	})
}