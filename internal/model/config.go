package model

type ServiceConfig struct {
	NodeAddress    string
	TxType         string
	SkipFailedTxs  bool
	CollectionName string
}
