package scraper

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeFanza scrapes movie data from Fanza (DMM)
func (s *Scraper) scrapeFanza(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting Fanza scraping for number: %s", number)
	
	// Normalize Fanza search number
	fanzaSearchNumber := number
	if strings.HasPrefix(strings.ToLower(fanzaSearchNumber), "h-") {
		fanzaSearchNumber = strings.Replace(fanzaSearchNumber, "h-", "h_", 1)
	}
	
	// Remove non-alphanumeric characters except underscore
	re := regexp.MustCompile(`[^0-9a-zA-Z_]`)
	fanzaSearchNumber = strings.ToLower(re.ReplaceAllString(fanzaSearchNumber, ""))
	
	// Try multiple URL formats for Fanza
	urlsToTry := []string{
		fmt.Sprintf("https://www.dmm.co.jp/mono/dvd/-/detail/=/cid=%s/", fanzaSearchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/videoa/-/detail/=/cid=%s/", fanzaSearchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/anime/-/detail/=/cid=%s/", fanzaSearchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/mono/anime/-/detail/=/cid=%s/", fanzaSearchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/videoc/-/detail/=/cid=%s/", fanzaSearchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/digital/nikkatsu/-/detail/=/cid=%s/", fanzaSearchNumber),
		fmt.Sprintf("https://www.dmm.co.jp/rental/-/detail/=/cid=%s/", fanzaSearchNumber),
	}

	for _, detailURL := range urlsToTry {
		logger.Debug("Trying Fanza URL: %s", detailURL)
		movieData, err := s.scrapeFanzaPage(ctx, detailURL, detailURL)
		if err == nil {
			return movieData, nil
		}
		logger.Debug("Failed to scrape %s: %v", detailURL, err)
	}
	
	return nil, fmt.Errorf("no valid Fanza page found for number: %s", number)
}

// scrapeFanzaPage scrapes a specific Fanza page
func (s *Scraper) scrapeFanzaPage(ctx context.Context, ageCheckURL, originalURL string) (*MovieData, error) {
	// Extract number from originalURL for fallback
	number := extractNumberFromURL(originalURL)
	
	// Set headers to simulate browser access
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Language": "ja,en-US;q=0.7,en;q=0.3",
		"DNT": "1",
		"Connection": "keep-alive",
		"Upgrade-Insecure-Requests": "1",
	}
	
	resp, err := s.httpClient.Get(ctx, ageCheckURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Read the response body to check content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	logger.Debug("Fanza response body length: %d bytes", len(body))
	if len(body) > 0 {
		// Log first 500 characters for debugging
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		logger.Debug("Fanza response preview: %s", preview)
	}
	
	// Check if this is still the age verification page or region blocked
	bodyStr := string(body)
	if strings.Contains(bodyStr, "年齢認証") || strings.Contains(bodyStr, "Age Verification") || strings.Contains(bodyStr, "Sorry! This content is not available in your region.") {
		return nil, fmt.Errorf("still on age verification page or region blocked")
	}
	
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	// Check if page contains valid content by looking for og:title
	if doc.Find("meta[property='og:title']").Length() == 0 {
		return nil, fmt.Errorf("invalid Fanza page - no og:title found")
	}
	
	movieData := &MovieData{
		Website: originalURL,
		Source:  "fanza",
	}
	
	// Extract title from og:title or h1
	if title, exists := doc.Find("meta[property='og:title']").Attr("content"); exists {
		title = strings.TrimSpace(title)
		// Clean title by removing number and FANZA suffix
		numberRegex := regexp.MustCompile(`^[A-Z0-9-]+\s*`)
		title = numberRegex.ReplaceAllString(title, "")
		fanzaRegex := regexp.MustCompile(`\s*-\s*FANZA.*$`)
		title = fanzaRegex.ReplaceAllString(title, "")
		movieData.Title = title
	} else if title := doc.Find("h1#title").Text(); title != "" {
		movieData.Title = strings.TrimSpace(title)
	} else if title := doc.Find("h1").First().Text(); title != "" {
		movieData.Title = strings.TrimSpace(title)
	}
	
	// Extract number from page or use original
	if numberText := doc.Find("td:contains('品番')").Next().Text(); numberText != "" {
		movieData.Number = strings.ToUpper(strings.TrimSpace(numberText))
	} else if numberText := doc.Find("span.product-code").Text(); numberText != "" {
		movieData.Number = strings.ToUpper(strings.TrimSpace(numberText))
	} else {
		// Extract from original number parameter
		movieData.Number = strings.ToUpper(number)
	}
	
	// Extract cover from og:image or other sources
	if cover, exists := doc.Find("meta[property='og:image']").Attr("content"); exists {
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		} else if strings.HasPrefix(cover, "/") {
			cover = "https://www.dmm.co.jp" + cover
		}
		movieData.Cover = cover
	} else if cover, exists := doc.Find("img[name='package-image']").Attr("src"); exists {
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		} else if strings.HasPrefix(cover, "/") {
			cover = "https://www.dmm.co.jp" + cover
		}
		movieData.Cover = cover
	} else if cover, exists := doc.Find("div#sample-video a img").Attr("src"); exists {
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		} else if strings.HasPrefix(cover, "/") {
			cover = "https://www.dmm.co.jp" + cover
		}
		movieData.Cover = cover
	}
	
	// Extract release date
	if releaseText := doc.Find("td:contains('発売日')").Next().Text(); releaseText != "" {
		releaseText = strings.TrimSpace(releaseText)
		// Convert date format from YYYY/MM/DD to YYYY-MM-DD
		dateRegex := regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})`)
		if matches := dateRegex.FindStringSubmatch(releaseText); len(matches) > 3 {
			year, month, day := matches[1], matches[2], matches[3]
			if len(month) == 1 {
				month = "0" + month
			}
			if len(day) == 1 {
				day = "0" + day
			}
			movieData.Release = fmt.Sprintf("%s-%s-%s", year, month, day)
			movieData.Year = year
		}
	} else if releaseText := doc.Find("td:contains('配信開始日')").Next().Text(); releaseText != "" {
		releaseText = strings.TrimSpace(releaseText)
		// Convert date format from YYYY/MM/DD to YYYY-MM-DD
		dateRegex := regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})`)
		if matches := dateRegex.FindStringSubmatch(releaseText); len(matches) > 3 {
			year, month, day := matches[1], matches[2], matches[3]
			if len(month) == 1 {
				month = "0" + month
			}
			if len(day) == 1 {
				day = "0" + day
			}
			movieData.Release = fmt.Sprintf("%s-%s-%s", year, month, day)
			movieData.Year = year
		}
	}
	
	// Extract runtime
	if runtimeText := doc.Find("td:contains('収録時間')").Next().Text(); runtimeText != "" {
		runtimeText = strings.TrimSpace(runtimeText)
		// Extract just the number part
		runtimeRegex := regexp.MustCompile(`(\d+)`)
		if matches := runtimeRegex.FindString(runtimeText); matches != "" {
			movieData.Runtime = matches
		}
	}
	
	// Extract director
	if director := doc.Find("td:contains('監督')").Next().Find("a").Text(); director != "" {
		movieData.Director = strings.TrimSpace(director)
	}
	
	// Extract studio
	if studio := doc.Find("td:contains('メーカー')").Next().Find("a").Text(); studio != "" {
		movieData.Studio = strings.TrimSpace(studio)
	}
	
	// Extract publisher/label
	if publisher := doc.Find("td:contains('レーベル')").Next().Find("a").Text(); publisher != "" {
		movieData.Label = strings.TrimSpace(publisher)
	}
	
	// Extract series
	if series := doc.Find("td:contains('シリーズ')").Next().Find("a").Text(); series != "" {
		movieData.Series = strings.TrimSpace(series)
	}
	
	// Extract actors
	var actors []string
	doc.Find("span#performer a").Each(func(i int, s *goquery.Selection) {
		if actor := strings.TrimSpace(s.Text()); actor != "" {
			actors = append(actors, actor)
		}
	})
	if len(actors) == 0 {
		// Fallback selector
		doc.Find("td:contains('出演者')").Next().Find("a").Each(func(i int, s *goquery.Selection) {
			if actor := strings.TrimSpace(s.Text()); actor != "" {
				actors = append(actors, actor)
			}
		})
	}
	movieData.ActorList = actors
	if len(actors) > 0 {
		movieData.Actor = strings.Join(actors, ",")
	}
	
	// Extract tags
	var tags []string
	doc.Find("td:contains('ジャンル')").Next().Find("a").Each(func(i int, s *goquery.Selection) {
		if tag := strings.TrimSpace(s.Text()); tag != "" {
			tags = append(tags, tag)
		}
	})
	movieData.Tag = tags
	
	// Extract outline from og:description or other sources
	if outline, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		movieData.Outline = strings.TrimSpace(outline)
	} else if outline := doc.Find("div.mg-b20.lh4").Text(); outline != "" {
		movieData.Outline = strings.TrimSpace(outline)
	} else if outline := doc.Find("p.mg-b20").Text(); outline != "" {
		movieData.Outline = strings.TrimSpace(outline)
	} else if outline := doc.Find("div.summary").Text(); outline != "" {
		movieData.Outline = strings.TrimSpace(outline)
	}
	
	// Extract extra fanart
	var extrafanart []string
	doc.Find("div#sample-image-block img").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			if strings.HasPrefix(src, "//") {
				src = "https:" + src
			} else if strings.HasPrefix(src, "/") {
				src = "https://www.dmm.co.jp" + src
			}
			// Get large image by replacing - with jp-
			src = strings.Replace(src, "-", "jp-", 1)
			extrafanart = append(extrafanart, src)
		}
	})
	if len(extrafanart) == 0 {
		// Fallback selector
		doc.Find("div.sample-image-block img").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if strings.HasPrefix(src, "//") {
					src = "https:" + src
				} else if strings.HasPrefix(src, "/") {
					src = "https://www.dmm.co.jp" + src
				}
				// Get large image by replacing - with jp-
				src = strings.Replace(src, "-", "jp-", 1)
				extrafanart = append(extrafanart, src)
			}
		})
	}
	movieData.Extrafanart = extrafanart
	
	// Extract trailer
	if trailer, exists := doc.Find("div#sample-video a").Attr("href"); exists {
		if strings.HasPrefix(trailer, "//") {
			trailer = "https:" + trailer
		} else if strings.HasPrefix(trailer, "/") {
			trailer = "https://www.dmm.co.jp" + trailer
		}
		movieData.Trailer = trailer
	} else if trailer, exists := doc.Find("video").Attr("src"); exists {
		if strings.HasPrefix(trailer, "//") {
			trailer = "https:" + trailer
		} else if strings.HasPrefix(trailer, "/") {
			trailer = "https://www.dmm.co.jp" + trailer
		}
		movieData.Trailer = trailer
	}
	

	
	logger.Debug("Successfully scraped Fanza data for: %s", movieData.Title)
	return movieData, nil
}