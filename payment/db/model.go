package db

import "time"

type Order struct {
	ID           string    `db:"id" json:"id"`                       // uuid
	TrxAddress   string    `db:"trx_address" json:"trx_address"`     // wallet address
	Amount       int64     `db:"amount" json:"amount"`               // amount to be paid, in original currency's smallest unit (e.g., cents for USD)
	Currency     string    `db:"currency" json:"currency"`           // currency, e.g., USD, TRX
	ActualAmount int64     `db:"actual_amount" json:"actual_amount"` // actual amount to be paid, in sun
	PaymentLink  string    `db:"payment_link" json:"payment_link"`   // payment link
	Status       string    `db:"status" json:"status"`               // 4 status: pending, paid, expired, callback_failed
	CreatedAt    time.Time `db:"created_at" json:"created_at"`       // created time
	Callback     string    `db:"callback" json:"callback"`           // callback url
	Method       string    `db:"method" json:"method"`               // payment method, e.g., TRX
}
