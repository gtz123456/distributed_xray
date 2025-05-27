package order

type TransactionResponse struct {
	Data    []Transaction `json:"data"`
	Success bool          `json:"success"`
	Meta    Meta          `json:"meta"`
}

type Transaction struct {
	Ret                  []Ret         `json:"ret"`
	Signature            []string      `json:"signature"`
	TxID                 string        `json:"txID"`
	NetUsage             int64         `json:"net_usage"`
	RawDataHex           string        `json:"raw_data_hex"`
	NetFee               int64         `json:"net_fee"`
	EnergyUsage          int64         `json:"energy_usage"`
	BlockNumber          int64         `json:"blockNumber"`
	BlockTimestamp       int64         `json:"block_timestamp"`
	EnergyFee            int64         `json:"energy_fee"`
	EnergyUsageTotal     int64         `json:"energy_usage_total"`
	RawData              RawData       `json:"raw_data"`
	InternalTransactions []interface{} `json:"internal_transactions"` // 可根据需要进一步定义
}

type Ret struct {
	ContractRet string `json:"contractRet"`
	Fee         int64  `json:"fee"`
}

type RawData struct {
	Contract      []Contract `json:"contract"`
	RefBlockBytes string     `json:"ref_block_bytes"`
	RefBlockHash  string     `json:"ref_block_hash"`
	Expiration    int64      `json:"expiration"`
	Timestamp     int64      `json:"timestamp"`
}

type Contract struct {
	Parameter ContractParameter `json:"parameter"`
	Type      string            `json:"type"`
}

type ContractParameter struct {
	Value   TransferValue `json:"value"`
	TypeURL string        `json:"type_url"`
}

type TransferValue struct {
	Amount       int64  `json:"amount"`
	OwnerAddress string `json:"owner_address"`
	ToAddress    string `json:"to_address"`
}

type Meta struct {
	At          int64  `json:"at"`
	PageSize    int    `json:"page_size"`
	Fingerprint string `json:"fingerprint"`
	Links       Links  `json:"links"`
}

type Links struct {
	Next string `json:"next"`
}
