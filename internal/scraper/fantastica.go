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

// scrapeFantastica scrapes movie data from Fantastica
func (s *Scraper) scrapeFantastica(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting Fantastica scraping for number: %s", number)

	// Search for the movie
	searchURL := fmt.Sprintf("https://fantastica-vr.com/search?q=%s", url.QueryEscape(number))
	logger.Debug("Fantastica search URL: %s", searchURL)

	resp, err := s.httpClient.Get(ctx, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search page: %w", err)
	}

	// Find the detail URL
	var detailURL string
	doc.Find("a[href*='/detail/']").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if exists {
			// Check if this result matches our number
			title := sel.Text()
			if strings.Contains(strings.ToUpper(title), strings.ToUpper(number)) {
				if strings.HasPrefix(href, "/") {
					detailURL = "https://fantastica-vr.com" + href
				} else {
					detailURL = href
				}
				return
			}
		}
	})

	if detailURL == "" {
		return nil, fmt.Errorf("movie not found on Fantastica")
	}

	return s.scrapeFantasticaDetail(ctx, detailURL, number)
}

// scrapeFantasticaDetail scrapes detailed movie data from Fantastica detail page
func (s *Scraper) scrapeFantasticaDetail(ctx context.Context, detailURL, number string) (*MovieData, error) {
	logger.Debug("Fantastica detail URL: %s", detailURL)

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
		Source:  "fantastica",
		Website: detailURL,
	}

	// Extract web number (作品番号)
	doc.Find("dt").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "作品番号") {
			webNumber := sel.Next().Text()
			if webNumber != "" {
				movieData.Number = strings.TrimSpace(webNumber)
			}
		}
	})

	// Extract title
	title := doc.Find(".title-area h2").Text()
	movieData.Title = strings.TrimSpace(title)
	movieData.OriginalTitle = movieData.Title

	// Extract actors
	var actors []string
	doc.Find("th").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "出演者") {
			sel.Next().Find("*").Each(func(j int, actorSel *goquery.Selection) {
				actorName := strings.TrimSpace(actorSel.Text())
				if actorName != "" {
					actors = append(actors, actorName)
				}
			})
		}
	})
	movieData.ActorList = actors
	movieData.Actor = strings.Join(actors, ",")

	// Create actor photo map (empty for now)
	actorPhoto := make(map[string]string)
	for _, actor := range actors {
		actorPhoto[actor] = ""
	}
	movieData.ActorPhoto = actorPhoto

	// Extract release date
	doc.Find("th").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "発売日") {
			releaseText := sel.Next().Text()
			if releaseText != "" {
				// Convert Japanese date format to standard format
				release := strings.ReplaceAll(releaseText, "年", "-")
				release = strings.ReplaceAll(release, "月", "-")
				release = strings.ReplaceAll(release, "日", "")
				movieData.Release = strings.TrimSpace(release)
				
				// Extract year
				re := regexp.MustCompile(`\d{4}`)
				year := re.FindString(release)
				if year != "" {
					movieData.Year = year
				}
			}
		}
	})

	// Extract runtime
	doc.Find("th").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "収録時間") {
			runtimeText := sel.Next().Text()
			if runtimeText != "" {
				runtime := strings.ReplaceAll(runtimeText, "分", "")
				movieData.Runtime = strings.TrimSpace(runtime)
			}
		}
	})

	// Extract tags
	var tags []string
	doc.Find("th").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "ジャンル") {
			sel.Next().Find("a").Each(func(j int, tagSel *goquery.Selection) {
				tagName := strings.TrimSpace(tagSel.Text())
				if tagName != "" {
					tags = append(tags, tagName)
				}
			})
		}
	})
	movieData.Tag = tags

	// Extract series
	doc.Find("th").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "シリーズ") {
			seriesText := sel.Next().Text()
			movieData.Series = strings.TrimSpace(seriesText)
		}
	})

	// Extract cover
	cover, exists := doc.Find(".vr_wrapper .img img").Attr("src")
	if exists {
		// Skip dummy images
		if !strings.Contains(cover, "dummy_large_white.jpg") {
			if strings.HasPrefix(cover, "/") {
				cover = "https://fantastica-vr.com" + cover
			}
			movieData.Cover = cover
		}
	}

	// Extract extra fanart
	var extrafanart []string
	doc.Find(".vr_images .vr_image a").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if exists {
			if strings.HasPrefix(href, "/") {
				href = "https://fantastica-vr.com" + href
			}
			extrafanart = append(extrafanart, href)
		}
	})
	movieData.Extrafanart = extrafanart

	// Extract outline
	outline := doc.Find(".outline").Text()
	if outline == "" {
		// Try alternative selector
		outline = doc.Find(".description").Text()
	}
	movieData.Outline = strings.TrimSpace(outline)

	// Validate required fields
	if movieData.Title == "" {
		return nil, fmt.Errorf("no valid title found")
	}

	return movieData, nil
}