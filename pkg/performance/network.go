package performance

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPClientPool 管理具有不同配置的HTTP客户端池
type HTTPClientPool struct {
	mu      sync.RWMutex
	clients map[string]*PooledClient
	config  *HTTPClientPoolConfig
	stats   *HTTPClientPoolStats
	running int32
	stopCh  chan struct{}
}

// HTTPClientPoolConfig HTTP客户端池的配置
type HTTPClientPoolConfig struct {
	MaxIdleConns        int           `json:"max_idle_conns"`
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `json:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `json:"idle_conn_timeout"`
	DialTimeout         time.Duration `json:"dial_timeout"`
	KeepAlive           time.Duration `json:"keep_alive"`
	TLSHandshakeTimeout time.Duration `json:"tls_handshake_timeout"`
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout"`
	ExpectContinueTimeout time.Duration `json:"expect_continue_timeout"`
	EnableCompression   bool          `json:"enable_compression"`
	EnableHTTP2         bool          `json:"enable_http2"`
	InsecureSkipVerify  bool          `json:"insecure_skip_verify"`
	UserAgent           string        `json:"user_agent"`
	MaxRetries          int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`
	EnableMetrics       bool          `json:"enable_metrics"`
	CleanupInterval     time.Duration `json:"cleanup_interval"`
}

// PooledClient 表示池化的HTTP客户端
type PooledClient struct {
	client      *http.Client
	config      *ClientConfig
	lastUsed    time.Time
	requestCount uint64
	errorCount   uint64
	created     time.Time
}

// ClientConfig 单个客户端的配置
type ClientConfig struct {
	Timeout         time.Duration `json:"timeout"`
	MaxRedirects    int           `json:"max_redirects"`
	FollowRedirects bool          `json:"follow_redirects"`
	CookieJar       bool          `json:"cookie_jar"`
	Proxy           string        `json:"proxy"`
	Headers         map[string]string `json:"headers"`
}

// HTTPClientPoolStats 保存HTTP客户端池统计信息
type HTTPClientPoolStats struct {
	mu              sync.RWMutex
	TotalRequests   uint64        `json:"total_requests"`
	SuccessRequests uint64        `json:"success_requests"`
	FailedRequests  uint64        `json:"failed_requests"`
	TotalLatency    time.Duration `json:"total_latency"`
	AverageLatency  time.Duration `json:"average_latency"`
	ActiveClients   int           `json:"active_clients"`
	ConnectionsCreated uint64     `json:"connections_created"`
	ConnectionsReused  uint64     `json:"connections_reused"`
	LastUpdated     time.Time     `json:"last_updated"`
}

// RequestCache 实现HTTP请求缓存
type RequestCache struct {
	mu      sync.RWMutex
	cache   map[string]*CachedResponse
	config  *RequestCacheConfig
	stats   *RequestCacheStats
	running int32
	stopCh  chan struct{}
}

// RequestCacheConfig 请求缓存的配置
type RequestCacheConfig struct {
	MaxSize         int           `json:"max_size"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	MaxResponseSize int64         `json:"max_response_size"`
	CacheableStatus []int         `json:"cacheable_status"`
	CacheableMethods []string     `json:"cacheable_methods"`
	IgnoreHeaders   []string      `json:"ignore_headers"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	EnableMetrics   bool          `json:"enable_metrics"`
	Compress        bool          `json:"compress"`
}

// CachedResponse 表示缓存的HTTP响应
type CachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	Expiry     time.Time         `json:"expiry"`
	Created    time.Time         `json:"created"`
	HitCount   uint64            `json:"hit_count"`
	Size       int64             `json:"size"`
}

// RequestCacheStats 保存请求缓存统计信息
type RequestCacheStats struct {
	mu          sync.RWMutex
	Hits        uint64    `json:"hits"`
	Misses      uint64    `json:"misses"`
	Stores      uint64    `json:"stores"`
	Evictions   uint64    `json:"evictions"`
	Size        int       `json:"size"`
	MemoryUsage int64     `json:"memory_usage"`
	HitRatio    float64   `json:"hit_ratio"`
	LastUpdated time.Time `json:"last_updated"`
}

// NetworkMonitor 监控网络性能
type NetworkMonitor struct {
	mu       sync.RWMutex
	metrics  *NetworkMetrics
	config   *NetworkMonitorConfig
	running  int32
	stopCh   chan struct{}
	callbacks []NetworkCallback
}

// NetworkMonitorConfig 网络监控器的配置
type NetworkMonitorConfig struct {
	UpdateInterval  time.Duration `json:"update_interval"`
	LatencyHistory  int           `json:"latency_history"`
	ThroughputWindow time.Duration `json:"throughput_window"`
	EnableMetrics   bool          `json:"enable_metrics"`
	TestEndpoints   []string      `json:"test_endpoints"`
	TestInterval    time.Duration `json:"test_interval"`
}

// NetworkMetrics 保存网络性能指标
type NetworkMetrics struct {
	mu                sync.RWMutex
	Latency           time.Duration   `json:"latency"`
	LatencyHistory    []time.Duration `json:"latency_history"`
	Throughput        float64         `json:"throughput"`
	PacketLoss        float64         `json:"packet_loss"`
	ConnectionErrors  uint64          `json:"connection_errors"`
	TimeoutErrors     uint64          `json:"timeout_errors"`
	DNSResolutionTime time.Duration   `json:"dns_resolution_time"`
	TCPConnectTime    time.Duration   `json:"tcp_connect_time"`
	TLSHandshakeTime  time.Duration   `json:"tls_handshake_time"`
	FirstByteTime     time.Duration   `json:"first_byte_time"`
	LastUpdated       time.Time       `json:"last_updated"`
}

// NetworkCallback 网络回调的函数类型
type NetworkCallback func(*NetworkMetrics)

// ConnectionPool 管理网络连接
type ConnectionPool struct {
	mu          sync.RWMutex
	connections map[string][]*PooledConnection
	config      *ConnectionPoolConfig
	stats       *ConnectionPoolStats
	running     int32
	stopCh      chan struct{}
}

// ConnectionPoolConfig 连接池的配置
type ConnectionPoolConfig struct {
	MaxConnections     int           `json:"max_connections"`
	MaxIdleTime        time.Duration `json:"max_idle_time"`
	ConnectionTimeout  time.Duration `json:"connection_timeout"`
	KeepAlive          time.Duration `json:"keep_alive"`
	CleanupInterval    time.Duration `json:"cleanup_interval"`
	EnableMetrics      bool          `json:"enable_metrics"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// PooledConnection 表示池化的网络连接
type PooledConnection struct {
	conn       net.Conn
	addr       string
	created    time.Time
	lastUsed   time.Time
	useCount   uint64
	healthy    bool
	inUse      int32
}

// ConnectionPoolStats 保存连接池统计信息
type ConnectionPoolStats struct {
	mu              sync.RWMutex
	ActiveConnections int     `json:"active_connections"`
	IdleConnections   int     `json:"idle_connections"`
	TotalConnections  uint64  `json:"total_connections"`
	ConnectionsCreated uint64 `json:"connections_created"`
	ConnectionsReused  uint64 `json:"connections_reused"`
	ConnectionsClosed  uint64 `json:"connections_closed"`
	ConnectionErrors   uint64 `json:"connection_errors"`
	LastUpdated       time.Time `json:"last_updated"`
}

// 默认配置

// DefaultHTTPClientPoolConfig 返回默认的HTTP客户端池配置
func DefaultHTTPClientPoolConfig() *HTTPClientPoolConfig {
	return &HTTPClientPoolConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       50,
		IdleConnTimeout:       90 * time.Second,
		DialTimeout:           30 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		EnableCompression:     true,
		EnableHTTP2:           true,
		InsecureSkipVerify:    false,
		UserAgent:             "Movie-Data-Capture/1.0",
		MaxRetries:            3,
		RetryDelay:            1 * time.Second,
		EnableMetrics:         true,
		CleanupInterval:       5 * time.Minute,
	}
}

// NewHTTPClientPool 创建新的HTTP客户端池
func NewHTTPClientPool(config *HTTPClientPoolConfig) *HTTPClientPool {
	if config == nil {
		config = DefaultHTTPClientPoolConfig()
	}

	return &HTTPClientPool{
		clients: make(map[string]*PooledClient),
		config:  config,
		stats:   &HTTPClientPoolStats{LastUpdated: time.Now()},
		stopCh:  make(chan struct{}),
	}
}

// Start 启动HTTP客户端池
func (hcp *HTTPClientPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&hcp.running, 0, 1) {
		return fmt.Errorf("HTTP client pool is already running")
	}

	if hcp.config.CleanupInterval > 0 {
		go hcp.cleanupLoop(ctx)
	}

	return nil
}

// Stop 停止HTTP客户端池
func (hcp *HTTPClientPool) Stop() {
	if atomic.CompareAndSwapInt32(&hcp.running, 1, 0) {
		close(hcp.stopCh)
	}
}

// GetClient 获取或创建具有指定配置的HTTP客户端
func (hcp *HTTPClientPool) GetClient(key string, clientConfig *ClientConfig) *http.Client {
	hcp.mu.RLock()
	pooledClient, exists := hcp.clients[key]
	hcp.mu.RUnlock()

	if exists {
		pooledClient.lastUsed = time.Now()
		atomic.AddUint64(&pooledClient.requestCount, 1)
		return pooledClient.client
	}

	// 创建新客户端
	hcp.mu.Lock()
	defer hcp.mu.Unlock()

	// 获取写锁后再次检查
	if pooledClient, exists := hcp.clients[key]; exists {
		pooledClient.lastUsed = time.Now()
		atomic.AddUint64(&pooledClient.requestCount, 1)
		return pooledClient.client
	}

	client := hcp.createClient(clientConfig)
	pooledClient = &PooledClient{
		client:   client,
		config:   clientConfig,
		lastUsed: time.Now(),
		created:  time.Now(),
	}

	hcp.clients[key] = pooledClient
	atomic.AddUint64(&hcp.stats.ConnectionsCreated, 1)

	return client
}

// createClient 创建具有指定配置的新HTTP客户端
func (hcp *HTTPClientPool) createClient(config *ClientConfig) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          hcp.config.MaxIdleConns,
		MaxIdleConnsPerHost:   hcp.config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       hcp.config.MaxConnsPerHost,
		IdleConnTimeout:       hcp.config.IdleConnTimeout,
		TLSHandshakeTimeout:   hcp.config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: hcp.config.ResponseHeaderTimeout,
		ExpectContinueTimeout: hcp.config.ExpectContinueTimeout,
		DisableCompression:    !hcp.config.EnableCompression,
		ForceAttemptHTTP2:     hcp.config.EnableHTTP2,
		DialContext: (&net.Dialer{
			Timeout:   hcp.config.DialTimeout,
			KeepAlive: hcp.config.KeepAlive,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: hcp.config.InsecureSkipVerify,
		},
	}

	// 如果指定了代理则配置代理
	if config != nil && config.Proxy != "" {
		if proxyURL, err := url.Parse(config.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	client := &http.Client{
		Transport: transport,
	}

	// 配置超时
	if config != nil && config.Timeout > 0 {
		client.Timeout = config.Timeout
	}

	// 配置重定向
	if config != nil && !config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if config != nil && config.MaxRedirects > 0 {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", config.MaxRedirects)
			}
			return nil
		}
	}

	// 配置cookie jar
	if config != nil && config.CookieJar {
		// 注意：在实际实现中，您需要创建一个合适的cookie jar
		// client.Jar = cookiejar.New(nil)
	}

	return client
}

// DoRequest 执行HTTP请求并跟踪指标
func (hcp *HTTPClientPool) DoRequest(ctx context.Context, req *http.Request, clientKey string, clientConfig *ClientConfig) (*http.Response, error) {
	start := time.Now()
	client := hcp.GetClient(clientKey, clientConfig)

	// 添加默认头部
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", hcp.config.UserAgent)
	}

	// 从配置中添加自定义头部
	if clientConfig != nil && clientConfig.Headers != nil {
		for key, value := range clientConfig.Headers {
			req.Header.Set(key, value)
		}
	}

	var resp *http.Response
	var err error

	// 重试逻辑
	for attempt := 0; attempt <= hcp.config.MaxRetries; attempt++ {
		resp, err = client.Do(req.WithContext(ctx))
		if err == nil {
			break
		}

		if attempt < hcp.config.MaxRetries {
			time.Sleep(hcp.config.RetryDelay)
		}
	}

	// 更新指标
	latency := time.Since(start)
	hcp.updateRequestMetrics(latency, err == nil)

	return resp, err
}

// updateRequestMetrics 更新请求指标
func (hcp *HTTPClientPool) updateRequestMetrics(latency time.Duration, success bool) {
	if !hcp.config.EnableMetrics {
		return
	}

	hcp.stats.mu.Lock()
	defer hcp.stats.mu.Unlock()

	hcp.stats.TotalRequests++
	if success {
		hcp.stats.SuccessRequests++
	} else {
		hcp.stats.FailedRequests++
	}

	hcp.stats.TotalLatency += latency
	if hcp.stats.TotalRequests > 0 {
		hcp.stats.AverageLatency = hcp.stats.TotalLatency / time.Duration(hcp.stats.TotalRequests)
	}

	hcp.stats.LastUpdated = time.Now()
}

// cleanupLoop 定期清理未使用的客户端
func (hcp *HTTPClientPool) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(hcp.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hcp.stopCh:
			return
		case <-ticker.C:
			hcp.performCleanup()
		}
	}
}

// performCleanup 移除未使用的客户端
func (hcp *HTTPClientPool) performCleanup() {
	hcp.mu.Lock()
	defer hcp.mu.Unlock()

	now := time.Now()
	keysToDelete := make([]string, 0)

	for key, client := range hcp.clients {
		// 移除一段时间未使用的客户端
		if now.Sub(client.lastUsed) > hcp.config.IdleConnTimeout {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(hcp.clients, key)
	}

	hcp.stats.ActiveClients = len(hcp.clients)
}

// GetStats 返回HTTP客户端池统计信息
func (hcp *HTTPClientPool) GetStats() *HTTPClientPoolStats {
	hcp.stats.mu.RLock()
	defer hcp.stats.mu.RUnlock()

	stats := *hcp.stats
	stats.ActiveClients = len(hcp.clients)
	return &stats
}

// 请求缓存实现

// DefaultRequestCacheConfig 返回默认的请求缓存配置
func DefaultRequestCacheConfig() *RequestCacheConfig {
	return &RequestCacheConfig{
		MaxSize:         1000,
		DefaultTTL:      1 * time.Hour,
		MaxResponseSize: 10 * 1024 * 1024, // 10MB
		CacheableStatus: []int{200, 301, 302, 304, 404, 410},
		CacheableMethods: []string{"GET", "HEAD"},
		IgnoreHeaders:   []string{"Authorization", "Cookie", "Set-Cookie"},
		CleanupInterval: 10 * time.Minute,
		EnableMetrics:   true,
		Compress:        true,
	}
}

// NewRequestCache 创建新的请求缓存
func NewRequestCache(config *RequestCacheConfig) *RequestCache {
	if config == nil {
		config = DefaultRequestCacheConfig()
	}

	return &RequestCache{
		cache:  make(map[string]*CachedResponse),
		config: config,
		stats:  &RequestCacheStats{LastUpdated: time.Now()},
		stopCh: make(chan struct{}),
	}
}

// Start 启动请求缓存
func (rc *RequestCache) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&rc.running, 0, 1) {
		return fmt.Errorf("request cache is already running")
	}

	if rc.config.CleanupInterval > 0 {
		go rc.cleanupLoop(ctx)
	}

	return nil
}

// Stop 停止请求缓存
func (rc *RequestCache) Stop() {
	if atomic.CompareAndSwapInt32(&rc.running, 1, 0) {
		close(rc.stopCh)
	}
}

// Get 获取缓存的响应
func (rc *RequestCache) Get(key string) (*CachedResponse, bool) {
	rc.mu.RLock()
	response, exists := rc.cache[key]
	rc.mu.RUnlock()

	if !exists {
		atomic.AddUint64(&rc.stats.Misses, 1)
		return nil, false
	}

	// 检查过期时间
	if time.Now().After(response.Expiry) {
		rc.Delete(key)
		atomic.AddUint64(&rc.stats.Misses, 1)
		return nil, false
	}

	atomic.AddUint64(&response.HitCount, 1)
	atomic.AddUint64(&rc.stats.Hits, 1)
	rc.updateHitRatio()

	return response, true
}

// Set 在缓存中存储响应
func (rc *RequestCache) Set(key string, response *CachedResponse) {
	if response.Size > rc.config.MaxResponseSize {
		return // 响应太大
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// 检查是否需要驱逐项目
	for len(rc.cache) >= rc.config.MaxSize {
		rc.evictLRU()
	}

	rc.cache[key] = response
	atomic.AddUint64(&rc.stats.Stores, 1)
	rc.stats.Size = len(rc.cache)
	rc.stats.MemoryUsage += response.Size
}

// Delete 删除缓存的响应
func (rc *RequestCache) Delete(key string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if response, exists := rc.cache[key]; exists {
		delete(rc.cache, key)
		rc.stats.Size = len(rc.cache)
		rc.stats.MemoryUsage -= response.Size
	}
}

// IsCacheable 检查请求/响应是否可缓存
func (rc *RequestCache) IsCacheable(method string, statusCode int) bool {
	// 检查方法
	methodCacheable := false
	for _, m := range rc.config.CacheableMethods {
		if m == method {
			methodCacheable = true
			break
		}
	}
	if !methodCacheable {
		return false
	}

	// 检查状态码
	for _, status := range rc.config.CacheableStatus {
		if status == statusCode {
			return true
		}
	}

	return false
}

// GenerateKey 为请求生成缓存键
func (rc *RequestCache) GenerateKey(req *http.Request) string {
	// 基于方法和URL的简单键生成
	// 在实际实现中，您可能希望包含相关头部
	return fmt.Sprintf("%s:%s", req.Method, req.URL.String())
}

// evictLRU 驱逐最近最少使用的项目
func (rc *RequestCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, response := range rc.cache {
		if oldestKey == "" || response.Created.Before(oldestTime) {
			oldestKey = key
			oldestTime = response.Created
		}
	}

	if oldestKey != "" {
		if response, exists := rc.cache[oldestKey]; exists {
			delete(rc.cache, oldestKey)
			rc.stats.MemoryUsage -= response.Size
			atomic.AddUint64(&rc.stats.Evictions, 1)
		}
	}
}

// updateHitRatio 更新缓存命中率
func (rc *RequestCache) updateHitRatio() {
	rc.stats.mu.Lock()
	defer rc.stats.mu.Unlock()

	total := rc.stats.Hits + rc.stats.Misses
	if total > 0 {
		rc.stats.HitRatio = float64(rc.stats.Hits) / float64(total) * 100
	}
}

// cleanupLoop 执行定期清理
func (rc *RequestCache) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(rc.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rc.stopCh:
			return
		case <-ticker.C:
			rc.performCleanup()
		}
	}
}

// performCleanup 移除过期项目
func (rc *RequestCache) performCleanup() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, response := range rc.cache {
		if now.After(response.Expiry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		if response, exists := rc.cache[key]; exists {
			delete(rc.cache, key)
			rc.stats.MemoryUsage -= response.Size
			atomic.AddUint64(&rc.stats.Evictions, 1)
		}
	}

	rc.stats.Size = len(rc.cache)
}

// GetStats 返回请求缓存统计信息
func (rc *RequestCache) GetStats() *RequestCacheStats {
	rc.stats.mu.RLock()
	defer rc.stats.mu.RUnlock()

	stats := *rc.stats
	return &stats
}

// 网络监控器实现

// DefaultNetworkMonitorConfig 返回默认的网络监控器配置
func DefaultNetworkMonitorConfig() *NetworkMonitorConfig {
	return &NetworkMonitorConfig{
		UpdateInterval:   10 * time.Second,
		LatencyHistory:   100,
		ThroughputWindow: 1 * time.Minute,
		EnableMetrics:    true,
		TestEndpoints:    []string{"8.8.8.8:53", "1.1.1.1:53"},
		TestInterval:     30 * time.Second,
	}
}

// NewNetworkMonitor 创建新的网络监控器
func NewNetworkMonitor(config *NetworkMonitorConfig) *NetworkMonitor {
	if config == nil {
		config = DefaultNetworkMonitorConfig()
	}

	return &NetworkMonitor{
		metrics: &NetworkMetrics{
			LatencyHistory: make([]time.Duration, 0, config.LatencyHistory),
			LastUpdated:    time.Now(),
		},
		config:    config,
		stopCh:    make(chan struct{}),
		callbacks: make([]NetworkCallback, 0),
	}
}

// Start 启动网络监控器
func (nm *NetworkMonitor) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&nm.running, 0, 1) {
		return fmt.Errorf("network monitor is already running")
	}

	go nm.monitorLoop(ctx)
	return nil
}

// Stop 停止网络监控器
func (nm *NetworkMonitor) Stop() {
	if atomic.CompareAndSwapInt32(&nm.running, 1, 0) {
		close(nm.stopCh)
	}
}

// AddCallback 添加网络回调
func (nm *NetworkMonitor) AddCallback(callback NetworkCallback) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.callbacks = append(nm.callbacks, callback)
}

// RecordLatency 记录延迟测量值
func (nm *NetworkMonitor) RecordLatency(latency time.Duration) {
	nm.metrics.mu.Lock()
	defer nm.metrics.mu.Unlock()

	nm.metrics.Latency = latency
	nm.metrics.LatencyHistory = append(nm.metrics.LatencyHistory, latency)

	if len(nm.metrics.LatencyHistory) > nm.config.LatencyHistory {
		nm.metrics.LatencyHistory = nm.metrics.LatencyHistory[1:]
	}

	nm.metrics.LastUpdated = time.Now()
}

// RecordError 记录网络错误
func (nm *NetworkMonitor) RecordError(errorType string) {
	nm.metrics.mu.Lock()
	defer nm.metrics.mu.Unlock()

	switch errorType {
	case "connection":
		nm.metrics.ConnectionErrors++
	case "timeout":
		nm.metrics.TimeoutErrors++
	}

	nm.metrics.LastUpdated = time.Now()
}

// monitorLoop 运行网络监控循环
func (nm *NetworkMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(nm.config.UpdateInterval)
	defer ticker.Stop()

	testTicker := time.NewTicker(nm.config.TestInterval)
	defer testTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-nm.stopCh:
			return
		case <-ticker.C:
			nm.updateMetrics()
		case <-testTicker.C:
			nm.performNetworkTests()
		}
	}
}

// updateMetrics 更新网络指标
func (nm *NetworkMonitor) updateMetrics() {
	// 更新指标并通知回调函数
	nm.notifyCallbacks()
}

// performNetworkTests 执行网络连接测试
func (nm *NetworkMonitor) performNetworkTests() {
	for _, endpoint := range nm.config.TestEndpoints {
		go nm.testEndpoint(endpoint)
	}
}

// testEndpoint 测试到特定端点的连接
func (nm *NetworkMonitor) testEndpoint(endpoint string) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, 5*time.Second)
	latency := time.Since(start)

	if err != nil {
		nm.RecordError("connection")
		return
	}

	conn.Close()
	nm.RecordLatency(latency)
}

// notifyCallbacks 通知所有注册的回调函数
func (nm *NetworkMonitor) notifyCallbacks() {
	nm.mu.RLock()
	callbacks := make([]NetworkCallback, len(nm.callbacks))
	copy(callbacks, nm.callbacks)
	nm.mu.RUnlock()

	metrics := nm.GetMetrics()
	for _, callback := range callbacks {
		go func(cb NetworkCallback) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic in network callback: %v\n", r)
				}
			}()
			cb(metrics)
		}(callback)
	}
}

// GetMetrics 返回当前网络指标
func (nm *NetworkMonitor) GetMetrics() *NetworkMetrics {
	nm.metrics.mu.RLock()
	defer nm.metrics.mu.RUnlock()

	metrics := *nm.metrics
	metrics.LatencyHistory = make([]time.Duration, len(nm.metrics.LatencyHistory))
	copy(metrics.LatencyHistory, nm.metrics.LatencyHistory)

	return &metrics
}

// 辅助函数

// ReadResponseBody 读取并返回响应体
func ReadResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// CreateCachedResponse 从HTTP响应创建缓存响应
func CreateCachedResponse(resp *http.Response, ttl time.Duration) (*CachedResponse, error) {
	body, err := ReadResponseBody(resp)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return &CachedResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
		Expiry:     time.Now().Add(ttl),
		Created:    time.Now(),
		Size:       int64(len(body)),
	}, nil
}