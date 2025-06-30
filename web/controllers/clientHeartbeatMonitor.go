package controllers

import (
	"go-distributed/registry"
	"go-distributed/web/db"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

func StartHeartbeatMonitor() {
	go func() {
		log.Println("Starting heartbeat monitor...")

		// reuse the http client for disconnect requests
		httpClient := &http.Client{
			Timeout: 10 * time.Second,
		}

		for {
			time.Sleep(HEARTBEAT_CHECK_INTERVAL)

			// remove disconnected nodes from userConnectionMap
			regs, err := registry.GetProviders(registry.NodeService)

			if err != nil {
				log.Printf("Error fetching node services: %v", err)
			}

			userConnectionMapMutex.Lock()
			for userUUID, connections := range userConnectionMap {
				validConnections := make([]UserConnection, 0)
				for _, conn := range connections {
					found := false
					for _, reg := range regs {
						if conn.ServiceID == reg.ServiceID && conn.NodeIP == reg.PublicIP {
							found = true
							break
						}
					}
					if found {
						validConnections = append(validConnections, conn)
					} else {
						log.Printf("Removing connection for user %s to node %s as it is no longer available.", userUUID, conn.NodeIP)
					}
				}
				if len(validConnections) == 0 {
					delete(userConnectionMap, userUUID)
					log.Printf("Removed user %s from connection map as they have no valid connections left.", userUUID)
				} else {
					userConnectionMap[userUUID] = validConnections
				}
			}
			userConnectionMapMutex.Unlock()

			usersToProcess := make(map[string][]UserConnection)

			// make a snapshot of the current state
			userConnectionMapMutex.RLock()
			for userUUID, connections := range userConnectionMap {
				copiedConnections := make([]UserConnection, len(connections))
				copy(copiedConnections, connections)
				usersToProcess[userUUID] = copiedConnections
			}
			userConnectionMapMutex.RUnlock()

			for userUUID, connections := range usersToProcess {
				var validConnections []UserConnection
				var timedOutConnections []UserConnection
				now := time.Now()

				for _, conn := range connections {
					if now.Sub(conn.LastHeartBeat) <= HEARTBEAT_TIMEOUT {
						validConnections = append(validConnections, conn)
					} else {
						timedOutConnections = append(timedOutConnections, conn)
					}
				}

				if len(timedOutConnections) > 0 {
					var wg sync.WaitGroup
					log.Printf("User %s has %d timed out connections to clean up.", userUUID, len(timedOutConnections))

					// Disconnect the timed out connections
					for _, conn := range timedOutConnections {
						wg.Add(1)
						go func(c UserConnection) {
							defer wg.Done()
							disconnectURL := "http://" + c.NodeIP + ":" + os.Getenv("Node_Port") + "/disconnect?uuid=" + userUUID
							req, err := http.NewRequest("GET", disconnectURL, nil)
							if err != nil {
								log.Printf("Error creating disconnect request for user %s, node %s: %v", userUUID, c.NodeIP, err)
								return
							}

							resp, err := httpClient.Do(req)
							if err != nil {
								log.Printf("Error sending disconnect request for user %s, node %s: %v", userUUID, c.NodeIP, err)
								return
							}
							defer resp.Body.Close()

							if resp.StatusCode != http.StatusOK {
								log.Printf("Failed to disconnect user %s from node %s, status code: %s", userUUID, c.NodeIP, resp.Status)
							} else {
								log.Printf("Successfully disconnected user %s from node %s.", userUUID, c.NodeIP)
							}
						}(conn)
					}
					wg.Wait()
				}

				// Update the userConnectionMap with valid connections
				userConnectionMapMutex.Lock()
				if len(validConnections) == 0 {
					delete(userConnectionMap, userUUID)
					// update the user in the database to set Active to false
					if err := db.DB.Model(&db.User{}).Where("uuid = ?", userUUID).Update("active", false).Error; err != nil {
						log.Printf("Error updating user %s to inactive: %v", userUUID, err)
					}

					log.Printf("Removed user %s from connection map as they have no valid connections left.", userUUID)
				} else {
					userConnectionMap[userUUID] = validConnections
				}
				userConnectionMapMutex.Unlock()
			}
		}
	}()
}
