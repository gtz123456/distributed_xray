package db

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Email    string `gorm:"unique"`
	UUID     string
	Password string

	Plan    string
	PlanEnd time.Time

	RenewCycle int64 // in seconds
	NextRenew  time.Time

	TrafficUsed  int // in Bytes
	TrafficLimit int // in Bytes, -1 means unlimited

	ReferralCode string
	Balance      int // in cents

	// email verification
	IsVerified  bool
	VerifyToken string
	TokenExpiry time.Time
}

type Voucher struct {
	gorm.Model
	Code string `gorm:"type:varchar(191);uniqueIndex"` // Redemption code
	Type string // "balance" or "plan"

	// Common fields
	Description string
	ExpiresAt   time.Time // Expiration time

	// Balance voucher fields
	Amount int // Unit: cents, only valid when type == "balance"

	// Plan voucher fields
	PlanName     string
	PlanDuration int // in months

	// Usage status
	RedeemedBy uint // User ID
	RedeemedAt *time.Time
	IsUsed     bool
}

type Payment struct {
	gorm.Model
	OrderID  string `gorm:"unique"`
	Amount   int    // in cents
	Currency string // e.g. "USD"
	Method   string // e.g. "credit_card", "paypal"
	Status   string // e.g. "pending", "paid", "failed"
}
