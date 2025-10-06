package controllers

import (
	"encoding/json"
	"fmt"
	"go-distributed/registry"
	"go-distributed/utils"
	"go-distributed/web/db"
	"os"

	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func Payment(c *gin.Context) {
	user, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	userinfo := user.(db.User)

	var req struct {
		Amount   int    `json:"amount"` // in cents
		Currency string `json:"currency"`
		Method   string `json:"method"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// generate a uuid as order ID
	orderid := utils.GenerateUUID()

	payment := db.Payment{
		OrderID:  orderid,
		UserID:   userinfo.ID,
		Amount:   req.Amount,
		Currency: req.Currency,
		Method:   req.Method,
		Status:   "pending",
	}

	if err := db.DB.Create(&payment).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create payment"})
		return
	}

	paymentService, err := registry.GetProviders(registry.PaymentService)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get payment service"})
		return
	}

	addr := paymentService[0].ServiceURL
	client := &http.Client{}
	publicIP, err := utils.GetPublicIP()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get public IP"})
		return
	}
	callbackURL := fmt.Sprintf("http://%s:%s/payment/callback", publicIP, os.Getenv("GIN_PORT"))

	reqURL := fmt.Sprintf("%s/api/payment/order/create?order_id=%s&amount=%d&callback=%s&method=%s&currency=%s",
		addr,
		orderid,
		req.Amount,
		url.QueryEscape(callbackURL),
		req.Method,
		req.Currency,
	)

	request, err := http.NewRequest("POST", reqURL, nil)

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create payment request"})
		return
	}

	resp, err := client.Do(request)
	if err != nil || resp.StatusCode != 200 {
		c.JSON(500, gin.H{"error": "Failed to process payment"})
		return
	}
	defer resp.Body.Close()

	// get response body as json
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(500, gin.H{"error": "Failed to parse payment response"})
		return
	}
	if payment.Method == "TRX" {
		if _, ok := result["trx_address"].(string); !ok {
			c.JSON(500, gin.H{"error": "Invalid payment response"})
		}

		var actualAmount int64
		if v, ok := result["actual_amount"].(float64); ok {
			actualAmount = int64(v)
		} else if v, ok := result["actual_amount"].(int64); ok {
			actualAmount = v
		} else {
			actualAmount = 0
		}

		fmt.Println("Payment created:", result)
		c.JSON(200, gin.H{"message": "Payment submitted", "order_id": orderid, "trx_address": result["trx_address"].(string), "actual_amount": actualAmount})
		return
	}
	// TODO: handle other payment methods
	c.JSON(500, gin.H{"error": "Unsupported payment method"})
}

func Callback(c *gin.Context) {
	orderID := c.Query("order_id")

	tx := db.DB.Begin()

	var payment db.Payment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("order_id = ?", orderID).First(&payment).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Payment not found"})
		return
	}

	// update payment status
	payment.Status = "paid"
	if err := tx.Save(&payment).Error; err != nil {
		tx.Rollback()
		return
	}

	// update user balance
	var user db.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&user, payment.UserID).Error; err != nil {
		tx.Rollback()
		return
	}

	user.Balance += payment.Amount
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	c.JSON(200, gin.H{"message": "Payment status updated"})
}

func updatePaymentStatus(orderID string) error { // query payment service for orders that failed to callback
	paymentService, err := registry.GetProviders(registry.PaymentService)
	if err != nil {
		return err
	}
	addr := paymentService[0].ServiceURL
	client := &http.Client{}
	request, err := http.NewRequest("POST", addr+"/api/payment/order/status?order_id="+orderID, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to update payment status")
	}

	// Parse the response body as JSON and return the order info if needed
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if status, ok := result["status"].(string); ok {
		if status == "paid" || status == "callback_failed" {
			db.DB.Model(&db.Payment{}).Where("order_id = ?", orderID).Update("status", "paid")
		}
	}

	return nil
}

func GetPaymentStatus(c *gin.Context) {
	orderID := c.Param("order_id")

	var payment db.Payment
	if err := db.DB.Where("order_id = ?", orderID).First(&payment).Error; err != nil {
		c.JSON(404, gin.H{"error": "Payment not found"})
		return
	}

	c.JSON(200, gin.H{"order_id": payment.OrderID, "status": payment.Status})
}

func ListPayments(c *gin.Context) {
	user, ok := c.Get("user")
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var payments []db.Payment
	if err := db.DB.Where("user_id = ?", user.(db.User).ID).Find(&payments).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch payments"})
		return
	}

	c.JSON(200, gin.H{"payments": payments})
}
