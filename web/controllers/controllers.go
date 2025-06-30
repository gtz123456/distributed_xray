package controllers

import (
	"encoding/json"
	"go-distributed/registry"
	"go-distributed/web/db"
	"io"
	"net/http"
	"os"
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
		Email:     body.Email,
		Password:  string(hash),
		UUID:      UUID,
		Plan:      "Free plan",
		PlanStart: time.Now(),
		PlanEnd:   time.Now().Add(100 * 365 * 24 * time.Hour),

		RenewCycle: 31 * 24 * time.Hour, // renew every 31 days
		NextRenew:  time.Now().Add(31 * 24 * time.Hour),

		TrafficUsed:  0,
		TrafficLimit: 50, // 50 GB for free plan
	}

	result := db.DB.Create(&user)

	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to create user." + result.Error.Error(),
		})
	}

	// Respond
	c.JSON(http.StatusOK, gin.H{})
}

func Login(c *gin.Context) {
	// Get email & pass off req body
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

	// Look up for requested user
	var user db.User

	db.DB.First(&user, "email = ?", body.Email)

	if user.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid email or password",
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
		"plan_start":    userinfo.PlanStart.Format(time.RFC3339),
		"plan_end":      userinfo.PlanEnd.Format(time.RFC3339),
		"renew_cycle":   userinfo.RenewCycle.String(),
		"next_renew":    userinfo.NextRenew.Format(time.RFC3339),
		"traffic_used":  userinfo.TrafficUsed,
		"traffic_limit": userinfo.TrafficLimit,
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
			Tags:        []string{},
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

	apiEndpoint := server.PublicIP + ":" + os.Getenv("Node_Port")

	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://"+apiEndpoint+"/connect?uuid="+uuid+"&email="+email+"&clientip="+clientIP, nil)
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

	// set the user as active
	userinfo.Active = true
	if err := db.DB.Model(&userinfo).Update("active", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update user active status: " + err.Error(),
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
