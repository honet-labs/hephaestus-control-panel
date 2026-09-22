package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
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

// GetRealClientIP reliably extracts visitor IP behind Cloudflare, Nginx, or direct connection
func GetRealClientIP(c *gin.Context) string {
	// 1. Cloudflare connecting IP (if present)
	if cfIP := strings.TrimSpace(c.GetHeader("CF-Connecting-IP")); cfIP != "" {
		if ip := net.ParseIP(cfIP); ip != nil {
			return ip.String()
		}
	}

	// 2. X-Real-IP set by Nginx reverse proxy
	if realIP := strings.TrimSpace(c.GetHeader("X-Real-IP")); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return ip.String()
		}
	}

	// 3. Gin ClientIP() which uses trusted proxies and X-Forwarded-For
	if clientIP := strings.TrimSpace(c.ClientIP()); clientIP != "" {
		return clientIP
	}

	// 4. Fallback to raw RemoteAddr
	remoteHost, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil && remoteHost != "" {
		return remoteHost
	}

	return c.Request.RemoteAddr
}

func getRateLimiters() (*RateLimiter, *RateLimiter) {
	initOnce.Do(func() {
		// General API: 60 req/s with burst 120 to ensure smooth UI dashboard polling and tab switching
		generalLimiter = &RateLimiter{
			rate:        60.0,
			capacity:    120.0,
			clients:     make(map[string]*clientBucket),
			maxFailures: 0,
		}

		// Auth Login: 10 attempts/min (0.2 req/s, burst 10, 15 min lockout on 5 consecutive failed logins from that IP)
		authLimiter = &RateLimiter{
			rate:        0.2,
			capacity:    10.0,
			clients:     make(map[string]*clientBucket),
			maxFailures: 5,
			lockoutTime: 15 * time.Minute,
		}

		// Background cleanup of stale buckets every 5 minutes
		go cleanupStaleBuckets(generalLimiter, authLimiter)
	})
	return generalLimiter, authLimiter
}

// GeneralRateLimitMiddleware limits general API traffic without hindering normal UI navigation
func GeneralRateLimitMiddleware() gin.HandlerFunc {
	limiter, _ := getRateLimiters()
	return func(c *gin.Context) {
		ip := GetRealClientIP(c)
		if !limiter.allow(ip) {
			c.Header("Retry-After", "1")
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
// Uses per-IP and per-IP+username protection to prevent shared account lockout (N-01)
func AuthRateLimitMiddleware() gin.HandlerFunc {
	_, limiter := getRateLimiters()
	return func(c *gin.Context) {
		ip := GetRealClientIP(c)

		// Extract target username from login payload
		username := ""
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore body for subsequent handler binding
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				var req struct {
					Username string `json:"username"`
				}
				if err := json.Unmarshal(bodyBytes, &req); err == nil && req.Username != "" {
					username = strings.ToLower(strings.TrimSpace(req.Username))
				}
			}
		}

		ipKey := "ip:" + ip
		// Scope username lockout to the specific visitor IP to prevent shared lockout Denial of Service (N-01)
		userKey := ""
		if username != "" {
			userKey = "ip_user:" + ip + ":" + username
		}

		// 1. Check lockout for IP
		if locked, remaining := limiter.isLockedOut(ipKey); locked {
			retrySec := int(remaining.Seconds())
			if retrySec <= 0 {
				retrySec = 1
			}
			c.Header("Retry-After", fmt.Sprintf("%d", retrySec))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "Account Locked",
				"message": fmt.Sprintf("Too many failed login attempts. IP temporarily locked. Try again in %d seconds.", retrySec),
			})
			return
		}

		// 2. Check lockout for target Account from this IP
		if userKey != "" {
			if locked, remaining := limiter.isLockedOut(userKey); locked {
				retrySec := int(remaining.Seconds())
				if retrySec <= 0 {
					retrySec = 1
				}
				c.Header("Retry-After", fmt.Sprintf("%d", retrySec))
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"success": false,
					"error":   "Account Locked",
					"message": fmt.Sprintf("Too many failed login attempts for this account from your IP. Temporarily locked. Try again in %d seconds.", retrySec),
				})
				return
			}
		}

		// 3. Check burst rate allowance for IP
		if !limiter.allow(ipKey) {
			c.Header("Retry-After", "2")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "Too Many Requests",
				"message": "Login attempt rate limit exceeded. Please wait before trying again.",
			})
			return
		}

		c.Next()

		// If login failed (401 Unauthorized), track failure on both IP and account
		if c.Writer.Status() == http.StatusUnauthorized {
			limiter.recordFailure(ipKey)
			if userKey != "" {
				limiter.recordFailure(userKey)
			}
			logger.Warn("Security", fmt.Sprintf("Failed login attempt from IP %s for user '%s'", ip, username))
		} else if c.Writer.Status() == http.StatusOK {
			limiter.resetFailure(ipKey)
			if userKey != "" {
				limiter.resetFailure(userKey)
			}
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
