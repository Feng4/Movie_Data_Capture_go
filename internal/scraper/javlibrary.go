package scraper
import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)// scrapeJavLibrary scrapes movie data from JavLibrary
// Note: JavLibrary has extremely strict anti-bot measures that prevent automated access.
// The website blocks connections at the TCP level and returns 403 errors for automated requests.
// This implementation provides the complete framework but cannot function due to these restrictions.
func (s *Scraper) scrapeJavLibrary(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting JavLibrary scraping for number: %s", number)
	logger.Debug("WARNING: JavLibrary has strict anti-bot protection that blocks automated access")
	
	// Convert number to uppercase as JavLibrary expects
	number = strings.ToUpper(number)
	
	// First, try to establish a session by visiting the main page
	if err := s.establishJavLibrarySession(ctx); err != nil {
		logger.Debug("Failed to establish JavLibrary session: %v", err)
	}
	
	// Try multiple approaches to access JavLibrary
	detailURL, err := s.findJavLibraryDetailURL(ctx, number)
	if err != nil {
		// JavLibrary blocks automated access at multiple levels:
		// 1. TCP connection termination
		// 2. HTTP 403 responses
		// 3. Advanced bot detection
		logger.Debug("JavLibrary access failed due to anti-bot protection")
		return nil, fmt.Errorf("JavLibrary access blocked by anti-bot protection: %w", err)
	}
	
	return s.scrapeJavLibraryPage(ctx, detailURL)
}

// establishJavLibrarySession tries to establish a session by visiting the main page
func (s *Scraper) establishJavLibrarySession(ctx context.Context) error {
	mainPageURLs := []string{
		"https://www.javlibrary.com/cn/",
		"http://www.javlibrary.com/cn/",
	}
	
	for _, mainURL := range mainPageURLs {
		logger.Debug("Visiting JavLibrary main page: %s", mainURL)
		
		headers := map[string]string{
			"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
			"Accept-Encoding":           "gzip, deflate, br",
			"Connection":                "keep-alive",
			"Upgrade-Insecure-Requests": "1",
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Cache-Control":             "max-age=0",
			"Cookie":                    "over18=1",
		}
		
		resp, err := s.httpClient.Get(ctx, mainURL, headers)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		
		logger.Debug("Main page response status: %d", resp.StatusCode)
		
		if resp.StatusCode == 200 {
			logger.Debug("Successfully established JavLibrary session")
			return nil
		}
	}
	
	return fmt.Errorf("failed to establish session with JavLibrary")
}

// findJavLibraryDetailURL attempts to find the detail page URL using multiple strategies
func (s *Scraper) findJavLibraryDetailURL(ctx context.Context, number string) (string, error) {
	// Try multiple URL formats and protocols
	searchURLs := []string{
		fmt.Sprintf("https://www.javlibrary.com/cn/vl_searchbyid.php?keyword=%s", number),
		fmt.Sprintf("http://www.javlibrary.com/cn/vl_searchbyid.php?keyword=%s", number),
		fmt.Sprintf("https://www.javlibrary.com/en/vl_searchbyid.php?keyword=%s", number),
	}
	
	for i, searchURL := range searchURLs {
		logger.Debug("Trying JavLibrary search URL (%d/%d): %s", i+1, len(searchURLs), searchURL)
		
		headers := map[string]string{
			"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
			"Accept-Encoding":           "gzip, deflate, br",
			"Connection":                "keep-alive",
			"Upgrade-Insecure-Requests": "1",
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Cache-Control":             "max-age=0",
			"Cookie":                    "over18=1",
			"Referer":                   "https://www.javlibrary.com/cn/",
		}
		
		resp, err := s.httpClient.Get(ctx, searchURL, headers)
		if err != nil {
			logger.Debug("Request failed: %v", err)
			continue
		}
		defer resp.Body.Close()
		
		logger.Debug("JavLibrary search response status: %d", resp.StatusCode)
		
		if resp.StatusCode != 200 {
			logger.Debug("Non-200 status, trying next URL")
			continue
		}
		
		doc, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			logger.Debug("Failed to parse HTML: %v", err)
			continue
		}
		
		// Check if we're directly on a detail page (redirect happened)
		if strings.Contains(resp.Request.URL.String(), "/?v=jav") {
			logger.Debug("Redirected to detail page: %s", resp.Request.URL.String())
			return resp.Request.URL.String(), nil
		}
		
		// Find the first movie link from search results
		var detailURL string
		doc.Find(".video").First().Find("a").Each(func(j int, s *goquery.Selection) {
			if href, exists := s.Attr("href"); exists {
				baseURL := "https://www.javlibrary.com/cn/"
				if strings.Contains(searchURL, "/en/") {
					baseURL = "https://www.javlibrary.com/en/"
				} else if strings.HasPrefix(searchURL, "http://") {
					baseURL = "http://www.javlibrary.com/cn/"
				}
				
				if strings.HasPrefix(href, "./") {
					detailURL = baseURL + strings.TrimPrefix(href, "./")
				} else if strings.HasPrefix(href, "/") {
					detailURL = strings.TrimSuffix(baseURL, "/cn/") + href
				} else {
					detailURL = href
				}
				logger.Debug("Found detail URL: %s", detailURL)
				return
			}
		})
		
		// Alternative search: look for exact number match in search results
		if detailURL == "" {
			doc.Find(".id").Each(func(j int, s *goquery.Selection) {
				if strings.TrimSpace(s.Text()) == number {
					if href, exists := s.Parent().Attr("href"); exists {
						baseURL := "https://www.javlibrary.com/cn"
						if strings.Contains(searchURL, "/en/") {
							baseURL = "https://www.javlibrary.com/en"
						} else if strings.HasPrefix(searchURL, "http://") {
							baseURL = "http://www.javlibrary.com/cn"
						}
						detailURL = baseURL + strings.TrimPrefix(href, ".")
						logger.Debug("Found exact match detail URL: %s", detailURL)
						return
					}
				}
			})
		}
		
		if detailURL != "" {
			return detailURL, nil
		}
		
		logger.Debug("No exact match found in search results")
	}
	
	return "", fmt.Errorf("no matching detail page found for number: %s after trying all search methods", number)
}



// scrapeJavLibraryPage scrapes data from a JavLibrary detail page
func (s *Scraper) scrapeJavLibraryPage(ctx context.Context, url string) (*MovieData, error) {
	logger.Debug("Scraping JavLibrary page: %s", url)
	
	headers := map[string]string{
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
		"Accept-Encoding":           "gzip, deflate, br",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Cache-Control":             "max-age=0",
		"Cookie":                    "over18=1",
	}
	
	resp, err := s.httpClient.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JavLibrary page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("JavLibrary returned status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	movieData := &MovieData{}
	
	// Extract title (Python: get_title)
	if title := doc.Find("div#video_title h3 a").First().Text(); title != "" {
		movieData.Title = strings.TrimSpace(title)
	}
	
	// Extract number (Python: get_number)
	if number := doc.Find("div#video_id table tr td.text").First().Text(); number != "" {
		movieData.Number = strings.TrimSpace(number)
	}
	
	// Extract actors (Python: get_actor)
	var actors []string
	doc.Find("div#video_cast table tr td.text span span.star a").Each(func(i int, s *goquery.Selection) {
		if actor := strings.TrimSpace(s.Text()); actor != "" {
			actors = append(actors, actor)
		}
	})
	movieData.ActorList = actors
	if len(actors) > 0 {
		movieData.Actor = strings.Join(actors, ",")
	}
	
	// Extract cover (Python: get_cover)
	if cover, exists := doc.Find("img#video_jacket_img").Attr("src"); exists {
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		} else if !strings.HasPrefix(cover, "http") {
			cover = "https:" + cover
		}
		movieData.Cover = cover
	}
	
	// Extract tags (Python: get_tag)
	var tags []string
	doc.Find("div#video_genres table tr td.text span a").Each(func(i int, s *goquery.Selection) {
		if tag := strings.TrimSpace(s.Text()); tag != "" {
			tags = append(tags, tag)
		}
	})
	movieData.Tag = tags
	
	// Extract release date (Python: get_release)
	if release := doc.Find("div#video_date table tr td.text").First().Text(); release != "" {
		movieData.Release = strings.TrimSpace(release)
		movieData.Year = extractYear(release)
	}
	
	// Extract studio (Python: get_studio)
	if studio := doc.Find("div#video_maker table tr td.text span a").First().Text(); studio != "" {
		movieData.Studio = strings.TrimSpace(studio)
	}
	
	// Extract publisher (Python: get_publisher)
	if publisher := doc.Find("div#video_label table tr td.text span a").First().Text(); publisher != "" {
		movieData.Label = strings.TrimSpace(publisher)
	}
	
	// Extract runtime (Python: get_runtime)
	if runtime := doc.Find("div#video_length table tr td span.text").First().Text(); runtime != "" {
		movieData.Runtime = strings.TrimSpace(runtime)
	}
	

	
	// Extract director (Python: get_director)
	if director := doc.Find("div#video_director table tr td.text span a").First().Text(); director != "" {
		movieData.Director = strings.TrimSpace(director)
	}
	
	// Set empty fields that JavLibrary doesn't provide
	movieData.Outline = ""     // JavLibrary doesn't provide outline
	movieData.Series = ""      // JavLibrary doesn't provide series
	movieData.Extrafanart = []string{} // JavLibrary doesn't provide extra fanart
	movieData.Trailer = ""     // JavLibrary doesn't provide trailer
	

	
	logger.Debug("Successfully scraped JavLibrary data - Number: %s, Title: %s", movieData.Number, movieData.Title)
	return movieData, nil
}