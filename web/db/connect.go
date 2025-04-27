package db

import (
	"fmt"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	var err error

	dsn := os.Getenv("DB")
	fmt.Println(dsn)

	tempDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	err = tempDB.Exec("CREATE DATABASE IF NOT EXISTS vpn CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;").Error
	if err != nil {
		panic(err)
	}

	sqlDB, _ := tempDB.DB()
	sqlDB.Close()

	dsnWithDB := dsn // + "/vpn?charset=utf8mb4&parseTime=True&loc=Local"
	DB, err = gorm.Open(mysql.Open(dsnWithDB), &gorm.Config{})
	if err != nil {
		panic(err)
	}
}
