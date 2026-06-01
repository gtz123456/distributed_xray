package middleware

import (
	"fmt"
	"go-distributed/utils"
	"go-distributed/web/db"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RequireAuth(c *gin.Context) {
	// Get the cookie off the request
	tokenString := c.GetHeader("Authorization")

	if tokenString == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	isBlacklisted, _ := utils.RedisClient.Get(utils.Ctx, "jwt_blacklist:"+tokenString).Result()
	if isBlacklisted == "true" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Decode/validate it
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(os.Getenv("SECRET")), nil
	})

	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Check the expiry date
		if float64(time.Now().Unix()) > claims["exp"].(float64) {
			c.AbortWithStatus(http.StatusUnauthorized)
		}

		// Find the user with token Subject
		var user db.User
		db.DB.First(&user, claims["sub"])

		if user.ID == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
		}

		// Attach the request
		c.Set("user", user)

		//Continue
		c.Next()
	} else {
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}

// AdminAuth verifies the request is from a human administrator.
// Only accepts the REGKEY env var (uppercase). Inter-service keys are rejected.
func AdminAuth(c *gin.Context) {
	regkey := c.GetHeader("regkey")
	if regkey == "" {
		regkey = c.Query("regkey")
	}

	expectedAdminKey := os.Getenv("REGKEY")

	if regkey == "" || expectedAdminKey == "" || regkey != expectedAdminKey {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.Next()
}

// ServiceAuth verifies the request is from a trusted internal service.
// Only accepts the regkey env var (lowercase). Admin keys are rejected.
func ServiceAuth(c *gin.Context) {
	regkey := c.GetHeader("regkey")

	expectedServiceKey := os.Getenv("regkey")

	if regkey == "" || expectedServiceKey == "" || regkey != expectedServiceKey {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.Next()
}

