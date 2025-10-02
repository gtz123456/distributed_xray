package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TODO: store in Redis

type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		rl.mu.Lock()
		defer rl.mu.Unlock()

		times := rl.requests[ip]

		// clean up old requests
		var newTimes []time.Time
		for _, t := range times {
			if now.Sub(t) < rl.window {
				newTimes = append(newTimes, t)
			}
		}

		// check if exceeded limit
		if len(newTimes) >= rl.limit {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests, please try later."})
			c.Abort()
			return
		}

		// add current request
		newTimes = append(newTimes, now)
		rl.requests[ip] = newTimes

		c.Next()
	}
}

func (rl *RateLimiter) StartCleanup(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			rl.mu.Lock()
			now := time.Now()
			for ip, times := range rl.requests {
				var newTimes []time.Time
				for _, t := range times {
					if now.Sub(t) < rl.window {
						newTimes = append(newTimes, t)
					}
				}
				if len(newTimes) == 0 {
					delete(rl.requests, ip)
				} else {
					rl.requests[ip] = newTimes
				}
			}
			rl.mu.Unlock()
		}
	}()
}
