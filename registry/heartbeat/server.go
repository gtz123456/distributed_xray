package heartbeat

import (
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type HeartBeatHandler interface {
	HandleHeartbeat(w http.ResponseWriter, r *http.Request)
}

type HeartBeatServer struct {
	HeartBeatTypeMap map[string]HeartBeatHandler
	LastHeartBeat    map[string]time.Time
	Validator        ServiceValidator
	Mutex            *sync.RWMutex
}

func NewHeartBeatServer() *HeartBeatServer {
	HeartBeatServer := &HeartBeatServer{
		HeartBeatTypeMap: nil,
		LastHeartBeat:    make(map[string]time.Time),
		Mutex:            &sync.RWMutex{},
	}
	HeartBeatTypeMap := make(map[string]HeartBeatHandler)
	HeartBeatTypeMap["/heartbeat/basic"] = &BasicHeartbeatHandler{BaseHeartBeatHandler{Server: HeartBeatServer}}
	// HeartBeatTypeMap["/heartbeat/info"] = &ServerInfoHeartbeatHandler{BaseHeartBeatHandler{server: HeartBeatServer}}
	HeartBeatServer.HeartBeatTypeMap = HeartBeatTypeMap
	return HeartBeatServer
}

type BaseHeartBeatHandler struct {
	Server *HeartBeatServer
}

func (b *BaseHeartBeatHandler) HandleCommonLogic(w http.ResponseWriter, r *http.Request) {
	ServiceID, err := io.ReadAll(r.Body)
	if err != nil || len(ServiceID) == 0 {
		http.Error(w, "Invalid ServiceID", http.StatusBadRequest)
		log.Println("Invalid ServiceID:", ServiceID)
		return
	}
	ServiceIDStr := string(ServiceID)

	if b.Server.Validator == nil {
		log.Println("No validator set for the heartbeat server")
		http.Error(w, "No validator set for the heartbeat server", http.StatusInternalServerError)
		return
	}

	if !b.Server.Validator.IsServiceRegistered(ServiceIDStr) {
		http.Error(w, "Service not authorized", http.StatusUnauthorized)
		return
	}

	b.Server.Mutex.Lock()
	b.Server.LastHeartBeat[ServiceIDStr] = time.Now()
	b.Server.Mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

type BasicHeartbeatHandler struct {
	BaseHeartBeatHandler
}

func (h *BasicHeartbeatHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	h.HandleCommonLogic(w, r)
}

/*
type ServerInfoHeartbeatHandler struct {
	BaseHeartBeatHandler
}

func (h *ServerInfoHeartbeatHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var info ServerInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	srcAddr := r.RemoteAddr
	h.server.mutex.Lock()
	defer h.server.mutex.Unlock()

	h.server.LastHeartBeat[srcAddr] = time.Now()

	log.Printf("Received server info from %s: CPU Usage=%s, Memory Usage=%s", srcAddr, info.CPUUsage, info.MemoryUsage)

	h.HandleCommonLogic(w, r)
} */

func (h *HeartBeatServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Mutex.Lock()
	handler := h.HeartBeatTypeMap[r.URL.Path]
	h.Mutex.Unlock()

	if handler == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	handler.HandleHeartbeat(w, r)

}
