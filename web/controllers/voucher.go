package controllers

import (
	"errors"
	"fmt"
	"time"

	"go-distributed/utils"
	"go-distributed/web/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func RedeemVoucher(userID uint, code string) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var user db.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, userID).Error; err != nil {
			return fmt.Errorf("user not found: %w", err)
		}

		var voucher db.Voucher
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ?", code).First(&voucher).Error; err != nil {
			return fmt.Errorf("voucher not found: %w", err)
		}

		// Validate voucher
		if voucher.IsUsed {
			return errors.New("voucher already used")
		}
		if time.Now().After(voucher.ExpiresAt) {
			return errors.New("voucher expired")
		}

		// update user based on voucher type
		switch voucher.Type {
		case "balance":
			user.Balance += voucher.Amount

		case "plan":
			now := time.Now()

			if user.Plan == "Free plan" { // upgrade from free plan
				user.TrafficUsed = 0 // reset traffic
				user.PlanEnd = time.Now().AddDate(0, voucher.PlanDuration, 0)
				user.TrafficLimit = 50 * 1000 * 1000 * 1000 // 50 GB for free plan
			} else { // extend premium plan
				// TODO: reset traffic used when renewing premium plan ?
				if now.After(user.PlanEnd) { // if the current plan is expired, start from now
					user.PlanEnd = now.AddDate(0, voucher.PlanDuration, 0)
				} else { // else extend from current plan end date
					user.PlanEnd = user.PlanEnd.AddDate(0, voucher.PlanDuration, 0)
				}

				user.TrafficLimit = 200 * 1000 * 1000 * 1000 // 200 GB for premium plan
			}

			user.NextRenew = now.AddDate(0, 0, 31) // set next renew to 31 days from now
			user.Plan = voucher.PlanName           // TODO: currently only one premium plan

		default:
			return errors.New("unknown voucher type")
		}

		// Update voucher status
		now := time.Now()
		voucher.IsUsed = true
		voucher.RedeemedBy = user.ID
		voucher.RedeemedAt = &now

		// save changes atomicly
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		if err := tx.Save(&voucher).Error; err != nil {
			return err
		}

		return nil
	})
}

func Redeem(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	if err := RedeemVoucher(userID.(uint), req.Code); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Voucher redeemed successfully"})
}

func GenerateVoucher(c *gin.Context) {
	regkey := c.Param("regkey")
	if regkey != utils.Regkey() {
		c.JSON(403, gin.H{
			"error": "Invalid registration key",
		})
		return
	}

	var req struct {
		Code         string `json:"code"`
		Type         string `json:"type"` // "balance" or "plan"
		Description  string `json:"description"`
		ExpiresAt    string `json:"expires_at"`    // in RFC3339 format
		Amount       int    `json:"amount"`        // in cents, only for balance voucher
		PlanName     string `json:"plan_name"`     // only for plan voucher
		PlanDuration int    `json:"plan_duration"` // in months, only for plan voucher
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid expiration date"})
		return
	}
	voucher := db.Voucher{
		Code:         req.Code,
		Type:         req.Type,
		Description:  req.Description,
		ExpiresAt:    expiresAt,
		Amount:       req.Amount,
		PlanName:     req.PlanName,
		PlanDuration: req.PlanDuration,
		IsUsed:       false,
	}
	if err := db.DB.Create(&voucher).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create voucher"})
		return
	}
	c.JSON(200, gin.H{"message": "Voucher created successfully", "voucher": voucher})
}
