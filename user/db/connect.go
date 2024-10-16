package db

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"os"
)

var DB *gorm.DB

func Connect() {
	var err error
	dsn := os.Getenv("DB")
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		panic(err)
	}
}

