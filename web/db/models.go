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

	Renew time.Duration

	TrafficLimit int // in GB, -1 means unlimited
	TrafficUsed  int
}
