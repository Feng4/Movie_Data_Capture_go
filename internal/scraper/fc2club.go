package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeFC2Club scrapes movie data from FC2Club
func (s *Scraper) scrapeFC2Club(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting FC2Club scraping for number: %s", number)

	// Search for the movie
	searchURL := fmt.Sprintf("https://fc2club.top/search?q=%s", url.QueryEscape(number))
	logger.Debug("FC2Club search URL: %s", searchURL)

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
	doc.Find("a[href*='/html/']").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if exists {
			// Check if this result matches our number
			title := sel.Text()
			if strings.Contains(strings.ToUpper(title), strings.ToUpper(number)) {
				if strings.HasPrefix(href, "/") {
					detailURL = "https://fc2club.top" + href
				} else {
					detailURL = href
				}
				return
			}
		}
	})

	if detailURL == "" {
		// Try direct URL construction
		detailURL = fmt.Sprintf("https://fc2club.top/html/%s.html", number)
	}

	return s.scrapeFC2ClubDetail(ctx, detailURL, number)
}

// scrapeFC2ClubDetail scrapes detailed movie data from FC2Club detail page
func (s *Scraper) scrapeFC2ClubDetail(ctx context.Context, detailURL, number string) (*MovieData, error) {
	logger.Debug("FC2Club detail URL: %s", detailURL)

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
		Source:  "fc2club",
		Website: detailURL,
	}

	// Extract number from h1
	numberText := doc.Find("h1").Text()
	if numberText != "" {
		movieData.Number = strings.TrimSpace(numberText)
	}

	// Extract title from h3
	title := doc.Find("h3").Text()
	if title != "" {
		// Remove FC2-{number} prefix if present
		title = strings.ReplaceAll(title, fmt.Sprintf("FC2-%s ", number), "")
		movieData.Title = strings.TrimSpace(title)
		movieData.OriginalTitle = movieData.Title
	}

	// Extract cover and extrafanart
	var extrafanart []string
	doc.Find("img.responsive").Each(func(i int, sel *goquery.Selection) {
		src, exists := sel.Attr("src")
		if exists {
			// Convert relative URLs to absolute
			if strings.HasPrefix(src, "../uploadfile") {
				src = strings.ReplaceAll(src, "../uploadfile", "https://fc2club.top/uploadfile")
			}
			extrafanart = append(extrafanart, src)
			
			// Use first image as cover
			if i == 0 {
				movieData.Cover = src
			}
		}
	})
	movieData.Extrafanart = extrafanart

	// Extract studio (seller info)
	doc.Find("strong").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "卖家信息") {
			studioText := sel.Parent().Find("a").Text()
			if studioText != "" {
				studio := strings.ReplaceAll(studioText, "本资源官网地址", "")
				movieData.Studio = strings.TrimSpace(studio)
			}
		}
	})

	// Extract score
	doc.Find("strong").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "影片评分") {
			scoreText := sel.Parent().Text()
			re := regexp.MustCompile(`\d+`)
			matches := re.FindString(scoreText)
			if matches != "" {
				if score, err := strconv.ParseFloat(matches, 64); err == nil {
					movieData.UserRating = score
				}
			}
		}
	})

	// Extract actors
	var actors []string
	doc.Find("strong").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "女优名字") {
			sel.Parent().Find("a").Each(func(j int, actorSel *goquery.Selection) {
				actorName := strings.TrimSpace(actorSel.Text())
				if actorName != "" {
					actors = append(actors, actorName)
				}
			})
		}
	})

	// If no actors found, use studio as actor (based on fc2_seller rule)
	if len(actors) == 0 && movieData.Studio != "" {
		actors = append(actors, movieData.Studio)
	}

	movieData.ActorList = actors
	movieData.Actor = strings.Join(actors, ",")

	// Create actor photo map (empty for now)
	actorPhoto := make(map[string]string)
	for _, actor := range actors {
		actorPhoto[actor] = ""
	}
	movieData.ActorPhoto = actorPhoto

	// Extract tags
	var tags []string
	doc.Find("strong").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "影片标签") {
			sel.Parent().Find("a").Each(func(j int, tagSel *goquery.Selection) {
				tagName := strings.TrimSpace(tagSel.Text())
				if tagName != "" {
					tags = append(tags, tagName)
				}
			})
		}
	})
	movieData.Tag = tags

	// Extract outline
	outline := doc.Find(".col.des").Text()
	if outline != "" {
		// Clean up the outline text
		outline = strings.ReplaceAll(outline, "\n", "")
		outline = strings.ReplaceAll(outline, "・", "")
		movieData.Outline = strings.TrimSpace(outline)
	}

	// Extract mosaic info
	doc.Find("strong").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "马赛克") {
			mosaicText := sel.Parent().Text()
			if strings.Contains(mosaicText, "无码") {
				movieData.Uncensored = true
			}
		}
	})

	// Validate required fields
	if movieData.Title == "" {
		return nil, fmt.Errorf("no valid title found")
	}

	return movieData, nil
}