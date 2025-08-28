package controllers

import (
	"go-distributed/web/db"
	"log"
	"os"
	"sync"
	"time"
)

const PLAN_MONITOR_INTERVAL = 10 * time.Second

func StartPlanMonitor() {
	go func() {
		time.Sleep(PLAN_MONITOR_INTERVAL)

		var users []db.User

		// Find all users whose NextRenew is before now and PlanEnd is equal or after now
		now := time.Now()
		err := db.DB.Model(&db.User{}).Where("next_renew < ? AND plan_end >= ?", now, now).Find(&users).Error
		if err != nil {
			return
		}

		// Reset TrafficUsed for each user
		for i := range users {
			users[i].TrafficUsed = 0
			users[i].NextRenew = users[i].NextRenew.Add(users[i].RenewCycle) // Update NextRenew to the next cycle
		}

		for i := range users {
			err = db.DB.Save(&users[i]).Error
			if err != nil {
				return
			}
		}

		users = nil
		// Find all active users whose PlanEnd is before now or TrafficUsed is equal or greater than TrafficLimit
		err = db.DB.Model(&db.User{}).Where("plan_end < ? OR traffic_used >= traffic_limit", now).Find(&users).Error
		if err != nil {
			return
		}

		// Map to collect disconnect URLs per disconnect url
		disconnectURLs := make(map[string][]string)

		for _, user := range users {
			log.Printf("User %s: TrafficUsed=%d, TrafficLimit=%d", user.Email, user.TrafficUsed, user.TrafficLimit)
			if user.PlanEnd.Before(now) || user.TrafficUsed >= user.TrafficLimit {
				// Disconnect all connections for this user
				err = db.DB.Model(&db.User{}).Where("uuid = ?", user.UUID).Update("active", false).Error
				if err != nil {
					return
				}

				// get all connections for this user, from UserConnection table
				userConnectionMapMutex.Lock()
				connections, exists := userConnectionMap[user.UUID]
				if exists {
					delete(userConnectionMap, user.UUID)
					for _, conn := range connections {
						disconnectURL := "http://" + conn.NodeIP + ":" + os.Getenv("Node_Port")
						disconnectURLs[disconnectURL] = append(disconnectURLs[disconnectURL], user.UUID)
					}
				}
				userConnectionMapMutex.Unlock()
			}
		}

		// Batch disconnect requests per URL
		var wg sync.WaitGroup
		for url, uuids := range disconnectURLs {
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
	}()
}
