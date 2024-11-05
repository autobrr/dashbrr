package cache

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

var (
	ErrKeyNotFound = redis.Nil
)

const (
	PrefixSession  = "session:"
	PrefixHealth   = "health:"
	PrefixVersion  = "version:"
	PrefixRate     = "rate:"
	DefaultTimeout = 5 * time.Second
	RetryAttempts  = 2
	RetryDelay     = 50 * time.Millisecond

	// Cache durations
	DefaultTTL  = 15 * time.Minute
	HealthTTL   = 30 * time.Minute
	StatsTTL    = 5 * time.Minute
	SessionsTTL = 1 * time.Minute

	CleanupInterval = 1 * time.Minute // Increased to reduce cleanup frequency
)

// Cache represents a Redis cache instance with local memory cache
type Cache struct {
	client *redis.Client
	local  *LocalCache
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup // Added WaitGroup for graceful shutdown
	closed bool
	mu     sync.RWMutex
}

// LocalCache provides in-memory caching to reduce Redis hits
type LocalCache struct {
	sync.RWMutex
	items map[string]*localCacheItem
}

type localCacheItem struct {
	value      []byte
	expiration time.Time
}

// NewCache creates a new Redis cache instance with optimized configuration
func NewCache(addr string) (*Cache, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Development-optimized Redis configuration
	client := redis.NewClient(&redis.Options{
		Addr:            addr,
		PoolSize:        10,
		MinIdleConns:    2,
		MaxConnAge:      5 * time.Minute,
		ReadTimeout:     DefaultTimeout,
		WriteTimeout:    DefaultTimeout,
		PoolTimeout:     DefaultTimeout * 2,
		IdleTimeout:     time.Minute,
		MaxRetries:      RetryAttempts,
		MinRetryBackoff: RetryDelay,
		MaxRetryBackoff: time.Second,
	})

	// Test connection with simple retry
	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		ctxTimeout, cancelTimeout := context.WithTimeout(ctx, DefaultTimeout)
		err := client.Ping(ctxTimeout).Err()
		cancelTimeout()

		if err == nil {
			lastErr = nil
			break
		}

		lastErr = err
		if i < RetryAttempts-1 {
			time.Sleep(RetryDelay)
		}
	}

	if lastErr != nil {
		cancel() // Clean up context if connection fails
		if client != nil {
			client.Close()
		}
		log.Error().Err(lastErr).Str("addr", addr).Msg("Failed to connect to Redis after retries")
		return nil, lastErr
	}

	cache := &Cache{
		client: client,
		local: &LocalCache{
			items: make(map[string]*localCacheItem),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Start cleanup goroutine
	cache.wg.Add(1)
	go func() {
		defer cache.wg.Done()
		cache.localCacheCleanup()
	}()

	return cache, nil
}

// Get retrieves a value from cache with local cache first
func (c *Cache) Get(ctx context.Context, key string, value interface{}) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return redis.ErrClosed
	}
	c.mu.RUnlock()

	// Try local cache first
	if data, ok := c.getFromLocalCache(key); ok {
		if err := json.Unmarshal(data, value); err != nil {
			log.Error().Err(err).Str("key", key).Msg("Failed to unmarshal local cached value")
		} else {
			return nil
		}
	}

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			data, err := c.client.Get(timeoutCtx, key).Bytes()
			cancel()

			if err == nil {
				// Store in local cache with same TTL as Redis
				ttl := c.client.TTL(ctx, key).Val()
				if ttl < 0 {
					if strings.HasPrefix(key, PrefixHealth) {
						ttl = HealthTTL
					} else if strings.HasPrefix(key, "sessions:") {
						ttl = SessionsTTL
					} else if strings.HasPrefix(key, "stats:") {
						ttl = StatsTTL
					} else {
						ttl = DefaultTTL
					}
				}
				c.setInLocalCache(key, data, ttl)
				return json.Unmarshal(data, value)
			}

			lastErr = err
			if err == redis.Nil {
				break
			}

			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}

	return lastErr
}

// Set stores a value in both Redis and local cache
func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return redis.ErrClosed
	}
	c.mu.RUnlock()

	if expiration == 0 {
		if strings.HasPrefix(key, PrefixHealth) {
			expiration = HealthTTL
		} else if strings.HasPrefix(key, "sessions:") {
			expiration = SessionsTTL
		} else if strings.HasPrefix(key, "stats:") {
			expiration = StatsTTL
		} else {
			expiration = DefaultTTL
		}
	}

	data, err := json.Marshal(value)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("Failed to marshal value for cache")
		return err
	}

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			err := c.client.Set(timeoutCtx, key, data, expiration).Err()
			cancel()

			if err == nil {
				c.setInLocalCache(key, data, expiration)
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}

	return lastErr
}

// Local cache methods
func (c *Cache) getFromLocalCache(key string) ([]byte, bool) {
	c.local.RLock()
	defer c.local.RUnlock()

	if item, exists := c.local.items[key]; exists {
		if time.Now().Before(item.expiration) {
			return item.value, true
		}
		delete(c.local.items, key)
	}
	return nil, false
}

func (c *Cache) setInLocalCache(key string, value []byte, ttl time.Duration) {
	c.local.Lock()
	defer c.local.Unlock()

	c.local.items[key] = &localCacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

func (c *Cache) localCacheCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				c.local.Lock()
				defer c.local.Unlock()

				now := time.Now()
				for key, item := range c.local.items {
					if now.After(item.expiration) {
						delete(c.local.items, key)
					}
				}
			}()
		case <-c.ctx.Done():
			return
		}
	}
}

// Delete removes a value from both Redis and local cache
func (c *Cache) Delete(ctx context.Context, key string) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return redis.ErrClosed
	}
	c.mu.RUnlock()

	// Remove from local cache immediately
	c.local.Lock()
	delete(c.local.items, key)
	c.local.Unlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			err := c.client.Del(timeoutCtx, key).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}

	return lastErr
}

// Rate limiting methods
func (c *Cache) Increment(ctx context.Context, key string, timestamp int64) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return redis.ErrClosed
	}
	c.mu.RUnlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			member := strconv.FormatInt(timestamp, 10)
			err := c.client.ZAdd(timeoutCtx, key, &redis.Z{
				Score:  float64(timestamp),
				Member: member,
			}).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return lastErr
}

func (c *Cache) CleanAndCount(ctx context.Context, key string, windowStart int64) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return redis.ErrClosed
	}
	c.mu.RUnlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			// Remove all entries strictly less than windowStart
			err := c.client.ZRemRangeByScore(timeoutCtx, key, "-inf", "("+strconv.FormatInt(windowStart, 10)).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return lastErr
}

func (c *Cache) GetCount(ctx context.Context, key string) (int64, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, redis.ErrClosed
	}
	c.mu.RUnlock()

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			count, err := c.client.ZCard(timeoutCtx, key).Result()
			cancel()

			if err == nil {
				return count, nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return 0, lastErr
}

func (c *Cache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return redis.ErrClosed
	}
	c.mu.RUnlock()

	if expiration == 0 {
		expiration = DefaultTTL
	}

	var lastErr error
	for i := 0; i < RetryAttempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			err := c.client.Expire(timeoutCtx, key, expiration).Err()
			cancel()

			if err == nil {
				return nil
			}

			lastErr = err
			if i < RetryAttempts-1 {
				time.Sleep(RetryDelay)
			}
		}
	}
	return lastErr
}

// Close closes the Redis connection and stops the cleanup goroutine
func (c *Cache) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return redis.ErrClosed
	}
	c.closed = true
	c.mu.Unlock()

	// Cancel context to stop cleanup goroutine
	c.cancel()

	// Wait for cleanup goroutine to finish
	c.wg.Wait()

	// Clear local cache
	func() {
		c.local.Lock()
		defer c.local.Unlock()
		c.local.items = make(map[string]*localCacheItem)
	}()

	// Close Redis client
	return c.client.Close()
}

// Rest of the methods remain unchanged...
