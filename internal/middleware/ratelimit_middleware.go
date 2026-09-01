package middleware

import (
	"net/http"
	"sync"
	"time"

	"go-hephaestus/internal/logger"

	"github.com/gin-gonic/gin"
)

type clientBucket struct {
	tokens     float64
	lastUpdate time.Time
	failedAuth int
	lockoutEnd time.Time
}

type RateLimiter struct {
	rate        float64 // tokens per second
	capacity    float64 // max burst capacity
	clients     map[string]*clientBucket
	mu          sync.Mutex
	maxFailures int
	lockoutTime time.Duration
}

var (
	generalLimiter *RateLimiter
	authLimiter    *RateLimiter
	initOnce       sync.Once
)

func getRateLimiters() (*RateLimiter, *RateLimiter) {
	initOnce.Do(func() {
		// General API: 120 req/min (2 req/s, burst 30)
		generalLimiter = &RateLimiter{
			rate:        2.0,
			capacity:    30.0,
			clients:     make(map[string]*clientBucket),
			maxFailures: 0,
		}

		// Auth Login: 5 attempts/min (0.1 req/s, burst 5, 15 min lockout on 5 fails)
		authLimiter = &RateLimiter{
			rate:        0.1,
			capacity:    5.0,
			clients:     make(map[string]*clientBucket),
			maxFailures: 5,
			lockoutTime: 15 * time.Minute,
		}

		// Background cleanup of stale buckets every 5 minutes
		go cleanupStaleBuckets(generalLimiter, authLimiter)
	})
	return generalLimiter, authLimiter
}

// GeneralRateLimitMiddleware limits general API traffic
func GeneralRateLimitMiddleware() gin.HandlerFunc {
	limiter, _ := getRateLimiters()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "Too Many Requests",
				"message": "API rate limit exceeded. Please slow down your requests.",
			})
			return
		}
		c.Next()
	}
}

// AuthRateLimitMiddleware protects against brute-force login attacks
func AuthRateLimitMiddleware() gin.HandlerFunc {
	_, limiter := getRateLimiters()
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if locked, remaining := limiter.isLockedOut(ip); locked {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "Account Locked",
				"message": fmt.Sprintf("Too many failed login attempts. IP temporarily locked. Try again in %d seconds.", int(remaining.Seconds())),
			})
			return
		}

		if !limiter.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "Too Many Requests",
				"message": "Login attempt rate limit exceeded. Please wait before trying again.",
			})
			return
		}

		c.Next()

		// If login failed (401 Unauthorized), track failure
		if c.Writer.Status() == http.StatusUnauthorized {
			limiter.recordFailure(ip)
			logger.Warn("Security", fmt.Sprintf("Failed login attempt from IP %s", ip))
		} else if c.Writer.Status() == http.StatusOK {
			limiter.resetFailure(ip)
		}
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.clients[ip]
	if !exists {
		rl.clients[ip] = &clientBucket{
			tokens:     rl.capacity - 1,
			lastUpdate: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.tokens = min(rl.capacity, b.tokens+elapsed*rl.rate)
	b.lastUpdate = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}

	return false
}

func (rl *RateLimiter) isLockedOut(ip string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.clients[ip]
	if !exists {
		return false, 0
	}

	if time.Now().Before(b.lockoutEnd) {
		return true, b.lockoutEnd.Sub(time.Now())
	}
	return false, 0
}

func (rl *RateLimiter) recordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.clients[ip]
	if !exists {
		b = &clientBucket{lastUpdate: time.Now()}
		rl.clients[ip] = b
	}

	b.failedAuth++
	if rl.maxFailures > 0 && b.failedAuth >= rl.maxFailures {
		b.lockoutEnd = time.Now().Add(rl.lockoutTime)
		logger.Warn("Security", fmt.Sprintf("IP %s locked out for %v due to %d consecutive failed logins", ip, rl.lockoutTime, b.failedAuth))
	}
}

func (rl *RateLimiter) resetFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if b, exists := rl.clients[ip]; exists {
		b.failedAuth = 0
		b.lockoutEnd = time.Time{}
	}
}

func cleanupStaleBuckets(limiters ...*RateLimiter) {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		now := time.Now()
		for _, rl := range limiters {
			rl.mu.Lock()
			for ip, b := range rl.clients {
				if now.Sub(b.lastUpdate) > 30*time.Minute && now.After(b.lockoutEnd) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
