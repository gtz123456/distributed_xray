// map the payment amount to the actual amount, so we can reuse the same wallet address
// the actual amount = payment amount + n * unit (for TRX, the minimum unit = 1sun = 0.000001trx)

package order

import (
	"errors"
	"go-distributed/payment/db"
	"go-distributed/utils"
	"os"
	"strconv"
	"time"
)

const defaultWalletAddress = "TQehEHqevPkudydohYrjJxDwdBkAgFUebw" // default wallet address

var intervalSet = NewIntervalSet()                             // store the actual amounts as intervals, for fast searching

func init() {
	utils.LoadEnv()

	if os.Getenv("TEST_MODE") == "1" {
		return
	}

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

	trxAmount, err := Convert(float64(amount)/100, currency, method) // convert USD to TRX

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

	utils.RedisClient.Set(utils.Ctx, "actual_amount_to_id:"+strconv.FormatInt(actualAmount, 10), id, paymentTimeout)

	return order, nil
}

// restore ActualAmountToID and intervalSet from existing orders in db, in case of server restart
func RestoreStateFromDB() error {
	var orders []db.Order

	now := time.Now()
	cutoffTime := now.Add(-paymentTimeout)
	result := db.DB.Model(&db.Order{}).
		Where("status = ?", "pending").
		Where("created_at > ?", cutoffTime).
		Find(&orders)
	if result.Error != nil {
		return result.Error
	}

	for _, order := range orders {
		intervalSet.Add(order.ActualAmount)
		
		ttl := time.Until(order.CreatedAt.Add(paymentTimeout))
		if ttl > 0 {
			utils.RedisClient.Set(utils.Ctx, "actual_amount_to_id:"+strconv.FormatInt(order.ActualAmount, 10), order.ID, ttl)
		}
	}

	return nil
}
