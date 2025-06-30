package controllers

import (
	"go-distributed/web/db"
	"time"
)

func PlanMonitor() {
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

	err = db.DB.Save(&users).Error
	if err != nil {
		return
	}

	users = nil
	// Find all active users whose PlanEnd is before now or TrafficUsed is equal or greater than TrafficLimit
	err = db.DB.Model(&db.User{}).Where("active = ? AND (plan_end < ? OR traffic_used >= traffic_limit)", true, now).Find(&users).Error
	if err != nil {
		return
	}

	for _, user := range users {
		if user.PlanEnd.Before(now) || user.TrafficUsed >= user.TrafficLimit {
			// Disconnect all connections for this user
		}
	}
}
