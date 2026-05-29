package main

import (
	"fmt"
	"go-distributed/utils"
	"go-distributed/web/db"
)

func main() {
	utils.LoadEnv()
	db.Connect()

	var users []db.User
	db.DB.Where("referral_code = ?", "").Find(&users)

	count := 0
	for _, user := range users {
		user.ReferralCode = utils.GenerateUUID()[:8]
		db.DB.Save(&user)
		count++
	}

	fmt.Printf("Migration completed. Generated referral codes for %d existing users.\n", count)
}
