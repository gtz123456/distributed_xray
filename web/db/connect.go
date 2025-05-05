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

	baseDSN := os.Getenv("DB") + "/"
	fmt.Println("Initial DSN:", baseDSN)

	tempDB, err := gorm.Open(mysql.Open(baseDSN), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("initial connection failed: %w", err))
	}

	err = tempDB.Exec("CREATE DATABASE IF NOT EXISTS vpn CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;").Error
	if err != nil {
		panic(fmt.Errorf("failed to create database: %w", err))
	}

	sqlDB, _ := tempDB.DB()
	sqlDB.Close()

	dsnWithDB := baseDSN + "vpn?charset=utf8mb4&parseTime=True&loc=Local"
	fmt.Println("Connecting to:", dsnWithDB)

	DB, err = gorm.Open(mysql.Open(dsnWithDB), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("final connection failed: %w", err))
	}
}
