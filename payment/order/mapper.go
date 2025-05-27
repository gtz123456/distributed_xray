// map the payment amount to the actual amount, so we can reuse the same wallet address
// the actual amount = payment amount + n * unit (for TRX, the minimum unit = 1sun = 0.000001trx)

package order

import (
	"time"
)

// d2d0d916521851f8a3a2ea8cc9d63d61ba57ca844d8e1240817a4dab60b2c0db

const defaultWalletAddress = "TTfNfANq9q68hm6xuAjSfafitBo9Did8SY" // default wallet address

type Order struct {
	ID           string    `db:"id"`            // uuid
	TrxAddress   string    `db:"trx_address"`   // wallet address
	Amount       int       `db:"amount"`        // amount to be paid, in sun
	ActualAmount int       `db:"actual_amount"` // actual amount to be paid, in sun
	PaymentLink  string    `db:"payment_link"`  // payment link
	Status       string    `db:"status"`        // 3 status: pending, paid, expired
	CreatedAt    time.Time `db:"created_at"`    // created time
	Callback     string    `db:"callback"`      // callback url
}

var ActualAmountToID map[int64]string = make(map[int64]string) // ActualAmount → Order ID
var intervalSet = NewIntervalSet()                             // store the actual amounts as intervals, for fast searching

var orderMap = make(map[string]*Order) // Order ID → Order

// find minimal actual amount for the given amount
func mapAmountToActualAmount(amount int) (int, error) {
	actualAmount := intervalSet.NextMissing(amount) // convert to int
	intervalSet.Add(actualAmount)                   // add to the interval set
	return actualAmount, nil
}

func CreateOrder(id string, amount int, callback string) (Order, error) {
	actualAmount, err := mapAmountToActualAmount(amount)

	if err != nil {
		return Order{}, err
	}

	order := Order{
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

	return order, nil
}
