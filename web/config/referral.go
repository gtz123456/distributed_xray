package config

import (
	"encoding/json"
	"log"
	"os"
)

type ReferralConfig struct {
	SignupRewardDaysReferrer     int `json:"signup_reward_days_referrer"`
	SignupRewardDaysReferee      int `json:"signup_reward_days_referee"`
	PaymentRebatePercentReferrer int `json:"payment_rebate_percent_referrer"`
}

var Referral ReferralConfig

func InitReferralConfig() {
	file, err := os.ReadFile("web/config/referral.json")
	if err != nil {
		log.Println("Referral config not found, using defaults")
		Referral = ReferralConfig{
			SignupRewardDaysReferrer:     30,
			SignupRewardDaysReferee:      30,
			PaymentRebatePercentReferrer: 20,
		}
		return
	}

	err = json.Unmarshal(file, &Referral)
	if err != nil {
		log.Printf("Failed to parse referral.json: %v. Using defaults", err)
		Referral = ReferralConfig{
			SignupRewardDaysReferrer:     30,
			SignupRewardDaysReferee:      30,
			PaymentRebatePercentReferrer: 20,
		}
	}
}
