package order

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-distributed/payment/db"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

type payHandler struct{}

func RegisterHandlers() {
	handler := new(payHandler)
	http.Handle("/api/payment/order/create", handler)
	http.Handle("/api/payment/order/status", handler)

	go func() {
		// UpdateOrderStatus every 5 seconds
		for {
			UpdateOrderStatus()
			time.Sleep(5 * time.Second)
		}
	}()

	go func() {
		// RemoveTimeoutOrders every 20 seconds
		for {
			RemoveTimeoutOrders()
			time.Sleep(20 * time.Second)
		}
	}()
}

func (ph *payHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		switch r.URL.Path {
		case "/api/payment/order/status":
			ph.handleGetOrderStatus(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	case http.MethodPost:
		switch r.URL.Path {
		case "/api/payment/order/create":
			ph.handleCreateOrder(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (ph *payHandler) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	amount := r.URL.Query().Get("amount")
	callback := r.URL.Query().Get("callback")

	amountInt, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}
	order, err := CreateOrder(id, int(amountInt)*1000000, callback)

	if err != nil {
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonData, err := json.Marshal(order)
	if err != nil {
		http.Error(w, "Failed to serialize order", http.StatusInternalServerError)
		return
	}
	w.Write(jsonData)
	fmt.Println("Order created:", order.ID, order.Amount, order.Callback)

}

func (ph *payHandler) handleGetOrderStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing order ID", http.StatusBadRequest)
		return
	}

	order, ok := orderMap[id] // first try to get from memory
	if !ok {
		// try to get from db
		var dbOrder db.Order
		result := db.DB.First(&dbOrder, "id = ?", id)
		if result.Error != nil {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		order = &dbOrder
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonData, err := json.Marshal(order)
	if err != nil {
		http.Error(w, "Failed to serialize order", http.StatusInternalServerError)
		return
	}
	w.Write(jsonData)
	fmt.Println("Order status requested:", order.ID, order.Status)
}

func GenerateTronAddress() {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}

	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))

	fmt.Println("🔐 Private Key (hex):", privateKeyHex)

	address := address.PubkeyToAddress(privateKey.PublicKey)
	fmt.Println("📬 Tron Address:", address)
}
