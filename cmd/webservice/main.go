package main

import (
	"go-distributed/utils"
	"go-distributed/web/controllers"
	"go-distributed/web/db"
	"go-distributed/web/middleware"

	"github.com/gin-gonic/gin"
)

func init() {
	utils.LoadEnv()
	db.Connect()
	db.Sync()
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {
	r := gin.Default()
	r.Use(CORSMiddleware())
	r.POST("/signup", controllers.Signup)
	r.POST("/login", controllers.Login)
	r.GET("/user", middleware.RequireAuth, controllers.User)
	r.GET("/realitykey", middleware.RequireAuth, controllers.Realitykey)
	r.GET("/servers", middleware.RequireAuth, controllers.Servers)
	r.Run()
}
