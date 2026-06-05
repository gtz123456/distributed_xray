package controllers

import (
	"encoding/json"
	"go-distributed/registry"
	"go-distributed/utils"
	"go-distributed/web/db"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ─── User Management ──────────────────────────────────────────────────────────

func ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	planFilter := c.Query("plan")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := db.DB.Model(&db.User{})
	if planFilter != "" {
		query = query.Where("plan = ?", planFilter)
	}

	var total int64
	query.Count(&total)

	var users []db.User
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	type UserSummary struct {
		ID           uint      `json:"id"`
		Email        string    `json:"email"`
		UUID         string    `json:"uuid"`
		Plan         string    `json:"plan"`
		PlanEnd      time.Time `json:"plan_end"`
		TrafficUsed  int       `json:"traffic_used"`
		TrafficLimit int       `json:"traffic_limit"`
		Balance      int       `json:"balance"`
		IsVerified   bool      `json:"is_verified"`
		CreatedAt    time.Time `json:"created_at"`
	}

	summaries := make([]UserSummary, len(users))
	for i, u := range users {
		summaries[i] = UserSummary{
			ID:           u.ID,
			Email:        u.Email,
			UUID:         u.UUID,
			Plan:         u.Plan,
			PlanEnd:      u.PlanEnd,
			TrafficUsed:  u.TrafficUsed,
			TrafficLimit: u.TrafficLimit,
			Balance:      u.Balance,
			IsVerified:   u.IsVerified,
			CreatedAt:    u.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"users":     summaries,
	})
}

func GetUser(c *gin.Context) {
	email := c.Param("email")
	var user db.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":            user.ID,
		"email":         user.Email,
		"uuid":          user.UUID,
		"plan":          user.Plan,
		"plan_end":      user.PlanEnd,
		"next_renew":    user.NextRenew,
		"renew_cycle":   user.RenewCycle,
		"traffic_used":  user.TrafficUsed,
		"traffic_limit": user.TrafficLimit,
		"balance":       user.Balance,
		"is_verified":   user.IsVerified,
		"created_at":    user.CreatedAt,
	})
}

// AdminSetPlan replaces the broken SetPlan: uuid from path, plan+duration from body.
func AdminSetPlan(c *gin.Context) {
	uuid := c.Param("uuid")
	var req struct {
		Plan           string `json:"plan"`
		Action         string `json:"action"` // "add" or "set"
		DeltaAmount    int    `json:"delta_amount"`
		DeltaUnit      string `json:"delta_unit"`      // "days", "months", "years"
		EndDate        string `json:"end_date"`        // "YYYY-MM-DD"
		DurationMonths int    `json:"duration_months"` // Fallback for old clients
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var user db.User
	if err := db.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var newEnd time.Time

	// Legacy fallback
	if req.Action == "" {
		req.Action = "add"
		req.DeltaAmount = req.DurationMonths
		req.DeltaUnit = "months"
		if req.DeltaAmount <= 0 {
			req.DeltaAmount = 1
		}
	}

	if req.Action == "set" {
		parsedEnd, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
			return
		}
		newEnd = parsedEnd
	} else if req.Action == "add" {
		baseTime := user.PlanEnd
		if baseTime.Before(time.Now()) {
			baseTime = time.Now()
		}

		switch req.DeltaUnit {
		case "days":
			newEnd = baseTime.AddDate(0, 0, req.DeltaAmount)
		case "years":
			newEnd = baseTime.AddDate(req.DeltaAmount, 0, 0)
		case "months":
			fallthrough
		default:
			newEnd = baseTime.AddDate(0, req.DeltaAmount, 0)
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action. Must be 'add' or 'set'"})
		return
	}

	switch req.Plan {
	case "Free plan":
		user.Plan = "Free plan"
		user.TrafficUsed = 0
		user.TrafficLimit = 10 * 1000 * 1000 * 1000 // 10 GB
		user.PlanEnd = newEnd
		user.NextRenew = time.Now().AddDate(0, 0, 31)
	case "Premium plan":
		user.Plan = "Premium plan"
		user.TrafficUsed = 0
		user.TrafficLimit = 200 * 1000 * 1000 * 1000 // 200 GB
		user.PlanEnd = newEnd
		user.NextRenew = time.Now().AddDate(0, 0, 31)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan, must be 'Free plan' or 'Premium plan'"})
		return
	}

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user plan"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plan updated successfully", "user_uuid": uuid, "plan": user.Plan, "plan_end": user.PlanEnd})
}

func AdminResetTraffic(c *gin.Context) {
	uuid := c.Param("uuid")
	var user db.User
	if err := db.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if err := db.DB.Model(&user).Update("traffic_used", 0).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset traffic"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Traffic reset successfully"})
}

func AdminAddBalance(c *gin.Context) {
	uuid := c.Param("uuid")
	var req struct {
		Amount int    `json:"amount"` // in cents, can be negative
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var user db.User
	if err := db.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.Balance += req.Amount
	if user.Balance < 0 {
		user.Balance = 0
	}
	if err := db.DB.Model(&user).Update("balance", user.Balance).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":     "Balance updated",
		"new_balance": user.Balance,
	})
}

func AdminBanUser(c *gin.Context) {
	uuid := c.Param("uuid")
	var user db.User
	if err := db.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Set PlanEnd to past so PlanMonitor kicks them off
	user.PlanEnd = time.Now().Add(-1 * time.Second)
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ban user"})
		return
	}

	// Also immediately clean up Redis connections
	conns, err := utils.RedisClient.HGetAll(utils.Ctx, "user_conns:"+uuid).Result()
	if err == nil && len(conns) > 0 {
		utils.RedisClient.Del(utils.Ctx, "user_conns:"+uuid)
		utils.RedisClient.SRem(utils.Ctx, "active_users", uuid)
		for _, connJSON := range conns {
			var conn UserConnection
			json.Unmarshal([]byte(connJSON), &conn)
			// The new declarative way: Just remove from node_ports hash
			utils.RedisClient.HDel(utils.Ctx, "node_ports:"+conn.NodeIP, uuid)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "User banned and disconnected"})
}

func AdminUnbanUser(c *gin.Context) {
	uuid := c.Param("uuid")
	var user db.User
	if err := db.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.Plan = "Free plan"
	user.PlanEnd = time.Now().AddDate(0, 1, 0)
	user.TrafficLimit = 10 * 1000 * 1000 * 1000
	user.TrafficUsed = 0
	user.NextRenew = time.Now().AddDate(0, 0, 31)
	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unban user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User unbanned, restored to Free plan"})
}

func AdminDeleteUser(c *gin.Context) {
	uuid := c.Param("uuid")
	var user db.User
	if err := db.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Clean up Redis
	utils.RedisClient.Del(utils.Ctx, "user_conns:"+uuid)
	utils.RedisClient.SRem(utils.Ctx, "active_users", uuid)

	if err := db.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// ─── Connections & Nodes ──────────────────────────────────────────────────────

func AdminListConnections(c *gin.Context) {
	activeUsers, err := utils.RedisClient.SMembers(utils.Ctx, "active_users").Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch active users"})
		return
	}

	type ConnectionInfo struct {
		UUID          string    `json:"uuid"`
		Email         string    `json:"email"`
		NodeIP        string    `json:"node_ip"`
		NodePort      string    `json:"node_port"`
		LastHeartbeat time.Time `json:"last_heartbeat"`
	}

	var connections []ConnectionInfo

	for _, uuid := range activeUsers {
		conns, err := utils.RedisClient.HGetAll(utils.Ctx, "user_conns:"+uuid).Result()
		if err != nil || len(conns) == 0 {
			continue
		}

		// Fetch email from DB
		var user db.User
		email := uuid // fallback
		if db.DB.Select("email").Where("uuid = ?", uuid).First(&user).Error == nil {
			email = user.Email
		}

		for _, connJSON := range conns {
			var conn UserConnection
			json.Unmarshal([]byte(connJSON), &conn)
			connections = append(connections, ConnectionInfo{
				UUID:          uuid,
				Email:         email,
				NodeIP:        conn.NodeIP,
				NodePort:      conn.NodePort,
				LastHeartbeat: conn.LastHeartBeat,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"online_count": len(connections),
		"connections":  connections,
	})
}

func AdminDisconnectUser(c *gin.Context) {
	uuid := c.Param("uuid")

	conns, err := utils.RedisClient.HGetAll(utils.Ctx, "user_conns:"+uuid).Result()
	if err != nil || len(conns) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User has no active connections"})
		return
	}

	utils.RedisClient.Del(utils.Ctx, "user_conns:"+uuid)
	utils.RedisClient.SRem(utils.Ctx, "active_users", uuid)

	for _, connJSON := range conns {
		var conn UserConnection
		json.Unmarshal([]byte(connJSON), &conn)
		// The new declarative way: Just remove from node_ports hash
		utils.RedisClient.HDel(utils.Ctx, "node_ports:"+conn.NodeIP, uuid)
	}

	c.JSON(http.StatusOK, gin.H{"message": "User disconnected"})
}

func AdminListNodes(c *gin.Context) {
	regs, err := registry.GetProviders(registry.NodeService)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nodes"})
		return
	}

	type NodeInfo struct {
		ServiceID   string   `json:"service_id"`
		PublicIP    string   `json:"public_ip"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		ServiceURL  string   `json:"service_url"`
	}

	nodes := make([]NodeInfo, len(regs))
	for i, reg := range regs {
		nodes[i] = NodeInfo{
			ServiceID:   reg.ServiceID,
			PublicIP:    reg.PublicIP,
			Description: reg.Description,
			Tags:        reg.Tags,
			ServiceURL:  reg.ServiceURL,
		}
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

// ─── Voucher Management ───────────────────────────────────────────────────────

func AdminListVouchers(c *gin.Context) {
	usedFilter := c.Query("used") // "true", "false", or "" (all)

	query := db.DB.Model(&db.Voucher{})
	if usedFilter == "true" {
		query = query.Where("is_used = ?", true)
	} else if usedFilter == "false" {
		query = query.Where("is_used = ?", false)
	}

	var vouchers []db.Voucher
	if err := query.Order("created_at DESC").Find(&vouchers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch vouchers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"vouchers": vouchers})
}

func AdminRevokeVoucher(c *gin.Context) {
	code := c.Param("code")
	var voucher db.Voucher
	if err := db.DB.Where("code = ?", code).First(&voucher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Voucher not found"})
		return
	}
	past := time.Now().Add(-1 * time.Second)
	voucher.IsUsed = true
	voucher.ExpiresAt = past
	if err := db.DB.Save(&voucher).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke voucher"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Voucher revoked"})
}

// ─── Stats ────────────────────────────────────────────────────────────────────

func AdminStats(c *gin.Context) {
	var totalUsers int64
	db.DB.Model(&db.User{}).Count(&totalUsers)

	var verifiedUsers int64
	db.DB.Model(&db.User{}).Where("is_verified = ?", true).Count(&verifiedUsers)

	var premiumUsers int64
	db.DB.Model(&db.User{}).Where("plan = ?", "Premium plan").Count(&premiumUsers)

	onlineUsers, _ := utils.RedisClient.SCard(utils.Ctx, "active_users").Result()

	var totalRevenue struct{ Total int }
	db.DB.Model(&db.Payment{}).Where("status = ?", "paid").Select("COALESCE(SUM(amount), 0) as total").Scan(&totalRevenue)

	var trafficStats struct{ Total int64 }
	db.DB.Model(&db.User{}).Select("COALESCE(SUM(traffic_used), 0) as total").Scan(&trafficStats)

	c.JSON(http.StatusOK, gin.H{
		"total_users":              totalUsers,
		"verified_users":           verifiedUsers,
		"premium_users":            premiumUsers,
		"online_users":             onlineUsers,
		"total_revenue_cents":      totalRevenue.Total,
		"total_traffic_used_bytes": trafficStats.Total,
	})
}

// ─── Shell Proxy ──────────────────────────────────────────────────────────────

var adminShellUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func AdminNodeShell(c *gin.Context) {
	serviceID := c.Param("serviceid")

	regs, err := registry.GetProviders(registry.NodeService)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nodes"})
		return
	}

	var targetNode *registry.Registration
	for _, r := range regs {
		if r.ServiceID == serviceID {
			targetNode = &r
			break
		}
	}

	if targetNode == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	// Upgrade client connection
	clientWS, err := adminShellUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer clientWS.Close()

	// Dial the node's websocket endpoint
	parsedURL, err := url.Parse(targetNode.ServiceURL)
	if err != nil {
		clientWS.WriteMessage(websocket.TextMessage, []byte("Invalid node URL"))
		return
	}

	wsURL := "ws://" + parsedURL.Host + "/shell?regkey=" + utils.Regkey()
	nodeWS, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		clientWS.WriteMessage(websocket.TextMessage, []byte("Failed to connect to node shell: "+err.Error()))
		return
	}
	defer nodeWS.Close()

	errc := make(chan error, 2)

	// Copy client -> node
	go func() {
		for {
			mt, p, err := clientWS.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := nodeWS.WriteMessage(mt, p); err != nil {
				errc <- err
				return
			}
		}
	}()

	// Copy node -> client
	go func() {
		for {
			mt, p, err := nodeWS.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := clientWS.WriteMessage(mt, p); err != nil {
				errc <- err
				return
			}
		}
	}()

	<-errc
}
