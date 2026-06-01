package main

import (
	"context"
	"fmt"
	"go-distributed/log"
	"go-distributed/registry"
	"go-distributed/service"
	"go-distributed/utils"
	"go-distributed/web/config"
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
	utils.InitRedis()
	db.Connect()
	db.Sync()
	config.InitReferralConfig()
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

	GINPORT := os.Getenv("GIN_PORT")
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
		RequiredServices: []registry.ServiceName{registry.NodeService, registry.LogService, registry.PaymentService},
		ServiceUpdateURL: serviceAddress + "/service",
	}

	_, err = service.Start(context.Background(), "", port, reg, log.RegisterHandlers)
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
	log.SetClientLogger(logProvider.ServiceURL, reg)

	controllers.StartHeartbeatMonitor()
	controllers.StartPlanMonitor()
	go controllers.StartReleaseCacheMonitor()

	r := gin.Default()
	r.Use(CORSMiddleware())

	globalLimiter := middleware.NewRateLimiter(15, time.Minute) // 15 requests/min/IP

	r.POST("/signup", globalLimiter.Middleware(), controllers.Signup)
	r.GET("/verify", globalLimiter.Middleware(), controllers.VerifyEmail)
	r.POST("/login", globalLimiter.Middleware(), controllers.Login)
	r.POST("/logout", globalLimiter.Middleware(), controllers.Logout)
	r.GET("/user", globalLimiter.Middleware(), middleware.RequireAuth, controllers.User)
	r.GET("/realitykey", globalLimiter.Middleware(), middleware.RequireAuth, controllers.Realitykey)
	r.GET("/servers", globalLimiter.Middleware(), middleware.RequireAuth, controllers.Servers)
	r.GET("/version", globalLimiter.Middleware(), controllers.Version)
	r.GET("/releases", globalLimiter.Middleware(), controllers.GetReleases)
	r.Static("/downloads", "./public/releases")
	r.POST("/connect", globalLimiter.Middleware(), middleware.RequireAuth, controllers.Connect)
	r.POST("/subscribe", globalLimiter.Middleware(), middleware.RequireAuth, controllers.Subscribe)
	r.POST("/redeem", globalLimiter.Middleware(), middleware.RequireAuth, controllers.Redeem)

	r.POST("/heartbeat", middleware.RequireAuth, controllers.HeartbeatFromClient)
	r.POST("/traffic", controllers.AddTraffic)

	r.POST("/payment", globalLimiter.Middleware(), middleware.RequireAuth, controllers.Payment)
	r.GET("/payment/status/:order_id", globalLimiter.Middleware(), middleware.RequireAuth, controllers.GetPaymentStatus)
	r.GET("/payment/list", globalLimiter.Middleware(), middleware.RequireAuth, controllers.ListPayments)
	r.POST("/payment/callback", middleware.ServiceAuth, controllers.Callback)
	// Admin routes (only accept REGKEY, not inter-service key)
	admin := r.Group("/admin")
	admin.Use(middleware.AdminAuth)
	{
		// User management
		admin.GET("/users", controllers.ListUsers)
		admin.GET("/user/:email", controllers.GetUser)
		admin.POST("/user/:uuid/setplan", controllers.AdminSetPlan)
		admin.POST("/user/:uuid/resettraffic", controllers.AdminResetTraffic)
		admin.POST("/user/:uuid/addbalance", controllers.AdminAddBalance)
		admin.POST("/user/:uuid/ban", controllers.AdminBanUser)
		admin.POST("/user/:uuid/unban", controllers.AdminUnbanUser)
		admin.DELETE("/user/:uuid", controllers.AdminDeleteUser)
		// Connections & nodes
		admin.GET("/connections", controllers.AdminListConnections)
		admin.POST("/user/:uuid/disconnect", controllers.AdminDisconnectUser)
		admin.GET("/nodes", controllers.AdminListNodes)
		admin.GET("/nodes/:serviceid/shell", controllers.AdminNodeShell)
		// Vouchers
		admin.POST("/generatevoucher", controllers.GenerateVoucher)
		admin.GET("/vouchers", controllers.AdminListVouchers)
		admin.DELETE("/voucher/:code", controllers.AdminRevokeVoucher)
		// Stats
		admin.GET("/stats", controllers.AdminStats)
		admin.GET("/cluster", controllers.AdminCluster)
	}
	r.Run(":" + GINPORT)
}
