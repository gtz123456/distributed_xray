package order_test

import (
	"go-distributed/payment/order"
	"testing"
)

func TestCryptoRate(t *testing.T) {
	usdToCny, _ := order.Convert(1, "USD", "CNY")
	t.Log("USD to CNY rate:", usdToCny)
	if usdToCny < 5 || usdToCny > 10 {
		t.Error("USD to CNY rate out of expected range:", usdToCny)
	}

	cnyToTrx, _ := order.Convert(100, "CNY", "TRX")
	t.Logf("100 CNY = %.4f TRX", cnyToTrx)

	usdToTrx, _ := order.Convert(100, "USD", "TRX")
	t.Logf("100 USD = %.4f TRX", usdToTrx)

	trxToCny, _ := order.Convert(100, "TRX", "CNY")
	t.Logf("100 TRX = %.4f CNY", trxToCny)
}
