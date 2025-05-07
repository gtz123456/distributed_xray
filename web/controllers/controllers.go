package controllers

import (
	"go-distributed/registry"
	"go-distributed/web/db"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var expireMap = make(map[string]time.Time)

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
		PlanStart: time.Now(),
		PlanEnd:   time.Now().Add(30 * 24 * time.Hour),
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
	// user, _ := c.Get("user")

	// userinfo := user.(db.User)

	c.JSON(http.StatusOK, gin.H{
		// "UUID": userinfo.UUID,
		"UUID": "cbcb66f7-a1e2-4b6f-a1b3-5599dd95bb9c", // for testing purpose, remove this line in production!
	})
}

func Realitykey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"pubkey": os.Getenv("REALITY_PUBKEY"),
	})
}

type Server struct {
	IP          string   `json:"ip"`
	IPV6        string   `json:"ipv6"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	// TrafficMultiplier int      `json:"traffic_multiplier"` // 0 means free, and for better servers the value will be higher
}

func GetServers() []Server {
	// TODO: Fetch servers from registry and cache them
	regs, err := registry.GetProviders(registry.NodeService)

	if err != nil {
		return []Server{}
	}

	servers := []Server{}

	for _, reg := range regs {
		server := Server{
			IP:          reg.PublicIP,
			IPV6:        "",
			Description: reg.Description,
			Tags:        []string{},
		}

		servers = append(servers, server)
	}
	return servers
}

func Servers(c *gin.Context) {
	//
	servers := GetServers()
	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
	})
}

func Version(c *gin.Context) {
	// returns if the version of client is supported or not
	SupportedVersions := map[string]bool{
		"0.1.0": true,
	}
	var body struct {
		Version string `json:"version"`
	}
	if c.Bind(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read body",
		})
		return
	}

	if _, ok := SupportedVersions[body.Version]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported version",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{})

}
