package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/httpclient"
	"movie-data-capture/pkg/logger"
)

// scrapeFC2 scrapes movie data from FC2 using CloudScraper
// Note: FC2 adult content service has been partially shut down since November 2024
// due to legal issues in Japan. Connection failures are expected.
func (s *Scraper) scrapeFC2(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting FC2 scraping for number: %s", number)
	logger.Info("Warning: FC2 adult content service has been partially shut down since November 2024")
	
	// Extract numeric part from FC2 number (FC2-1234567 -> 1234567)
	fc2Number := strings.TrimPrefix(number, "FC2-")
	fc2Number = strings.TrimPrefix(fc2Number, "FC2")
	fc2Number = strings.TrimPrefix(fc2Number, "PPV-")
	fc2Number = strings.TrimPrefix(fc2Number, "PPV")
	fc2Number = strings.Trim(fc2Number, "-_")
	
	logger.Debug("Extracted FC2 number: %s", fc2Number)
	
	// Create CloudScraper client with proxy support
	var cloudClient *httpclient.CloudScraperClient
	var err error
	
	if s.config.Proxy.Switch {
		proxyConfig := &httpclient.ProxyConfig{
			Enabled: true,
			Type:    s.config.Proxy.Type,
			Address: s.config.Proxy.Proxy,
		}
		cloudClient, err = httpclient.NewCloudScraperClientWithProxy(proxyConfig)
	} else {
		cloudClient, err = httpclient.NewCloudScraperClient()
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create cloudscraper client: %w", err)
	}
	
	// FC2 uses different URL formats, try multiple patterns
	urlPatterns := []string{
		"https://adult.contents.fc2.com/article/%s/",
		"https://adult.contents.fc2.com/users/%s/",
		"https://fc2.com/adult/%s/",
	}
	
	var resp *http.Response
	var contentURL string
	
	// Try different URL patterns
	for _, pattern := range urlPatterns {
		contentURL = fmt.Sprintf(pattern, fc2Number)
		logger.Debug("Trying FC2 URL: %s", contentURL)
		
		// Set FC2-specific cookies
		parsedURL, _ := url.Parse(contentURL)
		if parsedURL != nil {
			// Add FC2-specific cookies for adult content
			adultCookie := &http.Cookie{
				Name:   "adult",
				Value:  "1",
				Domain: parsedURL.Hostname(),
				Path:   "/",
			}
			cloudClient.SetCookie(parsedURL, adultCookie)
		}
		
		resp, err = cloudClient.Get(ctx, contentURL, map[string]string{
			"Cookie": "adult=1; fc2_lang=ja",
		})
		if err != nil {
			logger.Debug("Failed to fetch %s: %v", contentURL, err)
			// Check if it's a connection error (likely due to FC2 service shutdown)
			if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "forcibly closed") {
				logger.Info("FC2 connection failed - service may be unavailable due to November 2024 shutdown")
			}
			continue
		}
		
		logger.Debug("FC2 response status: %d for URL: %s", resp.StatusCode, contentURL)
		if resp.StatusCode == 200 {
			break
		} else if resp.StatusCode == 404 {
			resp.Body.Close()
			resp = nil
			continue
		}
		resp.Body.Close()
		resp = nil
	}
	
	if resp == nil {
		return nil, fmt.Errorf("FC2 content not accessible for number: %s. Direct FC2 access failed (service partially shut down since November 2024)", fc2Number)
	}
	defer resp.Body.Close()
	
	// Debug: log first 500 characters of HTML content
     bodyBytes, _ := io.ReadAll(resp.Body)
     bodyStr := string(bodyBytes)
     if len(bodyStr) > 500 {
         logger.Debug("FC2 HTML content (first 500 chars): %s...", bodyStr[:500])
     } else {
         logger.Debug("FC2 HTML content: %s", bodyStr)
     }
     
     // Check if FC2 is redirecting to login page
	     if strings.Contains(bodyStr, "fc2.com/login.php") {
	         return nil, fmt.Errorf("FC2 requires login to access content for number: %s", fc2Number)
	     }
	     
	     // Check if FC2 shows "not found" page
	     if strings.Contains(bodyStr, "没有发现您要找的商品") || strings.Contains(bodyStr, "商品が見つかりませんでした") || strings.Contains(bodyStr, "Not Found") {
	         logger.Info("FC2 shows 'content not found' page for number: %s", fc2Number)
	         return nil, fmt.Errorf("FC2 content not found for number: %s", fc2Number)
	     }
     
     // Create new reader for goquery
     resp.Body = io.NopCloser(strings.NewReader(bodyStr))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	movieData := &MovieData{
		Website: contentURL,
		Source:  "fc2",
		Number:  number,
	}
	
	// Try different selectors for title
	logger.Debug("Searching for title with selectors...")
	titleSelectors := []string{
		"h2.items_article_headerInfo__title",
		".items_article_headerInfo__title",
		"h1",
		"title",
		".items_article_headerInfo h2",
		".items_article_headerInfo__title h2",
		"[class*='title']",
		"[class*='Title']",
		"h2[class*='title']",
		"h1[class*='title']",
	}
	
	var titleResults []string
	for _, selector := range titleSelectors {
		titleText := ""
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if titleText == "" {
				titleText = strings.TrimSpace(s.Text())
			}
		})
		titleResults = append(titleResults, fmt.Sprintf("%s='%s'", selector, titleText))
		if titleText != "" && movieData.Title == "" {
			movieData.Title = titleText
		}
	}
	
	logger.Debug("Title selectors results: %s", strings.Join(titleResults, ", "))
	logger.Debug("Final extracted title: '%s'", movieData.Title)
	
	// Extract cover image
	if cover, exists := doc.Find(".items_article_MainitemThumb img").Attr("src"); exists {
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		} else if strings.HasPrefix(cover, "/") {
			cover = "https://adult.contents.fc2.com" + cover
		}
		movieData.Cover = cover
	}
	
	// Extract actors - FC2 typically uses seller as actor
	var actors []string
	// Try multiple selectors for actors
	actorSelectors := []string{
		".items_article_Actor a",
		".items_article_headerInfo__sellerName a",
		".items_article_headerInfo__seller a",
	}
	for _, selector := range actorSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if actor := strings.TrimSpace(s.Text()); actor != "" {
				actors = append(actors, actor)
			}
		})
		if len(actors) > 0 {
			break
		}
	}
	movieData.ActorList = actors
	// Convert ActorList to comma-separated string for Actor field
	if len(actors) > 0 {
		movieData.Actor = strings.Join(actors, ",")
	}
	
	// Extract studio (same as seller for FC2)
	if studio := doc.Find(".items_article_headerInfo__sellerName a").Text(); studio != "" {
		movieData.Studio = strings.TrimSpace(studio)
	}
	
	// Extract runtime from video info
	doc.Find(".items_article_headerInfo__info li").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "再生時間") || strings.Contains(text, "時間") {
			// Extract time in format like "12:34" or "12分34秒"
			re := regexp.MustCompile(`(\d+):(\d+)|（(\d+)分(\d+)秒）`)
			if matches := re.FindStringSubmatch(text); len(matches) > 0 {
				if matches[1] != "" && matches[2] != "" {
					// Format: MM:SS
					movieData.Runtime = matches[1]
				} else if matches[3] != "" {
					// Format: MM分SS秒
					movieData.Runtime = matches[3]
				}
			}
		}
	})
	
	// Extract release date
	doc.Find(".items_article_headerInfo__info li").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "販売日") {
			// Extract date in various formats
			re := regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})`)
			if matches := re.FindStringSubmatch(text); len(matches) > 0 {
				movieData.Release = fmt.Sprintf("%s-%02s-%02s", matches[1], matches[2], matches[3])
				movieData.Year = matches[1]
			}
		}
	})
	
	// Extract tags from article tags
	var tags []string
	doc.Find(".items_article_Maintag a").Each(func(i int, s *goquery.Selection) {
		if tag := strings.TrimSpace(s.Text()); tag != "" {
			tags = append(tags, tag)
		}
	})
	movieData.Tag = tags
	
	// Extract outline/description
	if outline := doc.Find(".items_article_headerInfo__comment").Text(); outline != "" {
		movieData.Outline = strings.TrimSpace(outline)
	} else if outline := doc.Find(".items_article_MainitemComment").Text(); outline != "" {
		movieData.Outline = strings.TrimSpace(outline)
	}
	
	// Extract extra fanart from sample images
	var extraFanart []string
	doc.Find(".items_article_SampleImagesArea img").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			if strings.HasPrefix(src, "//") {
				src = "https:" + src
			} else if strings.HasPrefix(src, "/") {
				src = "https://adult.contents.fc2.com" + src
			}
			extraFanart = append(extraFanart, src)
		}
	})
	movieData.Extrafanart = extraFanart
	
	logger.Debug("Successfully scraped FC2 data for: %s", movieData.Number)
	return movieData, nil
}