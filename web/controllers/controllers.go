package controllers

import (
	"encoding/json"
	"go-distributed/registry"
	"go-distributed/utils"
	"go-distributed/web/db"
	"go-distributed/web/email"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const MAX_CONNECTIONS_PER_USER = 2
const HEARTBEAT_TIMEOUT = 30 * time.Second
const HEARTBEAT_CHECK_INTERVAL = 10 * time.Second

var expireMap = make(map[string]time.Time)

var RateMap = map[string]int{
	"Free plan":    10 * 1000 * 1000 / 8,  // 10 Mbps
	"Premium plan": 200 * 1000 * 1000 / 8, // 200 Mbps
}

type Server struct {
	IP          string   `json:"ip"`
	IPV6        string   `json:"ipv6"`
	ServiceID   string   `json:"serviceid"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	// TrafficMultiplier int      `json:"traffic_multiplier"` // 0 means free, and for better servers the value will be higher
}

type UserConnection struct {
	NodeIP        string
	ServiceID     string
	NodePort      string
	ClientIP      string
	LastHeartBeat time.Time
}

var userConnectionMap = make(map[string][]UserConnection) // user UUID: UserConnection list
var userConnectionMapMutex = &sync.RWMutex{}

func Signup(c *gin.Context) {
	// Get the email/pass off req Body
	var body struct {
		Email    string
		Password string
	}

	if c.Bind(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read body",
		})

		return
	}

	// Hash the password
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 10)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to hash password.",
		})
		return
	}

	// Create the user
	UUID := uuid.New().String()

	user := db.User{
		Email:    body.Email,
		Password: string(hash),
		UUID:     UUID,
		Plan:     "Free plan",
		PlanEnd:  time.Now().Add(100 * 12 * 31 * 24 * time.Hour),

		RenewCycle: int64(31 * 24 * time.Hour), // renew every 31 days
		NextRenew:  time.Now().Add(31 * 24 * time.Hour),

		TrafficUsed:  0,
		TrafficLimit: 50 * 1000 * 1000 * 1000, // 50 GB for free trail

		IsVerified:  false,
		VerifyToken: uuid.New().String(),
		TokenExpiry: time.Now().Add(24 * time.Hour), // token valid for 24 hours
	}

	result := db.DB.Create(&user)

	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to create user." + result.Error.Error(),
		})

		return
	}

	// send verification email
	go func() {
		email.SendVerificationEmail(user.Email, user.VerifyToken)
	}()

	// Respond
	c.JSON(http.StatusOK, gin.H{})
}

func Login(c *gin.Context) {
	var body struct {
		Email    string
		Password string
	}

	if c.Bind(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read body",
		})

		return
	}

	var user db.User
	db.DB.First(&user, "email = ?", body.Email)
	if user.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	// check if email is verified
	if !user.IsVerified {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email not verified, please click the link in the verification email",
		})
		return
	}

	// Compare sent in password with saved users password
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	// Generate a JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(os.Getenv("SECRET")))

	// Store token in map
	expireMap[tokenString] = time.Now().Add(time.Hour * 24 * 30)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to create token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
	})
}

func User(c *gin.Context) {
	user, _ := c.Get("user")

	userinfo := user.(db.User)

	c.JSON(http.StatusOK, gin.H{
		"email":         userinfo.Email,
		"uuid":          userinfo.UUID,
		"plan":          userinfo.Plan,
		"plan_end":      userinfo.PlanEnd.Format(time.RFC3339),
		"renew_cycle":   strconv.FormatInt(userinfo.RenewCycle, 10),
		"next_renew":    userinfo.NextRenew.Format(time.RFC3339),
		"traffic_used":  userinfo.TrafficUsed,
		"traffic_limit": userinfo.TrafficLimit,
		"balance":       userinfo.Balance,
	})
}

func Realitykey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"pubkey": os.Getenv("REALITY_PUBKEY"),
	})
}

func Servers(c *gin.Context) {
	regs, err := registry.GetProviders(registry.NodeService)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch servers",
		})
		return
	}

	servers := []Server{}

	for _, reg := range regs {
		server := Server{
			IP:          reg.PublicIP,
			IPV6:        reg.PublicIPv6,
			ServiceID:   reg.ServiceID,
			Description: reg.Description,
			Tags:        reg.Tags,
		}

		servers = append(servers, server)
	}
	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
	})
}

func Version(c *gin.Context) {
	// returns if the version of client is supported or not
	SupportedVersions := map[string]bool{
		"0.1.0": true,
	}

	version := c.Query("client-version") // Get the client version from query parameter
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing client-version parameter",
		})
		return
	}

	if _, ok := SupportedVersions[version]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported version",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{})

}

func Connect(c *gin.Context) {
	clientIP := c.ClientIP()

	serviceID := c.Query("serviceid")

	user, ok := c.Get("user")

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get user ID",
		})
		return
	}

	userinfo := user.(db.User)

	// Check if the user has a valid plan
	if userinfo.PlanEnd.Before(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Your plan has expired. Please renew your plan to continue using the service.",
		})
		return
	}

	// check if user has enough traffic
	if userinfo.TrafficUsed >= userinfo.TrafficLimit && userinfo.TrafficLimit != -1 {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You have reached your traffic limit",
		})
		return
	}

	uuid := userinfo.UUID
	email := userinfo.Email

	// Get the server's api end point with the service ID
	regs, err := registry.GetProviders(registry.NodeService)
	if err != nil || len(regs) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch node service",
		})
		return
	}

	var server *registry.Registration
	for _, reg := range regs {
		if reg.ServiceID == serviceID {
			server = &reg
			break
		}
	}

	if server == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Node service not found",
		})
		return
	}

	plan := userinfo.Plan
	if plan == "" {
		plan = "Free plan" // Default to free plan if not set
	}

	rate := RateMap[plan]
	if rate == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid plan or rate limit not set for the plan",
		})
		return
	}

	apiEndpoint := server.PublicIP + ":" + os.Getenv("Node_Port")

	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+apiEndpoint+"/connect?uuid="+uuid+"&email="+email+"&clientip="+clientIP+"&rate="+strconv.Itoa(rate)+"&burst="+strconv.Itoa(rate)+"&regkey="+utils.Regkey(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create request to node service",
		})
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to connect to node service: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{
			"error": "Failed to connect to node service, status code: " + resp.Status,
		})
		return
	}

	// get the port from the response
	var responseBody struct {
		Port string `json:"port"`
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read response from node service: " + err.Error(),
		})
		return
	}

	if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read response from node service" + err.Error(),
		})
		return
	}

	// Respond with the node port and pubkey
	userConnectionMapMutex.Lock()
	defer userConnectionMapMutex.Unlock()

	userConnections := userConnectionMap[uuid]

	if len(userConnections) >= MAX_CONNECTIONS_PER_USER {
		userConnectionMap[uuid] = userConnectionMap[uuid][1:]
	}

	userConnectionMap[uuid] = append(userConnectionMap[uuid], UserConnection{
		NodeIP:        server.PublicIP,
		ServiceID:     serviceID,
		NodePort:      responseBody.Port,
		ClientIP:      clientIP,
		LastHeartBeat: time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{
		"port":   responseBody.Port,
		"uuid":   uuid,
		"pubkey": os.Getenv("REALITY_PUBKEY"), // TODO
	})
}

func HeartbeatFromClient(c *gin.Context) {
	// receive heartbeat from client and keep user connected to the node service
	serviceID := c.Query("serviceid")

	user, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get user ID",
		})
		return
	}

	userInfo, ok := user.(db.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user info",
		})
		return
	}

	userID := userInfo.UUID

	if serviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing serviceid or userid",
		})
		return
	}

	userConnectionMapMutex.Lock()
	defer userConnectionMapMutex.Unlock()

	_, ok = userConnectionMap[userID]

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not connected to any node service",
		})
		return
	}

	found := false

	for idx, conn := range userConnectionMap[userID] {
		if conn.ServiceID == serviceID {
			userConnectionMap[userID][idx].LastHeartBeat = time.Now() // Update the last heartbeat time
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not connected to the specified node service",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func AddTraffic(c *gin.Context) {
	var trafficReports []struct {
		UUID    string `json:"uuid"`
		Traffic int    `json:"traffic"`
	}

	if err := c.BindJSON(&trafficReports); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid traffic report format",
		})
		return
	}

	for _, report := range trafficReports {
		var user db.User
		if err := db.DB.First(&user, "uuid = ?", report.UUID).Error; err != nil {
			continue
		}
		user.TrafficUsed += report.Traffic
		db.DB.Model(&user).Update("traffic_used", user.TrafficUsed)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}

func Subscribe(c *gin.Context) {
	var req struct {
		Plan     string `json:"plan"`
		Duration int    `json:"duration"` // in months
	}

	user, ok := c.Get("user")

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get user ID",
		})
		return
	}

	userinfo := user.(db.User)

	if userinfo.UUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get user UUID",
		})
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	price := 300                   // TODO: get plan price from env
	amount := price * req.Duration // in cents

	if err := db.DB.First(&userinfo, "uuid = ?", userinfo.UUID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	// Check if user has enough balance
	if userinfo.Balance < amount {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": "Insufficient balance",
		})
		return
	}

	if req.Plan != "Premium plan" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid plan",
		})
		return
	}

	userinfo.Balance -= amount
	if userinfo.Plan == "Free plan" { // upgrade from free plan
		userinfo.TrafficUsed = 0 // reset traffic
		userinfo.PlanEnd = time.Now().AddDate(0, req.Duration, 0)
		userinfo.TrafficLimit = 50 * 1000 * 1000 * 1000 // 50 GB for free plan
	} else { // extend premium plan
		// TODO: reset traffic used when renewing premium plan ?
		userinfo.PlanEnd = userinfo.PlanEnd.AddDate(0, req.Duration, 0)
		userinfo.TrafficLimit = 200 * 1000 * 1000 * 1000 // 200 GB for premium plan
	}
	now := time.Now()
	userinfo.NextRenew = now.AddDate(0, 0, 31) // set next renew to 31 days from now

	userinfo.Plan = req.Plan
	db.DB.Save(&userinfo)

	// Subscribe the user to the service
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}
