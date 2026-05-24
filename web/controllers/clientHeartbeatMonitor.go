package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-distributed/registry"
	"go-distributed/utils"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

func sendDisconnectRequest(apiURL string, uuids []string) error {
	jsonData, err := json.Marshal(uuids)
	if err != nil {
		return fmt.Errorf("failed to marshal uuids to json: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response status: %s", resp.Status)
	}

	fmt.Printf("Successfully sent disconnect request for %d UUIDs\n", len(uuids))
	return nil
}

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

			timedOutMap := make(map[string][]string)
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
						continue
					}

					if now.Sub(conn.LastHeartBeat) <= HEARTBEAT_TIMEOUT {
						validCount++
					} else {
						disconnectURL := "http://" + conn.NodeIP + ":" + os.Getenv("Node_Port") + "/disconnect"
						timedOutMap[disconnectURL] = append(timedOutMap[disconnectURL], userUUID)
						utils.RedisClient.HDel(utils.Ctx, "user_conns:"+userUUID, serviceID)
					}
				}

				if validCount == 0 {
					utils.RedisClient.SRem(utils.Ctx, "active_users", userUUID)
					log.Printf("Removed user %s from active users as they have no valid connections left.", userUUID)
				}
			}

			// Batch disconnect requests per URL
			var wg sync.WaitGroup
			for url, uuids := range timedOutMap {
				wg.Add(1)
				go func(disconnectURL string, uuids []string) {
					defer wg.Done()
					if err := sendDisconnectRequest(disconnectURL, uuids); err != nil {
						log.Printf("Error sending batch disconnect request to %s: %v", disconnectURL, err)
					} else {
						log.Printf("Successfully sent batch disconnect request to %s for %d users.", disconnectURL, len(uuids))
					}
				}(url, uuids)
			}
			wg.Wait()
		}
	}()
}
