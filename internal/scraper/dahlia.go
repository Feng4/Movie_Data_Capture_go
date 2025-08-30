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

// scrapeDahlia scrapes movie data from Dahlia
func (s *Scraper) scrapeDahlia(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting Dahlia scraping for number: %s", number)

	// Search for the movie
	searchURL := fmt.Sprintf("https://www.dahlia-av.jp/search?q=%s", url.QueryEscape(number))
	logger.Debug("Dahlia search URL: %s", searchURL)

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
	doc.Find("a[href*='/works/']").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if exists && strings.Contains(href, "/works/") {
			// Check if this result matches our number
			title := sel.Text()
			if strings.Contains(strings.ToUpper(title), strings.ToUpper(number)) {
				if strings.HasPrefix(href, "/") {
					detailURL = "https://www.dahlia-av.jp" + href
				} else {
					detailURL = href
				}
				return
			}
		}
	})

	if detailURL == "" {
		return nil, fmt.Errorf("movie not found on Dahlia")
	}

	return s.scrapeDahliaDetail(ctx, detailURL, number)
}

// scrapeDahliaDetail scrapes detailed movie data from Dahlia detail page
func (s *Scraper) scrapeDahliaDetail(ctx context.Context, detailURL, number string) (*MovieData, error) {
	logger.Debug("Dahlia detail URL: %s", detailURL)

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
		Source:  "dahlia",
		Website: detailURL,
		Studio:  "DAHLIA", // Default studio
	}

	// Extract title
	title := doc.Find("h1").Text()
	movieData.Title = strings.TrimSpace(title)
	movieData.OriginalTitle = movieData.Title

	// Extract actors
	var actors []string
	doc.Find(".box_works01_list span").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "出演女優") {
			// Find the following p element
			actorText := sel.Parent().Find("p").Text()
			if actorText != "" {
				actorNames := strings.Split(actorText, ",")
				for _, name := range actorNames {
					name = strings.TrimSpace(name)
					if name != "" {
						actors = append(actors, name)
					}
				}
			}
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

	// Extract outline
	outline := doc.Find(".box_works01_text p").Text()
	movieData.Outline = strings.TrimSpace(outline)

	// Extract runtime
	doc.Find("span").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "収録時間") {
			runtimeText := sel.Next().Text()
			re := regexp.MustCompile(`\d+`)
			matches := re.FindString(runtimeText)
			if matches != "" {
				movieData.Runtime = matches
			}
		}
	})

	// Extract series
	doc.Find("span").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "系列") {
			seriesText := sel.Next().Text()
			movieData.Series = strings.TrimSpace(seriesText)
		}
	})

	// Extract director
	doc.Find("span").Each(func(i int, sel *goquery.Selection) {
		text := sel.Text()
		if strings.Contains(text, "导演") || strings.Contains(text, "導演") || strings.Contains(text, "監督") {
			directorText := sel.Next().Text()
			movieData.Director = strings.TrimSpace(directorText)
		}
	})

	// Extract publisher/studio
	doc.Find("span").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "メーカー") {
			studioText := sel.Next().Text()
			if studioText != "" {
				movieData.Studio = strings.TrimSpace(studioText)
			}
		}
	})

	// Extract release date
	doc.Find(".view_timer span").Each(func(i int, sel *goquery.Selection) {
		if strings.Contains(sel.Text(), "配信開始日") {
			releaseText := sel.Parent().Find("p").Text()
			if releaseText != "" {
				release := strings.ReplaceAll(releaseText, "/", "-")
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

	// Extract tags
	var tags []string
	doc.Find("a.genre").Each(func(i int, sel *goquery.Selection) {
		tagName := strings.TrimSpace(sel.Text())
		tagName = strings.ReplaceAll(tagName, "，", "")
		if tagName != "" {
			tags = append(tags, tagName)
		}
	})
	movieData.Tag = tags

	// Extract cover
	cover, exists := doc.Find(".works_img img").Attr("src")
	if exists {
		if strings.HasPrefix(cover, "/") {
			cover = "https://www.dahlia-av.jp" + cover
		}
		movieData.Cover = cover
	}

	// Extract extra fanart
	var extrafanart []string
	doc.Find(".works_sample img").Each(func(i int, sel *goquery.Selection) {
		src, exists := sel.Attr("src")
		if exists {
			if strings.HasPrefix(src, "/") {
				src = "https://www.dahlia-av.jp" + src
			}
			extrafanart = append(extrafanart, src)
		}
	})
	movieData.Extrafanart = extrafanart

	// Extract trailer
	trailer, exists := doc.Find("video source").Attr("src")
	if exists {
		if strings.HasPrefix(trailer, "/") {
			trailer = "https://www.dahlia-av.jp" + trailer
		}
		movieData.Trailer = trailer
	}

	// Validate required fields
	if movieData.Title == "" {
		return nil, fmt.Errorf("no valid title found")
	}

	return movieData, nil
}