package controllers

import (
	"go-distributed/utils"
	"go-distributed/web/db"
	"time"

	"github.com/gin-gonic/gin"
)

func SetPlan(c *gin.Context) {
	uuid := c.Param("uuid")
	plan := c.Param("plan")
	regkey := c.Param("regkey")

	if regkey != utils.Regkey() {
		c.JSON(403, gin.H{
			"error": "Invalid registration key",
		})
	}

	user := db.User{}
	err := db.DB.Model(&db.User{}).Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		c.JSON(404, gin.H{
			"error": "User not found",
		})
	}

	if user.UUID == "" {
		c.JSON(404, gin.H{
			"error": "User not found",
		})
	}

	// TODO: read plan from config file
	if plan == "Free plan" {
		user.Plan = "Free plan"
		user.TrafficUsed = 0
		user.TrafficLimit = 50 * 1000 // 50 GB
		user.NextRenew = time.Now().Add(31 * 24 * time.Hour)
		user.PlanEnd = time.Now().Add(100 * 12 * 31 * 24 * time.Hour) // Set PlanEnd to a far future date
	} else if plan == "Premium plan" {
		user.Plan = "Premium plan"
		user.TrafficUsed = 0
		user.TrafficLimit = 200 * 1000 // 200 GB
		user.NextRenew = time.Now().Add(31 * 24 * time.Hour)
		// user.PlanEnd = time.Now().Add(31 * 24 * time.Hour)
	} else {
		c.JSON(400, gin.H{
			"error": "Invalid plan",
		})
	}

	err = db.DB.Save(&user).Error
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Failed to update user plan",
		})
	}

	c.JSON(200, gin.H{
		"message": "Plan updated successfully",
		"user":    user,
	})
}
