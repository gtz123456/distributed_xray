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

	RenewCycle time.Duration // renew every X days
	NextRenew  time.Time

	TrafficUsed  int // in Bytes
	TrafficLimit int // in Bytes, -1 means unlimited

	ReferralCode string
	Balance      int // in cents
}
