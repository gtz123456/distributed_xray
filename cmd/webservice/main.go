package main

import (
	"context"
	"fmt"
	"go-distributed/log"
	"go-distributed/registry"
	"go-distributed/service"
	"go-distributed/utils"
	"go-distributed/web/controllers"
	"go-distributed/web/db"
	"go-distributed/web/middleware"
	stlog "log"
	"math/rand"
	"os"
	"time"

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
	host, err := utils.GetPublicIP()
	if err != nil {
		stlog.Fatalln("Error getting host IP:", err)
	}
	port := os.Getenv("Web_Port")
	if port == "" {
		port = "80"
	}

	GINPORT := os.Getenv("GIN_Port")
	if GINPORT == "" {
		GINPORT = "8080"
	}

	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)

	publicIP, err := utils.GetPublicIP()

	if err != nil {
		stlog.Fatalln("Error getting public IP:", err)
	}

	reg := registry.Registration{
		ServiceName:      registry.WebService,
		ServiceURL:       fmt.Sprintf("http://%v:%v", host, GINPORT),
		PublicIP:         publicIP,
		RequiredServices: []registry.ServiceName{registry.NodeService, registry.LogService},
		ServiceUpdateURL: serviceAddress + "/service",
	}

	_, err = service.Start(context.Background(), host, port, reg, log.RegisterHandlers)
	if err != nil {
		stlog.Fatalln(err)
	}

	var logProviders []registry.Registration

	for {
		logProviders, err = registry.GetProviders(registry.LogService)

		if err != nil {
			stlog.Println("Error getting log service:" + err.Error() + ". Retrying in 3 seconds")
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}

	fmt.Printf("Logging service found at %s\n", logProviders)
	logProvider := logProviders[rand.Intn(len(logProviders))]
	log.SetClientLogger(logProvider.ServiceURL, reg.ServiceName)

	controllers.StartHeartbeatMonitor()
	controllers.StartPlanMonitor()

	r := gin.Default()
	r.Use(CORSMiddleware())
	r.POST("/signup", controllers.Signup)
	r.POST("/login", controllers.Login)
	r.GET("/user", middleware.RequireAuth, controllers.User)
	r.GET("/realitykey", middleware.RequireAuth, controllers.Realitykey)
	r.GET("/servers", middleware.RequireAuth, controllers.Servers)
	r.GET("/version", controllers.Version)
	r.POST("connect", middleware.RequireAuth, controllers.Connect)
	r.POST("/heartbeat", middleware.RequireAuth, controllers.HeartbeatFromClient)

	r.POST("/traffic", controllers.AddTraffic)

	// Admin routes
	r.POST("/admin/setplan", controllers.SetPlan)
	r.Run()

}
