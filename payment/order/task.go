// schedule tasks to update order status from TronGrid api

package order

import (
	"encoding/json"
	"fmt"
	"go-distributed/payment/db"
	"go-distributed/utils"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const paymentTimeout = 15 * time.Minute
const apiUrl = "https://api.shasta.trongrid.io/v1/accounts/%s/transactions" // testnet

var trongridApiKey = os.Getenv("TRONGRID_API_KEY")

func UpdateOrderStatus() {
	// update order status from TronGrid api
	minTimestamp := time.Now().Add(-paymentTimeout).Unix() * 1000
	limit := 200 // number of transactions per page, default 20, max 200
	next := ""

	for { // query result is paged, so we need to loop until all results are fetched
		urlWithParams := fmt.Sprintf("%s?min_timestamp=%d&limit=%d", fmt.Sprintf(apiUrl, defaultWalletAddress), minTimestamp, limit)
		if next != "" {
			urlWithParams += "&next=" + next
		}

		req, err := http.NewRequest("GET", urlWithParams, nil)
		if err != nil {
			fmt.Println("Error creating request:", err)
			return
		}
		if trongridApiKey != "" {
			req.Header.Set("TRON-PRO-API-KEY", trongridApiKey)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("Error getting order status:", err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Println("Error getting order status:", resp.Status)
			return
		}

		var result TransactionResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			panic(err)
		}
		resp.Body.Close()

		for _, tx := range result.Data {
			// filter transactions within paymentTimeout
			if tx.BlockTimestamp/1000 < time.Now().Add(-paymentTimeout).Unix() {
				continue
			}

			// check trx transaction
			if tx.RawData.Contract[0].Type != "TransferContract" {
				continue
			}

			amount := tx.RawData.Contract[0].Parameter.Value.Amount

			orderID, err := utils.RedisClient.Get(utils.Ctx, "actual_amount_to_id:"+strconv.FormatInt(amount, 10)).Result()
			if err != nil {
				continue
			}

			var order db.Order
			if err := db.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
				log.Println("Order not found in DB:", orderID)
				continue
			}

			fmt.Println("Order found:", order)
			if order.Status == "pending" {
				order.Status = "paid"
				intervalSet.Remove(order.ActualAmount)
				utils.RedisClient.Del(utils.Ctx, "actual_amount_to_id:"+strconv.FormatInt(amount, 10))
				db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("status", "paid")

				if order.Callback != "" {
					regkey := utils.Regkey()
					callbackUrl := fmt.Sprintf("%s?order_id=%s", order.Callback, order.ID)
					req, err := http.NewRequest("POST", callbackUrl, nil)
					if err != nil {
						log.Println("Error creating callback request:", err)
						order.Status = "callback_failed"
						db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("status", "callback_failed")
						continue
					}
					req.Header.Set("regkey", regkey)
					client := &http.Client{}
					callbackResp, err := client.Do(req)
					if err != nil {
						log.Println("Error calling callback URL:", err)
						order.Status = "callback_failed"
						db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("status", "callback_failed")
						continue
					}
					if callbackResp.StatusCode != http.StatusOK {
						log.Println("Callback URL returned non-200 status:", callbackResp.StatusCode)
						order.Status = "callback_failed"
						db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("status", "callback_failed")
						callbackResp.Body.Close()
						continue
					}
					callbackResp.Body.Close()
				}
			}
		}

		if result.Meta.Links.Next == "" {
			break
		}
		next = result.Meta.Links.Next
	}
}

func RemoveTimeoutOrders() {
	now := time.Now()
	timeoutTime := now.Add(-paymentTimeout)

	var expiredOrders []db.Order
	db.DB.Model(&db.Order{}).Where("status = ? AND created_at < ?", "pending", timeoutTime).Find(&expiredOrders)

	for _, order := range expiredOrders {
		intervalSet.Remove(order.ActualAmount)
		log.Println("Order timeout:", order.ID)
		db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("status", "expired")
	}
}

func RetryCallbackFailedOrders() {
	var failedOrders []db.Order
	db.DB.Model(&db.Order{}).Where("status = ?", "callback_failed").Find(&failedOrders)

	for _, order := range failedOrders {
		if order.Callback != "" {
			regkey := utils.Regkey()
			callbackUrl := fmt.Sprintf("%s?order_id=%s", order.Callback, order.ID)
			req, err := http.NewRequest("POST", callbackUrl, nil)
			if err != nil {
				log.Println("Error creating callback request:", err)
				continue
			}
			req.Header.Set("regkey", regkey)
			client := &http.Client{}
			callbackResp, err := client.Do(req)
			if err != nil {
				log.Println("Error calling callback URL:", err)
				continue
			}
			if callbackResp.StatusCode != http.StatusOK {
				log.Println("Callback URL returned non-200 status:", callbackResp.StatusCode)
				callbackResp.Body.Close()
				continue
			}
			callbackResp.Body.Close()
			db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("status", "paid")
		}
	}
}
