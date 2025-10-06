// map the payment amount to the actual amount, so we can reuse the same wallet address
// the actual amount = payment amount + n * unit (for TRX, the minimum unit = 1sun = 0.000001trx)

package order

import (
	"errors"
	"go-distributed/payment/db"
	"go-distributed/utils"
	"time"
)

const defaultWalletAddress = "TQehEHqevPkudydohYrjJxDwdBkAgFUebw" // default wallet address

var ActualAmountToID map[int64]string = make(map[int64]string) // ActualAmount → Order ID
var intervalSet = NewIntervalSet()                             // store the actual amounts as intervals, for fast searching

var orderMap = make(map[string]*db.Order) // Order ID → Order
// TODO: replace with persistent storage eg. Redis

func init() {
	utils.LoadEnv()

	db.Connect()
	db.Sync()

	err := RestoreStateFromDB()
	if err != nil {
		panic(err)
	}
}

// find minimal actual amount for the given amount
func mapAmountToActualAmount(amount int64) (int64, error) {
	actualAmount := intervalSet.NextMissing(amount) // convert to int
	intervalSet.Add(actualAmount)                   // add to the interval set
	return actualAmount, nil
}

func CreateOrder(id string, amount int64, callback, method, currency string) (db.Order, error) {
	if method != "TRX" {
		return db.Order{}, errors.New("unsupported payment method")
	}
	// TODO: support more payment methods

	trxAmount, err := Convert(float64(amount), "USD", currency) // convert USD to TRX

	if err != nil {
		return db.Order{}, err
	}

	actualAmount, err := mapAmountToActualAmount(int64(trxAmount * 1000000)) // convert to sun
	if err != nil {
		return db.Order{}, err
	}

	order := db.Order{
		ID:           id,
		TrxAddress:   defaultWalletAddress,
		Amount:       amount,
		Currency:     currency,
		ActualAmount: actualAmount,
		Status:       "pending",
		CreatedAt:    time.Now(),
		Callback:     callback,
		Method:       method,
	}

	result := db.DB.Create(&order)
	if result.Error != nil {
		return db.Order{}, result.Error
	}

	orderMap[id] = &order
	ActualAmountToID[int64(actualAmount)] = id // map actual amount to order id

	return order, nil
}

// restore ActualAmountToID and intervalSet from existing orders in db, in case of server restart
func RestoreStateFromDB() error {
	var orders []db.Order

	now := time.Now()
	result := db.DB.Model(&db.Order{}).
		Where("status = ?", "pending").
		Where("created_at + interval ? second > ?", int(paymentTimeout.Seconds()), now).
		Find(&orders)
	if result.Error != nil {
		return result.Error
	}

	for _, order := range orders {
		orderMap[order.ID] = &order
		newActualAmount, err := mapAmountToActualAmount(order.Amount)
		if err != nil {
			return err
		}

		if newActualAmount != order.ActualAmount {
			order.ActualAmount = newActualAmount
			result = db.DB.Model(&db.Order{}).Where("id = ?", order.ID).Update("actual_amount", newActualAmount)
			if result.Error != nil {
				return result.Error
			}
		}
		ActualAmountToID[int64(order.ActualAmount)] = order.ID
	}

	return nil
}
