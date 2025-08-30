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

// CaribPRScraper implements the Scraper interface for CaribbeancomPR
type CaribPRScraper struct {
	BaseURL    string
	httpClient *httpclient.Client
}

// NewCaribPRScraper creates a new CaribPRScraper instance
func NewCaribPRScraper(httpClient *httpclient.Client) *CaribPRScraper {
	return &CaribPRScraper{
		BaseURL:    "https://www.caribbeancompr.com",
		httpClient: httpClient,
	}
}

// GetName returns the scraper name
func (c *CaribPRScraper) GetName() string {
	return "caribpr"
}

// ScrapeByNumber scrapes movie data by number
func (c *CaribPRScraper) ScrapeByNumber(ctx context.Context, number string) (*MovieData, error) {
	return c.Search(number)
}

// Search searches for movie data by number
func (c *CaribPRScraper) Search(number string) (*MovieData, error) {
	ctx := context.Background()
	// Clean the number for CaribPR format
	cleanNumber := c.cleanNumber(number)
	if cleanNumber == "" {
		return nil, fmt.Errorf("invalid number format for caribpr: %s", number)
	}

	// Build search URL
	searchURL := fmt.Sprintf("%s/moviepages/%s/index.html", c.BaseURL, cleanNumber)

	// Fetch the page
	doc, err := fetchDocument(ctx, c.httpClient, searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch caribpr page: %w", err)
	}

	// Parse the movie data
	movieData, err := c.parseMovieData(doc, cleanNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to parse caribpr movie data: %w", err)
	}

	return movieData, nil
}

// cleanNumber cleans and formats the number for CaribPR
func (c *CaribPRScraper) cleanNumber(number string) string {
	// Remove common prefixes
	number = strings.ToUpper(number)
	number = strings.TrimPrefix(number, "CARIBPR-")
	number = strings.TrimPrefix(number, "CARIB-PR-")
	number = strings.TrimPrefix(number, "CARIBBEAN-PR-")
	number = strings.TrimPrefix(number, "CARIBBEANPR-")
	number = strings.TrimPrefix(number, "CARIBBEANCOMPR-")

	// CaribPR numbers are typically in format: YYMMDD_XXX
	// Example: 010121_001
	re := regexp.MustCompile(`^(\d{6})[-_]?(\d{3})$`)
	matches := re.FindStringSubmatch(number)
	if len(matches) == 3 {
		return fmt.Sprintf("%s_%s", matches[1], matches[2])
	}

	// Try simple 6-digit format
	re2 := regexp.MustCompile(`^(\d{6})$`)
	if re2.MatchString(number) {
		return fmt.Sprintf("%s_001", number)
	}

	return ""
}

// parseMovieData parses movie data from the HTML document
func (c *CaribPRScraper) parseMovieData(doc *goquery.Document, number string) (*MovieData, error) {
	movieData := &MovieData{
		Number:  number,
		Website: c.BaseURL,
	}

	// Extract title
	title := doc.Find("h1[itemprop='name']").Text()
	if title == "" {
		title = doc.Find(".movie-title h1").Text()
	}
	if title == "" {
		title = doc.Find(".detail-title h1").Text()
	}
	movieData.Title = strings.TrimSpace(title)

	// Extract cover image
	coverImg := doc.Find(".movie-image img").AttrOr("src", "")
	if coverImg == "" {
		coverImg = doc.Find(".detail-image img").AttrOr("src", "")
	}
	if coverImg == "" {
		coverImg = doc.Find(".main-image img").AttrOr("src", "")
	}
	if coverImg != "" && !strings.HasPrefix(coverImg, "http") {
		movieData.Cover = c.BaseURL + coverImg
	} else {
		movieData.Cover = coverImg
	}

	// Extract release date
	releaseText := doc.Find(".movie-info .release-date").Text()
	if releaseText == "" {
		releaseText = doc.Find(".detail-info .release").Text()
	}
	if releaseText == "" {
		releaseText = doc.Find(".info-list .release").Text()
	}
	if releaseText != "" {
		if releaseDate, err := c.parseDate(releaseText); err == nil {
			movieData.Release = releaseDate.Format("2006-01-02")
			movieData.Year = strconv.Itoa(releaseDate.Year())
		}
	}

	// Extract runtime
	runtimeText := doc.Find(".movie-info .duration").Text()
	if runtimeText == "" {
		runtimeText = doc.Find(".detail-info .duration").Text()
	}
	if runtimeText == "" {
		runtimeText = doc.Find(".info-list .duration").Text()
	}
	if runtime := c.parseRuntime(runtimeText); runtime > 0 {
		movieData.Runtime = strconv.Itoa(runtime)
	}

	// Extract director
	director := doc.Find(".movie-info .director a").Text()
	if director == "" {
		director = doc.Find(".detail-info .director a").Text()
	}
	if director == "" {
		director = doc.Find(".info-list .director a").Text()
	}
	movieData.Director = strings.TrimSpace(director)

	// Extract studio
	studio := doc.Find(".movie-info .studio a").Text()
	if studio == "" {
		studio = doc.Find(".detail-info .studio a").Text()
	}
	if studio == "" {
		studio = doc.Find(".info-list .studio a").Text()
	}
	if studio == "" {
		studio = "CaribbeancomPR"
	}
	movieData.Studio = strings.TrimSpace(studio)

	// Extract actors
	actors := []string{}
	doc.Find(".movie-info .cast a, .detail-info .cast a, .info-list .cast a").Each(func(i int, s *goquery.Selection) {
		actorName := strings.TrimSpace(s.Text())
		if actorName != "" {
			actors = append(actors, actorName)
		}
	})
	movieData.Actor = joinActors(actors)
	movieData.ActorList = actors

	// Extract tags/genres
	tags := []string{}
	doc.Find(".movie-info .tags a, .detail-info .tags a, .info-list .tags a").Each(func(i int, s *goquery.Selection) {
		tagName := strings.TrimSpace(s.Text())
		if tagName != "" {
			tags = append(tags, tagName)
		}
	})
	movieData.Tag = tags

	// Extract outline/description
	outline := doc.Find(".movie-comment, .detail-comment, .description").Text()
	movieData.Outline = strings.TrimSpace(outline)

	// Extract extrafanart images
	extrafanart := []string{}
	doc.Find(".movie-gallery img, .detail-gallery img, .gallery img").Each(func(i int, s *goquery.Selection) {
		imgSrc := s.AttrOr("src", "")
		if imgSrc == "" {
			imgSrc = s.AttrOr("data-src", "")
		}
		if imgSrc == "" {
			imgSrc = s.AttrOr("data-lazy", "")
		}
		if imgSrc != "" {
			if !strings.HasPrefix(imgSrc, "http") {
				imgSrc = c.BaseURL + imgSrc
			}
			extrafanart = append(extrafanart, imgSrc)
		}
	})
	movieData.Extrafanart = extrafanart

	// Set series
	movieData.Series = "CaribbeancomPR"

	// Set label
	movieData.Label = "CaribbeancomPR"

	// Validate required fields
	if movieData.Title == "" {
		return nil, fmt.Errorf("no title found for number: %s", number)
	}

	return movieData, nil
}

// parseDate parses date string in various formats
func (c *CaribPRScraper) parseDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	
	// Common date formats for CaribPR
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"2006年01月02日",
		"January 2, 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// parseRuntime extracts runtime in minutes from text
func (c *CaribPRScraper) parseRuntime(runtimeText string) int {
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
func (c *CaribPRScraper) GetMovieDataByURL(rawURL string) (*MovieData, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Extract number from URL path
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	var number string
	for _, part := range pathParts {
		if matched, _ := regexp.MatchString(`\d{6}_\d{3}`, part); matched {
			number = part
			break
		}
	}

	if number == "" {
		return nil, fmt.Errorf("unable to extract number from URL: %s", rawURL)
	}

	return c.Search(number)
}

// IsValidNumber checks if the number format is valid for this scraper
func (c *CaribPRScraper) IsValidNumber(number string) bool {
	return c.cleanNumber(number) != ""
}

// GetSearchURL returns the search URL for a given number
func (c *CaribPRScraper) GetSearchURL(number string) string {
	cleanNumber := c.cleanNumber(number)
	if cleanNumber == "" {
		return ""
	}
	return fmt.Sprintf("%s/moviepages/%s/index.html", c.BaseURL, cleanNumber)
}

// MarshalJSON implements json.Marshaler interface
func (c *CaribPRScraper) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":     c.GetName(),
		"base_url": c.BaseURL,
	})
}