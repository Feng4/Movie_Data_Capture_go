package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
	"golang.org/x/net/publicsuffix"
)

// CloudScraperClient mimics Python's cloudscraper functionality
type CloudScraperClient struct {
	client *http.Client
	userAgents []string
	currentUA  string
}

// ProxyConfig represents proxy configuration for CloudScraper
type ProxyConfig struct {
	Enabled bool
	Type    string
	Address string
}

// NewCloudScraperClient creates a new client with anti-bot capabilities
func NewCloudScraperClient() (*CloudScraperClient, error) {
	return NewCloudScraperClientWithProxy(nil)
}

// NewCloudScraperClientWithProxy creates a new client with proxy support
func NewCloudScraperClientWithProxy(proxyConfig *ProxyConfig) (*CloudScraperClient, error) {
	// Create cookie jar for session management
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Configure TLS to mimic real browsers
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	// Create transport with optimized settings
	transport := &http.Transport{
		TLSClientConfig:       tlsConfig,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false, // Let Go handle compression automatically
	}

	// Configure proxy if provided
	if proxyConfig != nil && proxyConfig.Enabled {
		switch strings.ToLower(proxyConfig.Type) {
		case "socks5", "socks5h":
			if dialer, err := proxy.SOCKS5("tcp", proxyConfig.Address, nil, &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}); err == nil {
				transport.DialContext = dialer.(proxy.ContextDialer).DialContext
			}
		case "http", "https":
			proxyURL, err := url.Parse(fmt.Sprintf("%s://%s", proxyConfig.Type, proxyConfig.Address))
			if err == nil {
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		}
	}

	// Create HTTP client
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects and copy important headers
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			// Copy important headers to redirect request
			if len(via) > 0 {
				lastReq := via[len(via)-1]
				req.Header.Set("User-Agent", lastReq.Header.Get("User-Agent"))
				req.Header.Set("Accept", lastReq.Header.Get("Accept"))
				req.Header.Set("Accept-Language", lastReq.Header.Get("Accept-Language"))
			}
			return nil
		},
	}

	// Define realistic user agents (similar to cloudscraper)
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	return &CloudScraperClient{
		client:     client,
		userAgents: userAgents,
		currentUA:  userAgents[0],
	}, nil
}

// Get performs a GET request with anti-bot measures
func (c *CloudScraperClient) Get(ctx context.Context, targetURL string, headers map[string]string) (*http.Response, error) {
	// Add random delay to mimic human behavior
	delay := time.Duration(rand.Intn(2000)+500) * time.Millisecond
	time.Sleep(delay)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set realistic browser headers (similar to cloudscraper)
	c.setRealisticHeaders(req, targetURL)

	// Apply custom headers (override defaults if provided)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Perform request with retry logic
	return c.performRequestWithRetry(req, 3)
}

// setRealisticHeaders sets headers that mimic a real browser
func (c *CloudScraperClient) setRealisticHeaders(req *http.Request, targetURL string) {
	// Rotate user agent occasionally
	if rand.Float32() < 0.1 { // 10% chance to rotate
		c.currentUA = c.userAgents[rand.Intn(len(c.userAgents))]
	}

	req.Header.Set("User-Agent", c.currentUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,ja;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Set sec-ch-ua headers based on current user agent
	if strings.Contains(c.currentUA, "Chrome/120") {
		req.Header.Set("sec-ch-ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	} else if strings.Contains(c.currentUA, "Chrome/119") {
		req.Header.Set("sec-ch-ua", `"Google Chrome";v="119", "Chromium";v="119", "Not?A_Brand";v="24"`)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	}

	// Set site-specific headers
	parsedURL, _ := url.Parse(targetURL)
	if parsedURL != nil {
		switch {
		case strings.Contains(parsedURL.Host, "fc2.com"):
			req.Header.Set("Referer", "https://adult.contents.fc2.com/")
		case strings.Contains(parsedURL.Host, "javdb"):
			req.Header.Set("Cookie", "over18=1; locale=zh")
		case strings.Contains(parsedURL.Host, "javbus"):
			req.Header.Set("Cookie", "existmag=all")
		case strings.Contains(parsedURL.Host, "mgstage"):
			req.Header.Set("Cookie", "adc=1")
		}
	}
}

// performRequestWithRetry performs the request with exponential backoff retry
func (c *CloudScraperClient) performRequestWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			time.Sleep(backoff + jitter)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check for Cloudflare challenge or bot detection
		if c.isCloudflareChallenge(resp) {
			resp.Body.Close()
			lastErr = fmt.Errorf("cloudflare challenge detected")
			continue
		}

		// Success
		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, lastErr)
}

// isCloudflareChallenge detects if the response is a Cloudflare challenge
func (c *CloudScraperClient) isCloudflareChallenge(resp *http.Response) bool {
	// Check status codes that indicate challenges
	if resp.StatusCode == 403 || resp.StatusCode == 503 {
		// Read a small portion of the body to check for Cloudflare indicators
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			return false
		}
		bodyStr := strings.ToLower(string(body))

		// Common Cloudflare challenge indicators
		cloudflareIndicators := []string{
			"cloudflare",
			"checking your browser",
			"please wait while we are checking your browser",
			"ddos protection by cloudflare",
			"ray id",
			"cf-ray",
		}

		for _, indicator := range cloudflareIndicators {
			if strings.Contains(bodyStr, indicator) {
				return true
			}
		}
	}

	return false
}

// SetCookie adds a cookie to the client's jar
func (c *CloudScraperClient) SetCookie(u *url.URL, cookie *http.Cookie) {
	c.client.Jar.SetCookies(u, []*http.Cookie{cookie})
}

// GetCookies returns cookies for a given URL
func (c *CloudScraperClient) GetCookies(u *url.URL) []*http.Cookie {
	return c.client.Jar.Cookies(u)
}