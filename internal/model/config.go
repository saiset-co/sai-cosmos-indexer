package model

type ServiceConfig struct {
	StartBlock    int64   `json:"start_block"`
	NodeAddress   string  `json:"node_address"`
	TxType        string  `json:"tx_type"`
	SleepDuration int64   `json:"sleep_duration"`
	Storage       Storage `json:"storage"`
	SkipFailedTxs bool    `json:"skip_failed_txs"`
}

type Storage struct {
	Token          string `json:"token"`
	URL            string `json:"url"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	CollectionName string `json:"collection_name"`
}
