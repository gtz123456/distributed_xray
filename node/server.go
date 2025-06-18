package node

import (
	"context"
	"encoding/json"
	"go-distributed/registry"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type nodeHandler struct{}

type User struct {
	Uuid  string `json:"uuid"`
	Email string `json:"email"`
}

type ProxyService struct {
	cancelFunc context.CancelFunc
}

var (
	xrayCtl *XrayController
	cfg     = &BaseConfig{
		APIAddress: "127.0.0.1",
		APIPort:    8080,
	}
	connections     = make(map[string]int) // uuid: port
	proxyServices   = make(map[string]*ProxyService)
	connectionsLock sync.Mutex
)

func RegisterHandlers() {
	handler := new(nodeHandler)
	http.Handle("/info", handler)
	http.Handle("/limit", handler)
	http.Handle("/connect", handler)
	http.Handle("/disconnect", handler)
}

func (sh *nodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		switch r.URL.Path {
		case "/info":
			sh.handleInfo(w, r)

		case "/limit":
			sh.handleLimit(w, r)

		case "/connect":
			sh.handleConnect(w, r)

		case "/disconnect":
			sh.handleDisconnect(w, r)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (sh *nodeHandler) handleInfo(w http.ResponseWriter, r *http.Request) {
	// TODO:  return in json format
	msg, err := io.ReadAll(r.Body)

	if err != nil || len(msg) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cpuUsage, err := cpu.Percent(time.Second, false)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	info := map[string]interface{}{
		"cpu_usage":           cpuUsage[0],
		"memory_total":        memInfo.Total,
		"memory_used":         memInfo.Used,
		"memory_used_percent": memInfo.UsedPercent,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (sh *nodeHandler) handleLimit(w http.ResponseWriter, r *http.Request) {
	// only accept request from web service
	providers, err := registry.GetProviders(registry.WebService)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	srcAddr := r.RemoteAddr
	srcIP, _, err := net.SplitHostPort(srcAddr)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	allowed := false
	for _, prov := range providers {
		if prov.PublicIP == srcIP {
			allowed = true
			break
		}
	}

	if !allowed {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// get rate and burst from request
	rate, err := strconv.Atoi(r.Header.Get("Rate"))
	burst, err := strconv.Atoi(r.Header.Get("Burst"))

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	limiter := Limiter(srcAddr, rate, burst)
	limiters[srcAddr] = limiter

	w.WriteHeader(http.StatusOK)
}

func (sh *nodeHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	// only accept request from web service
	providers, err := registry.GetProviders(registry.WebService)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	srcAddr := r.RemoteAddr
	srcIP, _, err := net.SplitHostPort(srcAddr)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	allowed := false
	for _, prov := range providers {
		if prov.PublicIP == srcIP {
			allowed = true
			break
		}
	}

	if !allowed {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	uuid := r.URL.Query().Get("uuid")
	email := r.URL.Query().Get("email")
	clientip := r.URL.Query().Get("clientip")

	if uuid == "" || email == "" || clientip == "" {
		log.Println("Missing required headers: uuid, email, or clientip", uuid, email, clientip)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("Received connection request from UUID: %s, Email: %s, Client IP: %s", uuid, email, clientip)

	log.Printf("Current connections: %v", connections)
	if port, ok := connections[uuid]; ok {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"port": strconv.Itoa(port),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}

	xrayCtl = new(XrayController)
	err = xrayCtl.Init(cfg)

	defer xrayCtl.CmdConn.Close()
	if err != nil {
		log.Printf("Failed to initialize Xray controller: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	userInfo := &UserInfo{
		Uuid:  uuid,
		Level: 0,
		InTag: "test",
		Email: email,
	}

	err = addVlessUser(xrayCtl.HsClient, userInfo) // TODO: user might already exists, so we should check if user exists before adding
	if err != nil {
		removeVlessUser(xrayCtl.HsClient, userInfo)
		err = addVlessUser(xrayCtl.HsClient, userInfo) // try to add again
		if err != nil {
			log.Printf("Failed to add user: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		log.Printf("User %s added successfully", userInfo.Email)
	}

	var port int
	for {
		port = 10000 + rand.Intn(50000) // random port between 10000-60000
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			ln.Close()
			break // port is available
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go NewProxy(ctx, port)

	connectionsLock.Lock()
	connections[uuid] = port
	proxyServices[uuid] = &ProxyService{
		cancelFunc: cancel,
	}
	connectionsLock.Unlock()

	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"port": strconv.Itoa(port),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (sh *nodeHandler) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	log.Printf("Received disconnect request for UUID: %s", uuid)
	if uuid == "" {
		log.Println("Missing required header: uuid")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	connectionsLock.Lock()

	delete(connections, uuid)

	if svc, ok := proxyServices[uuid]; ok {
		svc.cancelFunc() // stop the proxy
		delete(proxyServices, uuid)
	}

	connectionsLock.Unlock()

	w.WriteHeader(http.StatusOK)
}
