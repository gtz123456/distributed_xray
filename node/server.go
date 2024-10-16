package node

import (
	"fmt"
	"go-distributed/registry"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type nodeHandler struct{}

func RegisterHandlers() {
	handler := new(nodeHandler)
	http.Handle("/info", handler)
	http.Handle("/limit", handler)
}

func (sh *nodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		switch r.URL.Path {
		case "/info":
			sh.handleInfo(w, r)

		case "/limit":
			sh.handleLimit(w, r)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (sh *nodeHandler) handleInfo(w http.ResponseWriter, r *http.Request) {
	msg, err := io.ReadAll(r.Body)

	if err != nil || len(msg) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cpuUsage, err := cpu.Percent(time.Second, false)
	memInfo, err := mem.VirtualMemory()

	memTotal := memInfo.Total
	memUsed := memInfo.Used
	memUsedPercent := memInfo.UsedPercent

	output := []byte(fmt.Sprintf("CPU Usage: %v\nMemory Total: %v\nMemory Used: %v\nMemory Used Percent: %v\n", cpuUsage[0], memTotal, memUsed, memUsedPercent))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(output)
}

func (sh *nodeHandler) handleLimit(w http.ResponseWriter, r *http.Request) {
	// only accept request from user service
	userServiceAddrs, err := registry.GetProviders(registry.UserService)

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
	for _, addr := range userServiceAddrs {
		if addr == srcIP {
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
	return

}
