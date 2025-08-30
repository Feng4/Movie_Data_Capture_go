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

// scrapeCNMDB scrapes movie data from CNMDB
func (s *Scraper) scrapeCNMDB(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting CNMDB scraping for number: %s", number)

	// Get number list for searching
	numberList := s.getNumberList(number)

	// Search for the movie
	searchURL := fmt.Sprintf("https://cnmdb.net/search?q=%s", url.QueryEscape(number))
	logger.Debug("CNMDB search URL: %s", searchURL)

	resp, err := s.httpClient.Get(ctx, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search page: %w", err)
	}

	// Find matching result
	var detailURL string
	var matchedNumber string
	var title string
	var cover string
	var studio string

	doc.Find(".post-item").Each(func(i int, sel *goquery.Selection) {
		titleText := sel.Find("h3 a").Text()
		if titleText != "" {
			for _, n := range numberList {
				if strings.Contains(strings.ToUpper(titleText), strings.ToUpper(n)) {
					matchedNumber = n
					title = titleText
					detailURL, _ = sel.Find("h3 a").Attr("href")
					cover, _ = sel.Find(".post-item-image img").Attr("src")
					studioURL, _ := sel.Find("a").Attr("href")
					studio = sel.Find("a span").Text()
					if strings.Contains(studioURL, "麻豆") {
						studio = "麻豆"
					}
					return
				}
			}
		}
	})

	if detailURL == "" {
		return nil, fmt.Errorf("movie not found on CNMDB")
	}

	// If we have basic info from search, use it; otherwise scrape detail page
	if title != "" {
		return s.buildCNMDBMovieData(matchedNumber, title, cover, studio, detailURL)
	}

	return s.scrapeCNMDBDetail(ctx, detailURL, matchedNumber)
}

// scrapeCNMDBDetail scrapes detailed movie data from CNMDB detail page
func (s *Scraper) scrapeCNMDBDetail(ctx context.Context, detailURL, number string) (*MovieData, error) {
	logger.Debug("CNMDB detail URL: %s", detailURL)

	resp, err := s.httpClient.Get(ctx, detailURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch detail page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse detail page: %w", err)
	}

	// Extract info from breadcrumb
	var breadcrumbItems []string
	doc.Find(".breadcrumb").Contents().Each(func(i int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		if text != "" && text != ">" {
			breadcrumbItems = append(breadcrumbItems, text)
		}
	})

	if len(breadcrumbItems) == 0 {
		return nil, fmt.Errorf("no breadcrumb information found")
	}

	// Extract number from URL
	urlParts := strings.Split(detailURL, "/")
	if len(urlParts) > 0 {
		number, _ = url.QueryUnescape(urlParts[len(urlParts)-1])
	}

	title := breadcrumbItems[len(breadcrumbItems)-1]
	studio := "麻豆"
	if len(breadcrumbItems) > 1 {
		if strings.Contains(breadcrumbItems[1], "麻豆") {
			studio = "麻豆"
		} else {
			studio = breadcrumbItems[len(breadcrumbItems)-2]
		}
	}

	// Extract cover
	cover, _ := doc.Find(".post-image-inner img").Attr("src")

	return s.buildCNMDBMovieData(number, title, cover, studio, detailURL)
}

// buildCNMDBMovieData builds MovieData from extracted information
func (s *Scraper) buildCNMDBMovieData(number, title, cover, studio, detailURL string) (*MovieData, error) {
	// Parse title to extract actor and series information
	parsedTitle, parsedNumber, actors, series := s.parseActorTitle(title, number, studio)

	movieData := &MovieData{
		Number:        parsedNumber,
		Title:         parsedTitle,
		OriginalTitle: parsedTitle,
		Actor:         strings.Join(actors, ","),
		ActorList:     actors,
		Studio:        studio,
		Series:        series,
		Cover:         cover,
		Source:        "cnmdb",
		Website:       detailURL,
		Uncensored:    true, // CNMDB typically hosts uncensored content
	}

	// Create actor photo map (empty for now)
	actorPhoto := make(map[string]string)
	for _, actor := range actors {
		actorPhoto[actor] = ""
	}
	movieData.ActorPhoto = actorPhoto

	// Validate required fields
	if movieData.Title == "" {
		return nil, fmt.Errorf("no valid title found")
	}

	return movieData, nil
}

// parseActorTitle parses title to extract actor names, series, and clean title
func (s *Scraper) parseActorTitle(title, number, studio string) (string, string, []string, string) {
	// Split title by common delimiters
	re := regexp.MustCompile(`[\.,\s]+`)
	tempList := re.Split(strings.ReplaceAll(title, "/", "."), -1)

	var actorList []string
	newTitle := ""
	series := ""
	parsedNumber := number

	for i, part := range tempList {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this part contains the number
		if strings.Contains(strings.ToUpper(part), strings.ToUpper(number)) {
			parsedNumber = part
			continue
		}

		// Check if this part is series info
		if strings.Contains(part, "系列") {
			series = part
			continue
		}

		// Skip studio-related parts at the beginning
		if i < 2 && (strings.Contains(part, "传媒") || strings.Contains(part, studio)) {
			continue
		}

		// Stop if we encounter studio-related parts later
		if i > 2 && (part == studio || strings.Contains(part, "麻豆") || 
			strings.Contains(part, "出品") || strings.Contains(part, "传媒")) {
			break
		}

		// Extract potential actor names (short names at the beginning)
		if i < 3 && len(part) <= 4 && len(actorList) < 1 {
			actorList = append(actorList, part)
			continue
		}

		// Extract other potential actor names (short names)
		if len(part) <= 3 && len(part) > 1 {
			actorList = append(actorList, part)
			continue
		}

		// Add to title
		if newTitle != "" {
			newTitle += "." + part
		} else {
			newTitle = part
		}
	}

	if newTitle != "" {
		title = newTitle
	}

	// Clean up title
	title = strings.Trim(title, ".")

	return title, parsedNumber, actorList, series
}