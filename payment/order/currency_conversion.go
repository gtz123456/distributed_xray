package order

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

var defaultRates = map[string]float64{
	"USD":  1.0,
	"USDT": 1.0,
	"CNY":  0.14,
	"TRX":  0.33, // 1 TRX = 0.33 USD
}

var ratesCache map[string]float64
var lastFetchTime int64 = 0
var cacheDuration int64 = 300 // seconds

// --- Open ER API ---
type ERResponse struct {
	Rates map[string]float64 `json:"rates"`
}

// --- OKX API ---
type OKXResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		InstId string `json:"instId"`
		Last   string `json:"last"`
	} `json:"data"`
}

// fetch fiat rates from Open ER API
func FetchFiatRates() (map[string]float64, error) {
	url := "https://open.er-api.com/v6/latest/USD"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var data ERResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	rates := make(map[string]float64)
	for k, v := range data.Rates {
		// 1 USD = v target → 1 target = 1/v USD
		if v > 0 {
			rates[k] = 1.0 / v
		}
	}
	rates["USD"] = 1.0
	return rates, nil
}

// fetch crypto pair price from OKX
func FetchOKXPair(instId string) (float64, error) {
	url := fmt.Sprintf("https://www.okx.com/api/v5/market/ticker?instId=%s", instId)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var result OKXResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	if len(result.Data) == 0 {
		return 0, fmt.Errorf("no data for %s", instId)
	}
	var price float64
	fmt.Sscanf(result.Data[0].Last, "%f", &price)
	return price, nil
}

func FetchAllRates() (map[string]float64, error) {
	// Fetch fiat rates
	fiatRates, err := FetchFiatRates()
	if err != nil {
		return nil, err
	}

	okxRates := make(map[string]float64)

	trxusdt, err := FetchOKXPair("TRX-USDT")
	if err != nil {
		return nil, err
	}

	if trxusdt > 0 {
		okxRates["TRX"] = trxusdt
	}

	// combine all rates
	allRates := make(map[string]float64)
	for k, v := range fiatRates {
		allRates[k] = v
	}
	for k, v := range okxRates {
		allRates[k] = v
	}

	for k, v := range defaultRates {
		if _, ok := allRates[k]; !ok {
			allRates[k] = v
		}
	}

	ratesCache = allRates
	lastFetchTime = time.Now().Unix()

	return allRates, nil
}

// Convert amount from one currency to another
func Convert(amount float64, from, to string) (float64, error) {
	if time.Now().Unix()-lastFetchTime > cacheDuration {
		_, err := FetchAllRates()
		if err != nil {
			return 0, err
		}
	}

	rA, ok1 := ratesCache[from]
	rB, ok2 := ratesCache[to]
	if !ok1 || !ok2 {
		return 0, fmt.Errorf("unsupported currency %s or %s", from, to)
	}
	return amount * (rA / rB), nil
}
