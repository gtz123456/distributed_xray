package node

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// NodeStatus is the complete real-time status snapshot of a node.
type NodeStatus struct {
	// System
	CPUPercent  float64 `json:"cpu_percent"`
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskPercent float64 `json:"disk_percent"`

	// Network (rolling 5-second window)
	BytesUpPerSec   int64 `json:"bytes_up_per_sec"`
	BytesDownPerSec int64 `json:"bytes_down_per_sec"`

	// Monthly traffic
	TrafficUsedBytes  int64 `json:"traffic_used_bytes"`
	TrafficLimitBytes int64 `json:"traffic_limit_bytes"`

	// Connections
	ConnectionCount int              `json:"connection_count"`
	Connections     []ConnStatusItem `json:"connections"`

	CollectedAt time.Time `json:"collected_at"`
}

// ConnStatusItem is per-user connection info returned in /status.
type ConnStatusItem struct {
	UUID      string `json:"uuid"`
	Port      int    `json:"port"`
	UpBytes   int    `json:"up_bytes"`
	DownBytes int    `json:"down_bytes"`
}

// speedSnapshot records the last bandwidth sampling point.
var speedSnapshot struct {
	at   time.Time
	up   int
	down int
}

func (sh *nodeHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	// --- CPU ---
	cpuPct, _ := cpu.Percent(200*time.Millisecond, false)
	var cpuPercent float64
	if len(cpuPct) > 0 {
		cpuPercent = cpuPct[0]
	}

	// --- Memory ---
	memInfo, _ := mem.VirtualMemory()

	// --- Disk ---
	diskInfo, _ := disk.Usage("/")

	// --- Monthly traffic ---
	trafficLimitGB := getEnvInt("TRAFFIC_LIMIT_GB", 10)
	trafficLimitBytes := trafficLimitGB * 1024 * 1024 * 1024
	curTraffic, _ := getCurrentTrafficBytes()
	startTraffic := readFileInt(usageFile)
	var usedBytes int64
	if startTraffic > 0 && curTraffic >= startTraffic {
		usedBytes = curTraffic - startTraffic
	}

	// --- Per-user connections + bandwidth ---
	connectionsLock.Lock()
	connsCopy := make(map[string]int, len(connections))
	for uuid, port := range connections {
		connsCopy[uuid] = port
	}
	connectionsLock.Unlock()

	var totalUp, totalDown int
	items := make([]ConnStatusItem, 0, len(connsCopy))
	for uuid, port := range connsCopy {
		item := ConnStatusItem{UUID: uuid, Port: port}
		if val, ok := statsStore.Load(port); ok {
			s := val.(*ConnStats)
			item.UpBytes = s.Uploaded
			item.DownBytes = s.Downloaded
			totalUp += s.Uploaded
			totalDown += s.Downloaded
		}
		items = append(items, item)
	}

	// --- Speed (bytes/sec since last call) ---
	now := time.Now()
	var upPerSec, downPerSec int64
	if !speedSnapshot.at.IsZero() {
		elapsed := now.Sub(speedSnapshot.at).Seconds()
		if elapsed > 0 {
			upPerSec = int64(float64(totalUp-speedSnapshot.up) / elapsed)
			downPerSec = int64(float64(totalDown-speedSnapshot.down) / elapsed)
		}
	}
	speedSnapshot.at = now
	speedSnapshot.up = totalUp
	speedSnapshot.down = totalDown

	if upPerSec < 0 {
		upPerSec = 0
	}
	if downPerSec < 0 {
		downPerSec = 0
	}

	status := NodeStatus{
		CPUPercent:        cpuPercent,
		MemTotal:          memInfo.Total,
		MemUsed:           memInfo.Used,
		MemPercent:        memInfo.UsedPercent,
		DiskTotal:         diskInfo.Total,
		DiskUsed:          diskInfo.Used,
		DiskPercent:       diskInfo.UsedPercent,
		BytesUpPerSec:     upPerSec,
		BytesDownPerSec:   downPerSec,
		TrafficUsedBytes:  usedBytes,
		TrafficLimitBytes: trafficLimitBytes,
		ConnectionCount:   len(items),
		Connections:       items,
		CollectedAt:       now,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
