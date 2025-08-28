package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-distributed/registry"
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

			// Map to collect timed out connections per disconnect URL
			timedOutMap := make(map[string][]string)
			now := time.Now()

			for userUUID, connections := range usersToProcess {
				var validConnections []UserConnection

				for _, conn := range connections {
					if now.Sub(conn.LastHeartBeat) <= HEARTBEAT_TIMEOUT {
						validConnections = append(validConnections, conn)
					} else {
						disconnectURL := "http://" + conn.NodeIP + ":" + os.Getenv("Node_Port") + "/disconnect"
						timedOutMap[disconnectURL] = append(timedOutMap[disconnectURL], userUUID)
					}
				}

				// Update the userConnectionMap with valid connections
				userConnectionMapMutex.Lock()
				if len(validConnections) == 0 {
					delete(userConnectionMap, userUUID)

					log.Printf("Removed user %s from connection map as they have no valid connections left.", userUUID)
				} else {
					userConnectionMap[userUUID] = validConnections
				}
				userConnectionMapMutex.Unlock()
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
