package controllers

import (
	"encoding/json"
	"go-distributed/utils"
	"go-distributed/web/db"
	"log"
	"time"
)

const PLAN_MONITOR_INTERVAL = 10 * time.Second

func StartPlanMonitor() {
	go func() {
		for {
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
				users[i].NextRenew = users[i].NextRenew.Add(time.Duration(users[i].RenewCycle) * time.Second) // Update NextRenew to the next cycle, RenewCycle is in seconds
			}

			for i := range users {
				err = db.DB.Save(&users[i]).Error
				if err != nil {
					return
				}
			}

			users = nil
			// Find all users whose PlanEnd is before now or TrafficUsed is equal or greater than TrafficLimit
			err = db.DB.Model(&db.User{}).Where("plan_end < ? OR (traffic_limit != -1 AND traffic_used >= traffic_limit)", now).Find(&users).Error
			if err != nil {
				log.Printf("PlanMonitor DB error: %v", err)
				continue
			}

			for _, user := range users {
				log.Printf("User %s: TrafficUsed=%d, TrafficLimit=%d", user.Email, user.TrafficUsed, user.TrafficLimit)

				// get all connections for this user from Redis
				conns, err := utils.RedisClient.HGetAll(utils.Ctx, "user_conns:"+user.UUID).Result()
				if err == nil && len(conns) > 0 {
					utils.RedisClient.Del(utils.Ctx, "user_conns:"+user.UUID)
					utils.RedisClient.SRem(utils.Ctx, "active_users", user.UUID)
					for _, connJSON := range conns {
						var conn UserConnection
						json.Unmarshal([]byte(connJSON), &conn)
						// The new declarative way: Just remove from node_ports hash
						utils.RedisClient.HDel(utils.Ctx, "node_ports:"+conn.NodeIP, user.UUID)
					}
				}
			}
		}
	}()
}
