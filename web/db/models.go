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

	Plan      string
	PlanStart time.Time
	PlanEnd   time.Time

	RenewCycle time.Duration // renew every X days
	NextRenew  time.Time

	TrafficUsed  int
	TrafficLimit int // in GB, -1 means unlimited

	Active bool // if the user has an active connection
}
