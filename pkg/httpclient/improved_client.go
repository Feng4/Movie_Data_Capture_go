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
	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/logger"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/120.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// ImprovedClient represents an improved HTTP client with session management
type ImprovedClient struct {
	httpClient *http.Client
	config     *config.ProxyConfig
	jar        *cookiejar.Jar
	userAgent  string
	retry      int
	timeout    time.Duration
}

// NewImprovedClient creates a new improved HTTP client
func NewImprovedClient(cfg *config.ProxyConfig) *ImprovedClient {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Increased default timeout
	}

	// Create cookie jar for session management
	jar, _ := cookiejar.New(nil)
	
	client := &ImprovedClient{
		config:    cfg,
		jar:       jar,
		userAgent: getRandomUserAgent(),
		retry:     cfg.Retry,
		timeout:   timeout,
	}

	client.httpClient = client.buildHTTPClient()
	return client
}

// buildHTTPClient builds HTTP client with improved configuration
func (c *ImprovedClient) buildHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false, // Enable compression
	}

	// Configure TLS to mimic Chrome behavior
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		// Add Chrome TLS cipher suites
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		// Chrome curve preferences
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	// Configure proxy if enabled
	if c.config.Switch && c.config.Proxy != "" {
		proxyURL, err := c.parseProxy()
		if err == nil {
			switch strings.ToLower(c.config.Type) {
			case "socks5", "socks5h":
				if dialer, err := proxy.SOCKS5("tcp", c.config.Proxy, nil, proxy.Direct); err == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
				}
			default:
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		}
	}

	return &http.Client{
		Timeout:   c.timeout,
		Transport: transport,
		Jar:       c.jar, // Enable cookie jar for session management
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
}

// GetWithSession performs HTTP GET request with improved session management
func (c *ImprovedClient) GetWithSession(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.doRequestWithSession(ctx, "GET", url, nil, headers)
}

// doRequestWithSession performs HTTP request with session management and retries
func (c *ImprovedClient) doRequestWithSession(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	var lastErr error
	
	maxRetries := c.retry
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			waitTime := time.Duration(attempt) * time.Second
			logger.Debug("Retrying request in %v (attempt %d/%d)", waitTime, attempt+1, maxRetries)
			
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			
			// Randomize user agent on retry
			c.userAgent = getRandomUserAgent()
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, err
		}

		// Set comprehensive headers to mimic real browser
		c.setRealisticHeaders(req, headers)

		// Add random delay to mimic human behavior
		if attempt > 0 {
			delay := time.Duration(500+rand.Intn(1000)) * time.Millisecond
			time.Sleep(delay)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			logger.Debug("Request failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
			continue
		}

		// Check for success or specific error codes that shouldn't be retried
		if resp.StatusCode == 200 || 
		   resp.StatusCode == 404 || 
		   resp.StatusCode == 403 {
			return resp, nil
		}

		// Close response body for non-successful responses that we'll retry
		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		logger.Debug("Request returned %d (attempt %d/%d)", resp.StatusCode, attempt+1, maxRetries)
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}

// setRealisticHeaders sets headers to mimic a real browser
func (c *ImprovedClient) setRealisticHeaders(req *http.Request, customHeaders map[string]string) {
	// Set user agent
	req.Header.Set("User-Agent", c.userAgent)
	
	// Set realistic browser headers
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,ja;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")
	
	// Set Chrome-specific headers
	req.Header.Set("sec-ch-ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)

	// Set custom headers (these will override defaults if same key)
	for key, value := range customHeaders {
		req.Header.Set(key, value)
	}
}

// SetCookies manually sets cookies for a domain
func (c *ImprovedClient) SetCookies(urlStr string, cookies map[string]string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	var httpCookies []*http.Cookie
	for name, value := range cookies {
		cookie := &http.Cookie{
			Name:   name,
			Value:  value,
			Path:   "/",
			Domain: u.Hostname(),
		}
		httpCookies = append(httpCookies, cookie)
	}

	c.jar.SetCookies(u, httpCookies)
	return nil
}

// GetCookies gets cookies for a domain
func (c *ImprovedClient) GetCookies(urlStr string) ([]*http.Cookie, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return c.jar.Cookies(u), nil
}

// getRandomUserAgent returns a random user agent
func getRandomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// parseProxy parses proxy configuration
func (c *ImprovedClient) parseProxy() (*url.URL, error) {
	proxyStr := c.config.Proxy
	if !strings.Contains(proxyStr, "://") {
		proxyStr = c.config.Type + "://" + proxyStr
	}
	return url.Parse(proxyStr)
}