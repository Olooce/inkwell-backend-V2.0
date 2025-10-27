package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var (
	rateLimiters = make(map[string]*rate.Limiter)
	mu           sync.Mutex
)

func getLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	if limiter, exists := rateLimiters[ip]; exists {
		return limiter
	}

	// Allow 5 requests per second with burst of 10
	limiter := rate.NewLimiter(5, 10)
	rateLimiters[ip] = limiter

	// Cleanup expired entries
	go func() {
		time.Sleep(time.Minute)
		mu.Lock()
		delete(rateLimiters, ip)
		mu.Unlock()
	}()

	return limiter
}

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := getLimiter(ip)

		if !limiter.Allow() {
			c.JSON(429, gin.H{"error": "Too many requests"})
			c.Abort()
			return
		}

		c.Next()
	}
}
