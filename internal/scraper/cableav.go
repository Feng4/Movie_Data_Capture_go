package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeCableAV scrapes movie data from CableAV
func (s *Scraper) scrapeCableAV(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting CableAV scraping for number: %s", number)

	// Get number list for searching
	numberList := s.getNumberList(number)

	// Try each number variant
	for _, searchNumber := range numberList {
		searchURL := fmt.Sprintf("https://cableav.tv/?s=%s", url.QueryEscape(searchNumber))
		logger.Debug("CableAV search URL: %s", searchURL)

		resp, err := s.httpClient.Get(ctx, searchURL, nil)
		if err != nil {
			logger.Debug("Failed to fetch search page for %s: %v", searchNumber, err)
			continue
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			logger.Debug("Failed to parse search page for %s: %v", searchNumber, err)
			continue
		}

		// Find matching result
		var detailURL string
		var matchedNumber string
		doc.Find("h3.title a[href][title]").Each(func(i int, sel *goquery.Selection) {
			href, _ := sel.Attr("href")
			title, _ := sel.Attr("title")
			if href != "" && title != "" {
				// Check if title matches any number in our list
				for _, n := range numberList {
					tempN := regexp.MustCompile(`[\W_]`).ReplaceAllString(strings.ToUpper(n), "")
					tempTitle := regexp.MustCompile(`[\W_]`).ReplaceAllString(strings.ToUpper(title), "")
					if strings.Contains(tempTitle, tempN) {
						detailURL = href
						matchedNumber = n
						return
					}
				}
			}
		})

		if detailURL != "" {
			return s.scrapeCableAVDetail(ctx, detailURL, matchedNumber)
		}
	}

	return nil, fmt.Errorf("movie not found on CableAV")
}

// scrapeCableAVDetail scrapes detailed movie data from CableAV detail page
func (s *Scraper) scrapeCableAVDetail(ctx context.Context, detailURL, number string) (*MovieData, error) {
	logger.Debug("CableAV detail URL: %s", detailURL)

	resp, err := s.httpClient.Get(ctx, detailURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch detail page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse detail page: %w", err)
	}

	movieData := &MovieData{
		Number:  number,
		Source:  "cableav",
		Website: detailURL,
	}

	// Extract title from entry content
	titleText := ""
	doc.Find(".entry-content p").Each(func(i int, sel *goquery.Selection) {
		text := sel.Text()
		if text != "" {
			titleText = text
			return
		}
	})

	if titleText != "" {
		// Remove number from title
		title := strings.ReplaceAll(titleText, number+" ", "")
		title = strings.TrimSpace(title)
		movieData.Title = title
		movieData.OriginalTitle = title
	} else {
		movieData.Title = number
		movieData.OriginalTitle = number
	}

	// Extract cover from og:image meta tag
	cover, exists := doc.Find("meta[property='og:image']").Attr("content")
	if exists {
		movieData.Cover = cover
	}

	// Extract tags from categories
	var tags []string
	doc.Find(".categories-wrap a").Each(func(i int, sel *goquery.Selection) {
		tagName := strings.TrimSpace(sel.Text())
		if tagName != "" {
			// Convert traditional Chinese to simplified (basic conversion)
			tagName = s.convertToSimplified(tagName)
			tags = append(tags, tagName)
		}
	})
	movieData.Tag = tags

	// For CableAV, actor information needs to be extracted from title or other sources
	// This is a simplified implementation - in practice, you might need more sophisticated parsing
	actors := s.extractActorFromTitle(movieData.Title)
	movieData.ActorList = actors
	movieData.Actor = strings.Join(actors, ",")

	// Create actor photo map (empty for now)
	actorPhoto := make(map[string]string)
	for _, actor := range actors {
		actorPhoto[actor] = ""
	}
	movieData.ActorPhoto = actorPhoto

	// Set as uncensored (CableAV typically hosts uncensored content)
	movieData.Uncensored = true

	// Validate required fields
	if movieData.Title == "" || movieData.Title == number {
		return nil, fmt.Errorf("no valid title found")
	}

	return movieData, nil
}

// getNumberList generates a list of number variants for searching
func (s *Scraper) getNumberList(number string) []string {
	numberList := []string{number}
	
	// Add variants without special characters
	cleanNumber := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(number, "")
	if cleanNumber != number {
		numberList = append(numberList, cleanNumber)
	}
	
	// Add uppercase variant
	upperNumber := strings.ToUpper(number)
	if upperNumber != number {
		numberList = append(numberList, upperNumber)
	}
	
	return numberList
}

// convertToSimplified performs basic traditional to simplified Chinese conversion
func (s *Scraper) convertToSimplified(text string) string {
	// Basic conversion map - in practice, you might want to use a proper library
	conversionMap := map[string]string{
		"國產": "国产",
		"無碼": "无码",
		"有碼": "有码",
		"亞洲": "亚洲",
		"歐美": "欧美",
		"動漫": "动漫",
		"三級": "三级",
	}
	
	result := text
	for traditional, simplified := range conversionMap {
		result = strings.ReplaceAll(result, traditional, simplified)
	}
	
	return result
}

// extractActorFromTitle attempts to extract actor names from the title
func (s *Scraper) extractActorFromTitle(title string) []string {
	// This is a simplified implementation
	// In practice, you might need more sophisticated parsing or external data
	var actors []string
	
	// Look for common patterns in Chinese adult video titles
	// This is a basic implementation and might need refinement
	re := regexp.MustCompile(`([\p{Han}]{2,4})(?:主演|出演|女優|女优)`)
	matches := re.FindAllStringSubmatch(title, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			actors = append(actors, match[1])
		}
	}
	
	return actors
}