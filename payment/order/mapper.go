// map the payment amount to the actual amount, so we can reuse the same wallet address
// the actual amount = payment amount + n * unit (for TRX, the minimum unit = 1sun = 0.000001trx)

package order

import (
	"go-distributed/payment/db"
	"time"
)

const defaultWalletAddress = "TQehEHqevPkudydohYrjJxDwdBkAgFUebw" // default wallet address

var ActualAmountToID map[int64]string = make(map[int64]string) // ActualAmount → Order ID
var intervalSet = NewIntervalSet()                             // store the actual amounts as intervals, for fast searching

var orderMap = make(map[string]*db.Order) // Order ID → Order
// TODO: replace with persistent storage

// find minimal actual amount for the given amount
func mapAmountToActualAmount(amount int) (int, error) {
	actualAmount := intervalSet.NextMissing(amount) // convert to int
	intervalSet.Add(actualAmount)                   // add to the interval set
	return actualAmount, nil
}

func CreateOrder(id string, amount int, callback string) (db.Order, error) {
	actualAmount, err := mapAmountToActualAmount(amount)

	if err != nil {
		return db.Order{}, err
	}

	order := db.Order{
		ID:           id,
		TrxAddress:   defaultWalletAddress,
		Amount:       amount,
		ActualAmount: actualAmount,
		Status:       "pending",
		CreatedAt:    time.Now(),
		Callback:     callback,
	}

	orderMap[id] = &order
	ActualAmountToID[int64(actualAmount)] = id // map actual amount to order id

	db.DB.Create(&order)

	return order, nil
}
