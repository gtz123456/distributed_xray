package middleware

import (
	"go-distributed/utils"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mr.Close()

	utils.RedisClient = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// limit to 3 requests per second
	limiter := NewRateLimiter(3, time.Second)
	r.GET("/ping", limiter.Middleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("Allow under limit", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
			req.RemoteAddr = "192.168.1.1:1234"
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("Block over limit", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("Allow after window expires", func(t *testing.T) {
		// Fast forward miniredis time
		mr.FastForward(2 * time.Second)

		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Different IP has its own limit", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
			req.RemoteAddr = "192.168.1.2:1234"
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		}
	})
}
