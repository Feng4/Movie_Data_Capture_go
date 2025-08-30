package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
	"movie-data-capture/internal/config"
)

const (
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	DefaultTimeout   = 30 * time.Second
)

// Client represents HTTP client with proxy and retry support
type Client struct {
	httpClient *http.Client
	config     *config.ProxyConfig
	userAgent  string
	retry      int
	timeout    time.Duration
}

// NewClient creates a new HTTP client with configuration
func NewClient(cfg *config.ProxyConfig) *Client {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	client := &Client{
		config:    cfg,
		userAgent: DefaultUserAgent,
		retry:     cfg.Retry,
		timeout:   timeout,
	}

	client.httpClient = client.buildHTTPClient()
	return client
}

// buildHTTPClient builds HTTP client with proxy and TLS configuration
func (c *Client) buildHTTPClient() *http.Client {
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
		DisableCompression:    false,
	}

	// Enhanced TLS configuration to mimic real browsers
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Allow self-signed certificates for scraping
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	// Override with custom CA if provided
	if c.config.CACertFile != "" {
		// TODO: Load custom CA certificate if needed
		tlsConfig.InsecureSkipVerify = false
	}

	transport.TLSClientConfig = tlsConfig

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
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			// Copy important headers from original request to redirect
			if len(via) > 0 {
				originalReq := via[0]
				headersToKeep := []string{"User-Agent", "Accept", "Accept-Language", "Accept-Encoding", "Cookie"}
				for _, header := range headersToKeep {
					if value := originalReq.Header.Get(header); value != "" {
						req.Header.Set(header, value)
					}
				}
			}
			return nil
		},
	}
}

// parseProxy parses proxy configuration
func (c *Client) parseProxy() (*url.URL, error) {
	proxyStr := c.config.Proxy
	if !strings.Contains(proxyStr, "://") {
		proxyStr = c.config.Type + "://" + proxyStr
	}
	return url.Parse(proxyStr)
}

// normalizeURL normalizes URL by adding protocol if missing
func (c *Client) normalizeURL(rawURL string) string {
	// Handle protocol-relative URLs (starting with //)
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	
	// Handle URLs without protocol
	if !strings.Contains(rawURL, "://") && !strings.HasPrefix(rawURL, "//") {
		return "https://" + rawURL
	}
	
	return rawURL
}

// Get performs HTTP GET request with retry
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	// Normalize URL - handle protocol-relative URLs
	normalizedURL := c.normalizeURL(url)
	return c.doRequestWithRetry(ctx, "GET", normalizedURL, nil, headers)
}

// Post performs HTTP POST request with retry
func (c *Client) Post(ctx context.Context, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	// Normalize URL - handle protocol-relative URLs
	normalizedURL := c.normalizeURL(url)
	return c.doRequestWithRetry(ctx, "POST", normalizedURL, body, headers)
}

// GetBytes gets response body as bytes
func (c *Client) GetBytes(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	resp, err := c.Get(ctx, url, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// GetString gets response body as string
func (c *Client) GetString(ctx context.Context, url string, headers map[string]string) (string, error) {
	data, err := c.GetBytes(ctx, url, headers)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// doRequestWithRetry performs HTTP request with retry mechanism
func (c *Client) doRequestWithRetry(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	var lastErr error
	
	maxRetries := c.retry
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, err
		}

		// Set default user agent
		req.Header.Set("User-Agent", c.userAgent)

		// Set custom headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				// Wait before retry
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, err)
		}

		return resp, nil
	}

	return nil, lastErr
}

// SetUserAgent sets custom user agent
func (c *Client) SetUserAgent(ua string) {
	if ua != "" {
		c.userAgent = ua
	}
}

// Close closes the HTTP client (cleanup if needed)
func (c *Client) Close() error {
	// Close idle connections
	c.httpClient.CloseIdleConnections()
	return nil
}