package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// MemoryPool 管理可重用的内存缓冲区
type MemoryPool struct {
	mu       sync.RWMutex
	pools    map[int]*sync.Pool
	config   *MemoryPoolConfig
	stats    *MemoryPoolStats
	cleanup  chan struct{}
	running  int32
}

// MemoryPoolConfig 内存池配置
type MemoryPoolConfig struct {
	MinSize        int           `json:"min_size"`
	MaxSize        int           `json:"max_size"`
	SizeIncrement  int           `json:"size_increment"`
	MaxPoolSize    int           `json:"max_pool_size"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	EnableMetrics  bool          `json:"enable_metrics"`
	Preallocate    bool          `json:"preallocate"`
	PreallocateCount int         `json:"preallocate_count"`
}

// MemoryPoolStats 保存内存池统计信息
type MemoryPoolStats struct {
	mu           sync.RWMutex
	Allocations  uint64 `json:"allocations"`
	Deallocations uint64 `json:"deallocations"`
	Hits         uint64 `json:"hits"`
	Misses       uint64 `json:"misses"`
	TotalSize    uint64 `json:"total_size"`
	ActiveBuffers uint64 `json:"active_buffers"`
}

// Buffer 表示可重用的缓冲区
type Buffer struct {
	data     []byte
	size     int
	capacity int
	pool     *MemoryPool
	poolSize int
	inUse    int32
}

// Cache 实现带TTL支持的LRU缓存
type Cache struct {
	mu       sync.RWMutex
	items    map[string]*CacheItem
	lruList  *LRUList
	config   *CacheConfig
	stats    *CacheStats
	cleanup  chan struct{}
	running  int32
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxSize         int           `json:"max_size"`
	TTL             time.Duration `json:"ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	EnableMetrics   bool          `json:"enable_metrics"`
	EvictionPolicy  string        `json:"eviction_policy"` // "lru", "lfu", "ttl"
	MaxMemory       uint64        `json:"max_memory"`
}

// CacheItem 表示缓存项
type CacheItem struct {
	key        string
	value      interface{}
	size       uint64
	expiry     time.Time
	accessTime time.Time
	accessCount uint64
	next       *CacheItem
	prev       *CacheItem
}

// CacheStats 保存缓存统计信息
type CacheStats struct {
	mu          sync.RWMutex
	Hits        uint64 `json:"hits"`
	Misses      uint64 `json:"misses"`
	Evictions   uint64 `json:"evictions"`
	Size        int    `json:"size"`
	MemoryUsage uint64 `json:"memory_usage"`
	HitRatio    float64 `json:"hit_ratio"`
}

// LRUList 实现用于LRU跟踪的双向链表
type LRUList struct {
	head *CacheItem
	tail *CacheItem
	size int
}

// GCOptimizer 提供垃圾回收优化
type GCOptimizer struct {
	mu              sync.RWMutex
	config          *GCConfig
	lastGCStats     runtime.MemStats
	lastOptimization time.Time
	running         int32
	stopCh          chan struct{}
	stats           *GCStats
}

// GCConfig GC优化配置
type GCConfig struct {
	TargetPercent    int           `json:"target_percent"`
	OptimizeInterval time.Duration `json:"optimize_interval"`
	MemoryThreshold  uint64        `json:"memory_threshold"`
	ForceGCThreshold uint64        `json:"force_gc_threshold"`
	EnableMetrics    bool          `json:"enable_metrics"`
	Adaptive         bool          `json:"adaptive"`
}

// GCStats 保存垃圾回收统计信息
type GCStats struct {
	mu              sync.RWMutex
	GCCount         uint32        `json:"gc_count"`
	TotalPauseTime  time.Duration `json:"total_pause_time"`
	AveragePauseTime time.Duration `json:"average_pause_time"`
	LastGCTime      time.Time     `json:"last_gc_time"`
	MemoryFreed     uint64        `json:"memory_freed"`
	Optimizations   uint64        `json:"optimizations"`
}

// DefaultMemoryPoolConfig 返回默认内存池配置
func DefaultMemoryPoolConfig() *MemoryPoolConfig {
	return &MemoryPoolConfig{
		MinSize:          1024,        // 1KB
		MaxSize:          1024 * 1024, // 1MB
		SizeIncrement:    1024,        // 1KB increments
		MaxPoolSize:      100,
		CleanupInterval:  5 * time.Minute,
		EnableMetrics:    true,
		Preallocate:      true,
		PreallocateCount: 10,
	}
}

// NewMemoryPool 创建新的内存池
func NewMemoryPool(config *MemoryPoolConfig) *MemoryPool {
	if config == nil {
		config = DefaultMemoryPoolConfig()
	}

	mp := &MemoryPool{
		pools:   make(map[int]*sync.Pool),
		config:  config,
		stats:   &MemoryPoolStats{},
		cleanup: make(chan struct{}),
	}

	// 为不同大小初始化池
	for size := config.MinSize; size <= config.MaxSize; size += config.SizeIncrement {
		mp.initializePool(size)
	}

	return mp
}

// initializePool 为特定大小初始化池
func (mp *MemoryPool) initializePool(size int) {
	mp.pools[size] = &sync.Pool{
		New: func() interface{} {
			return &Buffer{
				data:     make([]byte, size),
				size:     0,
				capacity: size,
				pool:     mp,
				poolSize: size,
			}
		},
	}

	// 如果启用则预分配缓冲区
	if mp.config.Preallocate {
		for i := 0; i < mp.config.PreallocateCount; i++ {
			buf := mp.pools[size].Get().(*Buffer)
			mp.pools[size].Put(buf)
		}
	}
}

// Start 启动内存池
func (mp *MemoryPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&mp.running, 0, 1) {
		return fmt.Errorf("memory pool is already running")
	}

	if mp.config.CleanupInterval > 0 {
		go mp.cleanupLoop(ctx)
	}

	return nil
}

// Stop 停止内存池
func (mp *MemoryPool) Stop() {
	if atomic.CompareAndSwapInt32(&mp.running, 1, 0) {
		close(mp.cleanup)
	}
}

// Get 从池中获取缓冲区
func (mp *MemoryPool) Get(size int) *Buffer {
	// 查找合适的池大小
	poolSize := mp.findPoolSize(size)
	if poolSize == -1 {
		// 大小太大，直接分配
		atomic.AddUint64(&mp.stats.Misses, 1)
		atomic.AddUint64(&mp.stats.Allocations, 1)
		return &Buffer{
			data:     make([]byte, size),
			size:     size,
			capacity: size,
			pool:     mp,
			poolSize: -1,
		}
	}

	mp.mu.RLock()
	pool, exists := mp.pools[poolSize]
	mp.mu.RUnlock()

	if !exists {
		atomic.AddUint64(&mp.stats.Misses, 1)
		atomic.AddUint64(&mp.stats.Allocations, 1)
		return &Buffer{
			data:     make([]byte, size),
			size:     size,
			capacity: size,
			pool:     mp,
			poolSize: -1,
		}
	}

	buf := pool.Get().(*Buffer)
	buf.size = size
	atomic.StoreInt32(&buf.inUse, 1)
	atomic.AddUint64(&mp.stats.Hits, 1)
	atomic.AddUint64(&mp.stats.ActiveBuffers, 1)

	return buf
}

// Put 将缓冲区返回到池中
func (mp *MemoryPool) Put(buf *Buffer) {
	if buf == nil || buf.pool != mp {
		return
	}

	if !atomic.CompareAndSwapInt32(&buf.inUse, 1, 0) {
		return // 已经返回
	}

	atomic.AddUint64(&mp.stats.Deallocations, 1)
	atomic.AddUint64(&mp.stats.ActiveBuffers, ^uint64(0)) // 递减

	if buf.poolSize == -1 {
		// 直接分配，让GC处理
		return
	}

	// 重置缓冲区
	buf.size = 0
	for i := range buf.data {
		buf.data[i] = 0
	}

	mp.mu.RLock()
	pool, exists := mp.pools[buf.poolSize]
	mp.mu.RUnlock()

	if exists {
		pool.Put(buf)
	}
}

// findPoolSize 为请求的大小查找合适的池大小
func (mp *MemoryPool) findPoolSize(size int) int {
	if size > mp.config.MaxSize {
		return -1
	}

	// 向上舍入到最近的增量
	poolSize := ((size + mp.config.SizeIncrement - 1) / mp.config.SizeIncrement) * mp.config.SizeIncrement
	if poolSize < mp.config.MinSize {
		poolSize = mp.config.MinSize
	}

	return poolSize
}

// cleanupLoop 执行定期清理
func (mp *MemoryPool) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(mp.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mp.cleanup:
			return
		case <-ticker.C:
			mp.performCleanup()
		}
	}
}

// performCleanup 执行内存池清理
func (mp *MemoryPool) performCleanup() {
	// 强制GC清理未使用的缓冲区
	runtime.GC()
}

// GetStats 返回内存池统计信息
func (mp *MemoryPool) GetStats() *MemoryPoolStats {
	mp.stats.mu.RLock()
	defer mp.stats.mu.RUnlock()

	stats := *mp.stats
	return &stats
}

// Buffer 方法

// Data 返回缓冲区数据
func (b *Buffer) Data() []byte {
	return b.data[:b.size]
}

// Bytes 将缓冲区作为字节返回
func (b *Buffer) Bytes() []byte {
	return b.data[:b.size]
}

// Len 返回缓冲区长度
func (b *Buffer) Len() int {
	return b.size
}

// Cap 返回缓冲区容量
func (b *Buffer) Cap() int {
	return b.capacity
}

// Reset 重置缓冲区
func (b *Buffer) Reset() {
	b.size = 0
}

// Write 向缓冲区写入数据
func (b *Buffer) Write(data []byte) (int, error) {
	if b.size+len(data) > b.capacity {
		return 0, fmt.Errorf("buffer overflow")
	}

	copy(b.data[b.size:], data)
	b.size += len(data)
	return len(data), nil
}

// Read 从缓冲区读取数据
func (b *Buffer) Read(data []byte) (int, error) {
	n := copy(data, b.data[:b.size])
	return n, nil
}

// Release 将缓冲区释放回池中
func (b *Buffer) Release() {
	if b.pool != nil {
		b.pool.Put(b)
	}
}

// Cache 实现

// DefaultCacheConfig 返回默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:         1000,
		TTL:             1 * time.Hour,
		CleanupInterval: 10 * time.Minute,
		EnableMetrics:   true,
		EvictionPolicy:  "lru",
		MaxMemory:       100 * 1024 * 1024, // 100MB
	}
}

// NewCache 创建新的缓存
func NewCache(config *CacheConfig) *Cache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	return &Cache{
		items:   make(map[string]*CacheItem),
		lruList: NewLRUList(),
		config:  config,
		stats:   &CacheStats{},
		cleanup: make(chan struct{}),
	}
}

// Start 启动缓存
func (c *Cache) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return fmt.Errorf("cache is already running")
	}

	if c.config.CleanupInterval > 0 {
		go c.cleanupLoop(ctx)
	}

	return nil
}

// Stop 停止缓存
func (c *Cache) Stop() {
	if atomic.CompareAndSwapInt32(&c.running, 1, 0) {
		close(c.cleanup)
	}
}

// Get 从缓存中获取值
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		atomic.AddUint64(&c.stats.Misses, 1)
		return nil, false
	}

	// 检查过期时间
	if !item.expiry.IsZero() && time.Now().After(item.expiry) {
		c.Delete(key)
		atomic.AddUint64(&c.stats.Misses, 1)
		return nil, false
	}

	// 更新访问信息
	item.accessTime = time.Now()
	atomic.AddUint64(&item.accessCount, 1)

	// 移动到LRU列表前端
	c.mu.Lock()
	c.lruList.MoveToFront(item)
	c.mu.Unlock()

	atomic.AddUint64(&c.stats.Hits, 1)
	c.updateHitRatio()

	return item.value, true
}

// Set 在缓存中设置值
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.config.TTL)
}

// SetWithTTL 在缓存中设置带特定TTL的值
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := c.calculateSize(value)
	expiry := time.Time{}
	if ttl > 0 {
		expiry = time.Now().Add(ttl)
	}

	// 检查项目是否已存在
	if existingItem, exists := c.items[key]; exists {
		// 更新现有项目
		c.stats.MemoryUsage -= existingItem.size
		existingItem.value = value
		existingItem.size = size
		existingItem.expiry = expiry
		existingItem.accessTime = time.Now()
		c.lruList.MoveToFront(existingItem)
		c.stats.MemoryUsage += size
		return
	}

	// 创建新项目
	item := &CacheItem{
		key:        key,
		value:      value,
		size:       size,
		expiry:     expiry,
		accessTime: time.Now(),
	}

	// 检查是否需要驱逐项目
	c.evictIfNeeded(size)

	// 添加新项目
	c.items[key] = item
	c.lruList.AddToFront(item)
	c.stats.Size++
	c.stats.MemoryUsage += size
}

// Delete 从缓存中删除值
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists {
		delete(c.items, key)
		c.lruList.Remove(item)
		c.stats.Size--
		c.stats.MemoryUsage -= item.size
	}
}

// Clear 清除缓存中的所有项目
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
	c.lruList = NewLRUList()
	c.stats.Size = 0
	c.stats.MemoryUsage = 0
}

// evictIfNeeded 如有必要则驱逐项目
func (c *Cache) evictIfNeeded(newItemSize uint64) {
	// 检查大小限制
	for c.stats.Size >= c.config.MaxSize {
		c.evictLRU()
	}

	// 检查内存限制
	for c.stats.MemoryUsage+newItemSize > c.config.MaxMemory {
		c.evictLRU()
	}
}

// evictLRU 驱逐最近最少使用的项目
func (c *Cache) evictLRU() {
	if c.lruList.tail != nil {
		item := c.lruList.tail
		delete(c.items, item.key)
		c.lruList.Remove(item)
		c.stats.Size--
		c.stats.MemoryUsage -= item.size
		atomic.AddUint64(&c.stats.Evictions, 1)
	}
}

// calculateSize 计算值的近似大小
func (c *Cache) calculateSize(value interface{}) uint64 {
	switch v := value.(type) {
	case string:
		return uint64(len(v))
	case []byte:
		return uint64(len(v))
	case int, int32, int64, uint, uint32, uint64:
		return 8
	case float32, float64:
		return 8
	case bool:
		return 1
	default:
		// 使用unsafe.Sizeof的粗略估计
		return uint64(unsafe.Sizeof(v))
	}
}

// updateHitRatio 更新缓存命中率
func (c *Cache) updateHitRatio() {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRatio = float64(c.stats.Hits) / float64(total) * 100
	}
}

// cleanupLoop 执行定期清理
func (c *Cache) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.cleanup:
			return
		case <-ticker.C:
			c.performCleanup()
		}
	}
}

// performCleanup 移除过期项目
func (c *Cache) performCleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, item := range c.items {
		if !item.expiry.IsZero() && now.After(item.expiry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		if item, exists := c.items[key]; exists {
			delete(c.items, key)
			c.lruList.Remove(item)
			c.stats.Size--
			c.stats.MemoryUsage -= item.size
			atomic.AddUint64(&c.stats.Evictions, 1)
		}
	}
}

// GetStats 返回缓存统计信息
func (c *Cache) GetStats() *CacheStats {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	stats := *c.stats
	return &stats
}

// LRU 列表实现

// NewLRUList 创建新的LRU列表
func NewLRUList() *LRUList {
	return &LRUList{}
}

// AddToFront 将项目添加到列表前端
func (l *LRUList) AddToFront(item *CacheItem) {
	if l.head == nil {
		l.head = item
		l.tail = item
	} else {
		item.next = l.head
		l.head.prev = item
		l.head = item
	}
	l.size++
}

// Remove 从列表中移除项目
func (l *LRUList) Remove(item *CacheItem) {
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		l.head = item.next
	}

	if item.next != nil {
		item.next.prev = item.prev
	} else {
		l.tail = item.prev
	}

	item.prev = nil
	item.next = nil
	l.size--
}

// MoveToFront 将项目移动到列表前端
func (l *LRUList) MoveToFront(item *CacheItem) {
	if l.head == item {
		return
	}

	l.Remove(item)
	l.AddToFront(item)
}

// GC 优化器实现

// DefaultGCConfig 返回默认GC配置
func DefaultGCConfig() *GCConfig {
	return &GCConfig{
		TargetPercent:    100,
		OptimizeInterval: 30 * time.Second,
		MemoryThreshold:  500 * 1024 * 1024, // 500MB
		ForceGCThreshold: 1024 * 1024 * 1024, // 1GB
		EnableMetrics:    true,
		Adaptive:         true,
	}
}

// NewGCOptimizer 创建新的GC优化器
func NewGCOptimizer(config *GCConfig) *GCOptimizer {
	if config == nil {
		config = DefaultGCConfig()
	}

	return &GCOptimizer{
		config: config,
		stopCh: make(chan struct{}),
		stats:  &GCStats{},
	}
}

// Start 启动GC优化器
func (gco *GCOptimizer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&gco.running, 0, 1) {
		return fmt.Errorf("GC optimizer is already running")
	}

	// 设置初始GC目标
	runtime.SetGCPercent(gco.config.TargetPercent)
	runtime.ReadMemStats(&gco.lastGCStats)
	gco.lastOptimization = time.Now()

	go gco.optimizeLoop(ctx)
	return nil
}

// Stop 停止GC优化器
func (gco *GCOptimizer) Stop() {
	if atomic.CompareAndSwapInt32(&gco.running, 1, 0) {
		close(gco.stopCh)
	}
}

// optimizeLoop 运行GC优化循环
func (gco *GCOptimizer) optimizeLoop(ctx context.Context) {
	ticker := time.NewTicker(gco.config.OptimizeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-gco.stopCh:
			return
		case <-ticker.C:
			gco.optimize()
		}
	}
}

// optimize 执行GC优化
func (gco *GCOptimizer) optimize() {
	gco.mu.Lock()
	defer gco.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 更新统计信息
	gco.updateStats(&memStats)

	// 检查是否需要强制GC
	if memStats.Alloc > gco.config.ForceGCThreshold {
		runtime.GC()
		atomic.AddUint64(&gco.stats.Optimizations, 1)
	}

	// 自适应GC调优
	if gco.config.Adaptive {
		gco.adaptiveOptimize(&memStats)
	}

	gco.lastGCStats = memStats
	gco.lastOptimization = time.Now()
}

// updateStats 更新GC统计信息
func (gco *GCOptimizer) updateStats(memStats *runtime.MemStats) {
	gco.stats.mu.Lock()
	defer gco.stats.mu.Unlock()

	if memStats.NumGC > gco.lastGCStats.NumGC {
		gco.stats.GCCount = memStats.NumGC
		gco.stats.LastGCTime = time.Unix(0, int64(memStats.LastGC))

		// 计算最近GC的暂停时间
		var totalPause time.Duration
		gcCount := memStats.NumGC - gco.lastGCStats.NumGC
		for i := uint32(0); i < gcCount && i < 256; i++ {
			idx := (memStats.NumGC - 1 - i) % 256
			totalPause += time.Duration(memStats.PauseNs[idx])
		}

		gco.stats.TotalPauseTime += totalPause
		if gco.stats.GCCount > 0 {
			gco.stats.AveragePauseTime = gco.stats.TotalPauseTime / time.Duration(gco.stats.GCCount)
		}

		// 计算释放的内存
		if gco.lastGCStats.Alloc > memStats.Alloc {
			gco.stats.MemoryFreed += gco.lastGCStats.Alloc - memStats.Alloc
		}
	}
}

// adaptiveOptimize 执行自适应GC优化
func (gco *GCOptimizer) adaptiveOptimize(memStats *runtime.MemStats) {
	// 根据内存使用模式调整GC目标
	memoryPressure := float64(memStats.Alloc) / float64(gco.config.MemoryThreshold)

	var newTarget int
	if memoryPressure > 1.0 {
		// 高内存压力，更积极
		newTarget = int(float64(gco.config.TargetPercent) * 0.5)
	} else if memoryPressure > 0.8 {
		// 中等内存压力
		newTarget = int(float64(gco.config.TargetPercent) * 0.75)
	} else {
		// 低内存压力，不那么积极
		newTarget = gco.config.TargetPercent
	}

	if newTarget < 10 {
		newTarget = 10
	}
	if newTarget > 500 {
		newTarget = 500
	}

	runtime.SetGCPercent(newTarget)
}

// ForceGC 强制垃圾回收
func (gco *GCOptimizer) ForceGC() {
	runtime.GC()
	atomic.AddUint64(&gco.stats.Optimizations, 1)
}

// GetStats 返回GC统计信息
func (gco *GCOptimizer) GetStats() *GCStats {
	gco.stats.mu.RLock()
	defer gco.stats.mu.RUnlock()

	stats := *gco.stats
	return &stats
}