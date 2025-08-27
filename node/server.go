package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-distributed/registry"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
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
	statsStore      = &StatsStore{}
	statsCache      = &StatsStore{}
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

		case "/connect":
			sh.handleConnect(w, r)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	case http.MethodPost:
		switch r.URL.Path {
		case "/disconnect":
			sh.handleDisconnect(w, r)
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
	rateLimit := r.URL.Query().Get("rate")
	burst := r.URL.Query().Get("burst")

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

	rateLimitInt := 0
	if rateLimit != "" {
		var err error
		rateLimitInt, err = strconv.Atoi(rateLimit)
		if err != nil {
			rateLimitInt = defaultlimit.Rate
		}
	}

	burstInt := 0
	if burst != "" {
		var err error
		burstInt, err = strconv.Atoi(burst)
		if err != nil {
			burstInt = defaultlimit.Burst
		}
	}

	go NewProxy(ctx, port, clientip, rateLimitInt, burstInt, statsStore) // start proxy service

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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var uuids []string
	if err := json.Unmarshal(body, &uuids); err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(uuids) == 0 {
		log.Println("Received an empty list of UUIDs")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("Received disconnect request for %d UUIDs", len(uuids))

	connectionsLock.Lock()
	defer connectionsLock.Unlock()

	for _, uuid := range uuids {
		log.Printf("Processing disconnect for UUID: %s", uuid)

		port := connections[uuid]

		statsStore.Delete(port) // remove stats for this port

		delete(connections, uuid)

		if svc, ok := proxyServices[uuid]; ok {
			svc.cancelFunc()
			delete(proxyServices, uuid)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func StartTrafficReport() {
	go func() {
		for {
			time.Sleep(5 * time.Second)

			connectionsLock.Lock()
			connectionsSnapshot := make(map[string]int)
			for uuid, port := range connections {
				connectionsSnapshot[uuid] = port
			}
			connectionsLock.Unlock()

			report := make([]map[string]interface{}, 0, len(connectionsSnapshot))

			for uuid, port := range connectionsSnapshot {
				val, ok := statsStore.Load(port)
				if !ok {
					continue
				}
				stats := val.(*ConnStats)

				val, ok = statsCache.Load(port)
				var oldStats ConnStats
				if ok {
					oldStats = *(val.(*ConnStats))
				} else {
					oldStats = ConnStats{}
				}

				diff := (stats.Downloaded + stats.Uploaded) - (oldStats.Downloaded + oldStats.Uploaded)

				statsCopy := *stats
				statsCache.Store(port, &statsCopy)

				report = append(report, map[string]interface{}{
					"uuid":    uuid,
					"traffic": diff,
				})
			}

			if len(report) > 0 {
				data, err := json.Marshal(report)
				if err != nil {
					log.Println("Marshal traffic report error:", err)
					continue
				}

				providers, err := registry.GetProviders(registry.WebService)

				if err != nil {
					log.Println("GetProviders error:", err)
					continue
				}

				webPort := os.Getenv("Web_Port")
				provider := providers[0] // TODO

				go func(provider registry.Registration) {
					url := fmt.Sprintf("http://%s:%s/traffic", provider.PublicIP, webPort)
					req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
					if err != nil {
						log.Println("Create request error:", err)
						return
					}
					req.Header.Set("Content-Type", "application/json")

					client := &http.Client{}
					resp, err := client.Do(req)
					if err != nil {
						log.Println("Send request error:", err)
						return
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						log.Println("Send request failed:", resp.Status)
					}
				}(provider)
			}
		}
	}()
}
