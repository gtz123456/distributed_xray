package controllers

import (
	"encoding/json"
	"go-distributed/registry"
	"go-distributed/utils"
	"log"
	"time"
)

// HTTP disconnect request removed in favor of Redis declarative state
func StartHeartbeatMonitor() {
	go func() {
		log.Println("Starting heartbeat monitor...")

		for {
			time.Sleep(HEARTBEAT_CHECK_INTERVAL)

			// remove disconnected nodes from userConnectionMap
			regs, err := registry.GetProviders(registry.NodeService)

			if err != nil {
				log.Printf("Error fetching node services: %v", err)
			}

			users, err := utils.RedisClient.SMembers(utils.Ctx, "active_users").Result()
			if err != nil {
				log.Printf("Error fetching active users from redis: %v", err)
				continue
			}

			now := time.Now()

			for _, userUUID := range users {
				conns, err := utils.RedisClient.HGetAll(utils.Ctx, "user_conns:"+userUUID).Result()
				if err != nil {
					continue
				}

				validCount := 0
				for serviceID, connJSON := range conns {
					var conn UserConnection
					json.Unmarshal([]byte(connJSON), &conn)

					// check if node is still available
					found := false
					for _, reg := range regs {
						if conn.ServiceID == reg.ServiceID && conn.NodeIP == reg.PublicIP {
							found = true
							break
						}
					}

					if !found {
						log.Printf("Removing connection for user %s to node %s as it is no longer available.", userUUID, conn.NodeIP)
						utils.RedisClient.HDel(utils.Ctx, "user_conns:"+userUUID, serviceID)
						// Also proactively delete the entire node_ports key for this dead node
						utils.RedisClient.Del(utils.Ctx, "node_ports:"+conn.NodeIP)
						continue
					}

					if now.Sub(conn.LastHeartBeat) <= HEARTBEAT_TIMEOUT {
						validCount++
					} else {
						log.Printf("Connection for user %s to node %s timed out. Removing from redis.", userUUID, conn.NodeIP)
						utils.RedisClient.HDel(utils.Ctx, "user_conns:"+userUUID, serviceID)
						// The new declarative way: Just remove from node_ports hash
						utils.RedisClient.HDel(utils.Ctx, "node_ports:"+conn.NodeIP, userUUID)
					}
				}

				if validCount == 0 {
					utils.RedisClient.SRem(utils.Ctx, "active_users", userUUID)
					log.Printf("Removed user %s from active users as they have no valid connections left.", userUUID)
				}
			}
		}
	}()
}
