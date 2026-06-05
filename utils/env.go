package utils

import (
	"os"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	godotenv.Load()
}

func Regkey() string {
	return os.Getenv("regkey")
}
