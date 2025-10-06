package db

import "time"

type Order struct {
	ID           string    `db:"id" json:"id"`                       // uuid
	TrxAddress   string    `db:"trx_address" json:"trx_address"`     // wallet address
	Amount       int       `db:"amount" json:"amount"`               // amount to be paid, in sun
	ActualAmount int       `db:"actual_amount" json:"actual_amount"` // actual amount to be paid, in sun
	PaymentLink  string    `db:"payment_link" json:"payment_link"`   // payment link
	Status       string    `db:"status" json:"status"`               // 4 status: pending, paid, expired, callback_failed
	CreatedAt    time.Time `db:"created_at" json:"created_at"`       // created time
	Callback     string    `db:"callback" json:"callback"`           // callback url
}
