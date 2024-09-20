package heartbeat

import "log"

type HeartbeatStrategy interface {
	SendHeartbeat() error
}

type InfoHeartbeat struct {
	url string
}

type Heartbeat struct {
	Strategy HeartbeatStrategy
}

func (h *Heartbeat) Send() {
	err := h.Strategy.SendHeartbeat()
	if err != nil {
		log.Println(err)
	}
}

// ServerInfo 代表服务器状态信息
type ServerInfo struct {
	CPUUsage    string `json:"cpu_usage"`
	MemoryUsage string `json:"memory_usage"`
}
