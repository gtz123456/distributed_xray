package db

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"os"
)

var DB *gorm.DB

func Connect() {
	var err error
	dsn := os.Getenv("DB")
	fmt.Println(dsn)
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		panic(err)
	}
}
