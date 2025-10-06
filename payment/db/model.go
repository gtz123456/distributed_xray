package db

import "time"

type Order struct {
	ID           string    `db:"id"`            // uuid
	TrxAddress   string    `db:"trx_address"`   // wallet address
	Amount       int       `db:"amount"`        // amount to be paid, in sun
	ActualAmount int       `db:"actual_amount"` // actual amount to be paid, in sun
	PaymentLink  string    `db:"payment_link"`  // payment link
	Status       string    `db:"status"`        // 4 status: pending, paid, expired, callback_failed
	CreatedAt    time.Time `db:"created_at"`    // created time
	Callback     string    `db:"callback"`      // callback url
}
