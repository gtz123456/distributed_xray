package qrcode

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

func GenerateTRXQRCode(address string, amount float64, id string) error {
	uri := fmt.Sprintf("tron:%s?amount=%.6f", address, amount)
	return qrcode.WriteFile(uri, qrcode.Medium, 256, id+".png")
}
