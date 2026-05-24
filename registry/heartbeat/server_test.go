package heartbeat

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockValidator struct {
	valid map[string]bool
	mu    sync.RWMutex
}

func (m *MockValidator) IsServiceRegistered(serviceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.valid[serviceID]
}

func TestHeartBeatServer_Concurrent(t *testing.T) {
	server := NewHeartBeatServer()
	
	validator := &MockValidator{
		valid: make(map[string]bool),
	}
	server.Validator = validator

	// Register 100 fake services
	numServices := 100
	for i := 0; i < numServices; i++ {
		serviceID := "service-" + strconv.Itoa(i)
		validator.valid[serviceID] = true
	}

	var wg sync.WaitGroup

	// Fire concurrent heartbeats
	for i := 0; i < numServices; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			serviceID := "service-" + strconv.Itoa(idx)

			req := httptest.NewRequest(http.MethodPost, "/heartbeat/basic", bytes.NewBufferString(serviceID))
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		}(i)
	}

	wg.Wait()

	server.Mutex.RLock()
	assert.Equal(t, numServices, len(server.LastHeartBeat))
	server.Mutex.RUnlock()
}

func TestHeartBeatServer_Unauthorized(t *testing.T) {
	server := NewHeartBeatServer()
	
	validator := &MockValidator{
		valid: make(map[string]bool),
	}
	server.Validator = validator

	req := httptest.NewRequest(http.MethodPost, "/heartbeat/basic", bytes.NewBufferString("unknown-service"))
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	server.Mutex.RLock()
	assert.Equal(t, 0, len(server.LastHeartBeat))
	server.Mutex.RUnlock()
}

func TestHeartBeatServer_NotFound(t *testing.T) {
	server := NewHeartBeatServer()
	
	req := httptest.NewRequest(http.MethodPost, "/invalid/path", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
