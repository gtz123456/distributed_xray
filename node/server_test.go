package node

import (
	"encoding/json"
	"go-distributed/utils"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRestoreProxies(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()

	utils.RedisClient = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	serverIP := "192.168.1.100"
	NodeIP = serverIP

	// Inject fake proxy configs to redis
	cfg1 := ProxyConfig{
		Port:         20000,
		ClientIP:     "10.0.0.1",
		RateLimitInt: 50,
		BurstInt:     100,
	}
	cfg1JSON, _ := json.Marshal(cfg1)
	utils.RedisClient.HSet(utils.Ctx, "node_ports:"+serverIP, "uuid-1", cfg1JSON)

	cfg2 := ProxyConfig{
		Port:         20001,
		ClientIP:     "10.0.0.2",
		RateLimitInt: 100,
		BurstInt:     200,
	}
	cfg2JSON, _ := json.Marshal(cfg2)
	utils.RedisClient.HSet(utils.Ctx, "node_ports:"+serverIP, "uuid-2", cfg2JSON)

	// Call restore
	RestoreProxies(serverIP)

	// Sleep slightly to let goroutines spawn
	time.Sleep(10 * time.Millisecond)

	connectionsLock.Lock()
	defer connectionsLock.Unlock()

	// Verify connections are restored
	assert.Equal(t, 20000, connections["uuid-1"])
	assert.Equal(t, 20001, connections["uuid-2"])

	// Verify proxyServices are populated
	assert.NotNil(t, proxyServices["uuid-1"])
	assert.NotNil(t, proxyServices["uuid-2"])

	// Cancel them to not leak goroutines in tests
	proxyServices["uuid-1"].cancelFunc()
	proxyServices["uuid-2"].cancelFunc()
}
