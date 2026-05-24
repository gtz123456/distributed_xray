package middleware

import (
	"go-distributed/utils"
	"go-distributed/web/db"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestEnv() (*miniredis.Miniredis, *gin.Engine) {
	// Setup miniredis
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	utils.RedisClient = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Setup SQLite in-memory DB
	database, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.DB = database
	db.DB.AutoMigrate(&db.User{})

	// Setup Gin
	gin.SetMode(gin.TestMode)
	r := gin.New()

	return mr, r
}

func TestRequireAuth(t *testing.T) {
	mr, r := setupTestEnv()
	defer mr.Close()

	os.Setenv("SECRET", "test_secret")

	// Create a test user
	user := db.User{Email: "test@example.com"}
	db.DB.Create(&user)

	r.GET("/protected", RequireAuth, func(c *gin.Context) {
		u, _ := c.Get("user")
		c.JSON(http.StatusOK, gin.H{"user_id": u.(db.User).ID})
	})

	// Generate a valid token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte("test_secret"))

	t.Run("Valid Token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", tokenString)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Missing Token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Blacklisted Token", func(t *testing.T) {
		utils.RedisClient.Set(utils.Ctx, "jwt_blacklist:"+tokenString, "true", time.Hour)

		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", tokenString)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Expired Token", func(t *testing.T) {
		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": user.ID,
			"exp": time.Now().Add(-time.Hour).Unix(),
		})
		expiredTokenString, _ := expiredToken.SignedString([]byte("test_secret"))

		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", expiredTokenString)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAdminAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	os.Setenv("REGKEY", "ADMIN_SECRET")
	os.Setenv("regkey", "service_secret")

	r.GET("/admin", AdminAuth, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("Valid Admin Key", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("regkey", "ADMIN_SECRET")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Invalid Admin Key (Service Key)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("regkey", "service_secret")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestServiceAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	os.Setenv("REGKEY", "ADMIN_SECRET")
	os.Setenv("regkey", "service_secret")

	r.GET("/service", ServiceAuth, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	t.Run("Valid Service Key", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/service", nil)
		req.Header.Set("regkey", "service_secret")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Invalid Service Key (Admin Key)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/service", nil)
		req.Header.Set("regkey", "ADMIN_SECRET")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
