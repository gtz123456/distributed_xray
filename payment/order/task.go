// schedule tasks to update order status from TronGrid api

package order

import (
	"encoding/json"
	"fmt"
	"go-distributed/payment/db"
	"net/http"
	"os"
	"time"

	"log"
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

			orderID, ok := ActualAmountToID[amount]
			if !ok {
				log.Println("Order ID not found for amount:", amount)
				continue
			}

			order, ok := orderMap[orderID]
			if !ok {
				log.Println("Order not found:", orderID)
				continue
			}

			fmt.Println("Order found:", order)
			if order.Status == "pending" {
				order.Status = "paid"
				intervalSet.Remove(order.ActualAmount)
				db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("status", "paid")
				if order.Callback != "" {
					callbackUrl := fmt.Sprintf("%s?id=%s", order.Callback, order.ID)
					resp, err := http.Get(callbackUrl)
					if err != nil {
						log.Println("Error calling callback URL:", err)
						continue
					}
					if resp.StatusCode != http.StatusOK {
						log.Println("Callback URL returned non-200 status:", resp.StatusCode)
						resp.Body.Close()
						continue
					}
					resp.Body.Close()

					delete(ActualAmountToID, amount)
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
	for id, order := range orderMap {
		if time.Since(order.CreatedAt) > paymentTimeout {
			delete(orderMap, id)
			delete(ActualAmountToID, int64(order.ActualAmount))
			log.Println("Order timeout:", id)
		}
	}
}
