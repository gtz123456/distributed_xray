package controllers

import (
	"encoding/json"
	"fmt"
	"go-distributed/registry"
	"go-distributed/utils"
	"go-distributed/web/db"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// NodeStatusItem is what the WebService returns for each node in the cluster.
type NodeStatusItem struct {
	// Identity
	ServiceID   string `json:"service_id"`
	PublicIP    string `json:"public_ip"`
	Description string `json:"description"`
	Tags        []string `json:"tags"`
	Online      bool   `json:"online"`

	// System metrics
	CPUPercent  float64 `json:"cpu_percent"`
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskPercent float64 `json:"disk_percent"`

	// Network speed (bytes/sec)
	BytesUpPerSec   int64 `json:"bytes_up_per_sec"`
	BytesDownPerSec int64 `json:"bytes_down_per_sec"`

	// Traffic
	TrafficUsedBytes  int64 `json:"traffic_used_bytes"`
	TrafficLimitBytes int64 `json:"traffic_limit_bytes"`

	// Users
	ConnectionCount int              `json:"connection_count"`
	Connections     []NodeConnItem   `json:"connections"`

	CollectedAt time.Time `json:"collected_at"`
	Error       string    `json:"error,omitempty"`
}

type NodeConnItem struct {
	UUID      string `json:"uuid"`
	Email     string `json:"email"`
	Port      int    `json:"port"`
	UpBytes   int    `json:"up_bytes"`
	DownBytes int    `json:"down_bytes"`
}

// nodeStatusRaw is the raw JSON from NodeService /status.
type nodeStatusRaw struct {
	CPUPercent        float64 `json:"cpu_percent"`
	MemTotal          uint64  `json:"mem_total"`
	MemUsed           uint64  `json:"mem_used"`
	MemPercent        float64 `json:"mem_percent"`
	DiskTotal         uint64  `json:"disk_total"`
	DiskUsed          uint64  `json:"disk_used"`
	DiskPercent       float64 `json:"disk_percent"`
	BytesUpPerSec     int64   `json:"bytes_up_per_sec"`
	BytesDownPerSec   int64   `json:"bytes_down_per_sec"`
	TrafficUsedBytes  int64   `json:"traffic_used_bytes"`
	TrafficLimitBytes int64   `json:"traffic_limit_bytes"`
	ConnectionCount   int     `json:"connection_count"`
	Connections       []struct {
		UUID      string `json:"uuid"`
		Port      int    `json:"port"`
		UpBytes   int    `json:"up_bytes"`
		DownBytes int    `json:"down_bytes"`
	} `json:"connections"`
	CollectedAt time.Time `json:"collected_at"`
}

func fetchNodeStatus(reg registry.Registration) NodeStatusItem {
	item := NodeStatusItem{
		ServiceID:   reg.ServiceID,
		PublicIP:    reg.PublicIP,
		Description: reg.Description,
		Tags:        reg.Tags,
	}

	nodePort := os.Getenv("Node_Port")
	if nodePort == "" {
		nodePort = "8002"
	}
	url := fmt.Sprintf("http://%s:%s/status", reg.PublicIP, nodePort)

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		item.Online = false
		item.Error = err.Error()
		return item
	}
	req.Header.Set("regkey", utils.Regkey())

	resp, err := client.Do(req)
	if err != nil {
		item.Online = false
		item.Error = "unreachable: " + err.Error()
		return item
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		item.Online = false
		item.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return item
	}

	var raw nodeStatusRaw
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		item.Online = false
		item.Error = "decode error: " + err.Error()
		return item
	}

	item.Online = true
	item.CPUPercent = raw.CPUPercent
	item.MemTotal = raw.MemTotal
	item.MemUsed = raw.MemUsed
	item.MemPercent = raw.MemPercent
	item.DiskTotal = raw.DiskTotal
	item.DiskUsed = raw.DiskUsed
	item.DiskPercent = raw.DiskPercent
	item.BytesUpPerSec = raw.BytesUpPerSec
	item.BytesDownPerSec = raw.BytesDownPerSec
	item.TrafficUsedBytes = raw.TrafficUsedBytes
	item.TrafficLimitBytes = raw.TrafficLimitBytes
	item.ConnectionCount = raw.ConnectionCount
	item.CollectedAt = raw.CollectedAt

	// Enrich connections with email from DB
	for _, c := range raw.Connections {
		conn := NodeConnItem{
			UUID:      c.UUID,
			Port:      c.Port,
			UpBytes:   c.UpBytes,
			DownBytes: c.DownBytes,
		}
		var u userEmailOnly
		if db.DB.Table("users").Select("email").Where("uuid = ?", c.UUID).Scan(&u).Error == nil {
			conn.Email = u.Email
		}
		item.Connections = append(item.Connections, conn)
	}

	return item
}

type userEmailOnly struct {
	Email string
}

// AdminCluster returns real-time status of every registered node.
func AdminCluster(c *gin.Context) {
	regs, err := registry.GetProviders(registry.NodeService)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch node list"})
		return
	}

	results := make([]NodeStatusItem, len(regs))
	var wg sync.WaitGroup
	for i, reg := range regs {
		wg.Add(1)
		go func(idx int, r registry.Registration) {
			defer wg.Done()
			results[idx] = fetchNodeStatus(r)
		}(i, reg)
	}
	wg.Wait()

	// Aggregate totals
	var totalConns int
	var totalUp, totalDown int64
	for _, r := range results {
		totalConns += r.ConnectionCount
		totalUp += r.BytesUpPerSec
		totalDown += r.BytesDownPerSec
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes":            results,
		"node_count":       len(results),
		"total_conns":      totalConns,
		"total_up_per_sec": totalUp,
		"total_down_per_sec": totalDown,
	})
}
