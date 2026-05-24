package registry

import (
	"go-distributed/registry/heartbeat"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRemoveInactiveServices(t *testing.T) {
	// Initialize registry components
	hbServer := heartbeat.NewHeartBeatServer()
	NewRegistryService(hbServer)

	// Add a dummy service
	reg.add(Registration{
		ServiceName: "TestService",
		ServiceURL:  "http://localhost:8080",
		ServiceID:   "test-uuid",
	})

	assert.True(t, reg.IsServiceRegistered("test-uuid"))
	assert.Equal(t, 1, len(reg.registrationsMap["TestService"]))

	// Set last heartbeat to 25 seconds ago
	hbServer.Mutex.Lock()
	hbServer.LastHeartBeat["test-uuid"] = time.Now().Add(-25 * time.Second)
	hbServer.Mutex.Unlock()

	// Manually trigger garbage collection
	removeInactiveServices()

	// Should be removed
	assert.Equal(t, 0, len(reg.registrationsMap["TestService"]))
}
