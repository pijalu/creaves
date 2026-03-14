package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
)

// SetRenderEngine allows setting the render engine from the actions package
var renderEngine *render.Engine

// SetRenderEngine sets the global render engine for templates
func SetRenderEngine(engine *render.Engine) {
	renderEngine = engine
}

// RateLimiterConfig holds the configuration for rate limiting
type RateLimiterConfig struct {
	RequestsPerMinute int           // Maximum requests per minute
	LockoutAttempts   int           // Number of failed attempts before lockout
	LockoutDuration   time.Duration // Duration of lockout after too many failed attempts
}

// DefaultRateLimiterConfig returns a default configuration
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerMinute: 5,
		LockoutAttempts:   5,
		LockoutDuration:   15 * time.Minute,
	}
}

// tokenBucket implements a simple token bucket algorithm
type tokenBucket struct {
	tokens     int
	maxTokens  int
	lastRefill time.Time
	mu         sync.Mutex
}

// newTokenBucket creates a new token bucket
func newTokenBucket(maxTokens int) *tokenBucket {
	return &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		lastRefill: time.Now(),
	}
}

// consume tries to consume a token, returns true if successful
func (tb *tokenBucket) consume() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tokensToAdd := int(elapsed.Minutes()) * tb.maxTokens

	if tokensToAdd > 0 {
		tb.tokens = min(tb.maxTokens, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}

	// Consume a token if available
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// rateLimitEntry tracks rate limiting state for a single IP/user
type rateLimitEntry struct {
	bucket        *tokenBucket
	failedAttempts int
	lockedUntil   time.Time
	mu            sync.Mutex
}

// RateLimiter middleware for rate limiting
type RateLimiter struct {
	config RateLimiterConfig
	entries map[string]*rateLimitEntry
	mu      sync.RWMutex
}

// NewRateLimiter creates a new rate limiter middleware
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		config:  config,
		entries: make(map[string]*rateLimitEntry),
	}
}

// getEntry gets or creates a rate limit entry for a key
func (rl *RateLimiter) getEntry(key string) *rateLimitEntry {
	rl.mu.RLock()
	entry, exists := rl.entries[key]
	rl.mu.RUnlock()

	if exists {
		return entry
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists = rl.entries[key]; exists {
		return entry
	}

	entry = &rateLimitEntry{
		bucket: newTokenBucket(rl.config.RequestsPerMinute),
	}
	rl.entries[key] = entry
	return entry
}

// cleanupOldEntries removes entries that haven't been used recently
func (rl *RateLimiter) cleanupOldEntries() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.entries {
		entry.mu.Lock()
		// Remove if locked until has passed and no recent activity
		if entry.lockedUntil.Before(now) && entry.bucket.tokens >= rl.config.RequestsPerMinute-1 {
			delete(rl.entries, key)
		}
		entry.mu.Unlock()
	}
}

// getClientKey extracts a unique identifier for the client (IP address)
func getClientKey(c buffalo.Context) string {
	// Check for X-Forwarded-For header first
	xff := c.Request().Header.Get("X-Forwarded-For")
	if xff != "" {
		return xff
	}

	// Check for X-Real-IP header
	xri := c.Request().Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return c.Request().RemoteAddr
}

// RecordFailedAttempt records a failed authentication attempt
func (rl *RateLimiter) RecordFailedAttempt(c buffalo.Context) {
	key := getClientKey(c)
	entry := rl.getEntry(key)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.failedAttempts++
}

// RecordSuccess records a successful authentication
func (rl *RateLimiter) RecordSuccess(c buffalo.Context) {
	key := getClientKey(c)
	entry := rl.getEntry(key)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Reset failed attempts on successful login
	entry.failedAttempts = 0
}

// IsLocked checks if the client is currently locked out
func (rl *RateLimiter) IsLocked(c buffalo.Context) bool {
	key := getClientKey(c)
	entry := rl.getEntry(key)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if entry.lockedUntil.After(time.Now()) {
		return true
	}

	// Check if we should lock out
	if entry.failedAttempts >= rl.config.LockoutAttempts {
		entry.lockedUntil = time.Now().Add(rl.config.LockoutDuration)
		return true
	}

	return false
}

// Allow checks if the request should be allowed
func (rl *RateLimiter) Allow(c buffalo.Context) bool {
	key := getClientKey(c)
	entry := rl.getEntry(key)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Check if locked
	if entry.lockedUntil.After(time.Now()) {
		return false
	}

	// Check if lockout threshold reached
	if entry.failedAttempts >= rl.config.LockoutAttempts {
		entry.lockedUntil = time.Now().Add(rl.config.LockoutDuration)
		return false
	}

	// Try to consume a token
	return entry.bucket.consume()
}

// Handler returns the Buffalo middleware handler
func (rl *RateLimiter) Handler(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		// Periodically cleanup old entries
		go rl.cleanupOldEntries()

		if !rl.Allow(c) {
			entry := rl.getEntry(getClientKey(c))
			entry.mu.Lock()
			lockedUntil := entry.lockedUntil
			entry.mu.Unlock()

			if lockedUntil.After(time.Now()) {
				// Account is locked due to too many failed attempts
				c.Flash().Add("danger", "Too many failed attempts. Please try again later.")
				return c.Render(http.StatusTooManyRequests, renderEngine.HTML("/auth/locked.plush.html"))
			}

			// Rate limit exceeded
			c.Flash().Add("warning", "Too many requests. Please wait a moment.")
			return c.Render(http.StatusTooManyRequests, renderEngine.HTML("/auth/ratelimited.plush.html"))
		}

		return next(c)
	}
}

// AuthRateLimiter is the global rate limiter for authentication endpoints
var AuthRateLimiter *RateLimiter

func init() {
	AuthRateLimiter = NewRateLimiter(DefaultRateLimiterConfig())
}
