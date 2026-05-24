package middleware

import (
	"go-distributed/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	limit  int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now().UnixNano()
		windowStart := now - int64(rl.window)
		key := "rate_limit:" + ip

		pipe := utils.RedisClient.Pipeline()
		pipe.ZRemRangeByScore(utils.Ctx, key, "0", strconv.FormatInt(windowStart, 10))
		pipe.ZAdd(utils.Ctx, key, redis.Z{Score: float64(now), Member: now})
		countCmd := pipe.ZCard(utils.Ctx, key)
		pipe.Expire(utils.Ctx, key, rl.window)

		_, err := pipe.Exec(utils.Ctx)
		if err != nil {
			c.Next()
			return
		}

		if countCmd.Val() > int64(rl.limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests, please try later."})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (rl *RateLimiter) StartCleanup(interval time.Duration) {
	// No longer needed, Redis handles expiration
}
