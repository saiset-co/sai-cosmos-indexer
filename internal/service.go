package internal

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/saiset-co/sai-service-crud-plus/logger"
	"github.com/saiset-co/saiCosmosIndexer/internal/model"
	"github.com/saiset-co/saiService"
	"github.com/saiset-co/saiStorageUtil"
	"github.com/spf13/cast"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

const (
	filePathAddresses   = "./addresses.json"
	filePathLatestBlock = "./latest_handled_block"
)

type InternalService struct {
	mu           *sync.Mutex
	Context      *saiService.Context
	config       model.ServiceConfig
	currentBlock int64
	client       http.Client
	addresses    map[string]struct{}
	storage      saiStorageUtil.Database
}

func (is *InternalService) Init() {
	is.Context.GetConfig("storage.mongo_collection_name", "")

	is.mu = &sync.Mutex{}
	is.client = http.Client{
		Timeout: 5 * time.Second,
	}
	is.addresses = make(map[string]struct{})
	is.storage = saiStorageUtil.Storage(
		cast.ToString(is.Context.GetConfig("storage.url", "")),
		cast.ToString(is.Context.GetConfig("storage.email", "")),
		cast.ToString(is.Context.GetConfig("storage.token", "")),
	)

	is.config.TxType = cast.ToString(is.Context.GetConfig("tx_type", ""))
	is.config.NodeAddress = cast.ToString(is.Context.GetConfig("node_address", ""))
	is.config.CollectionName = cast.ToString(is.Context.GetConfig("storage.mongo_collection_name", ""))
	is.config.SkipFailedTxs = cast.ToBool(is.Context.GetConfig("skip_failed_tx", false))

	err := is.loadAddresses()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Logger.Error("loadAddresses", zap.Error(err))
	}

	fileBytes, err := os.ReadFile(filePathLatestBlock)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Logger.Error("can't read "+filePathLatestBlock, zap.Error(err))
		}
	} else {
		latestHandledBlock, err := strconv.Atoi(string(fileBytes))
		if err != nil {
			logger.Logger.Error("strconv.Atoi", zap.Error(err))
		}

		is.currentBlock = int64(latestHandledBlock)
	}

	startBlock := cast.ToInt64(is.Context.GetConfig("start_block", 0))
	if is.currentBlock < startBlock {
		is.currentBlock = startBlock
	}
}

func (is *InternalService) Process() {
	sleepDuration := cast.ToDuration(is.Context.GetConfig("sleep_duration", 2))

	for {
		select {
		case <-is.Context.Context.Done():
			logger.Logger.Debug("saiCosmosIndexer loop is done")
			return
		default:
			if len(is.addresses) == 0 {
				time.Sleep(time.Second * sleepDuration)
				continue
			}

			latestBlockHeight, err := is.getLatestBlock()
			if err != nil {
				logger.Logger.Error("getLatestBlock", zap.Error(err))
				time.Sleep(time.Second * sleepDuration)
				continue
			}

			if is.currentBlock >= latestBlockHeight {
				time.Sleep(time.Second * sleepDuration)
				continue
			}

			err = is.handleBlockTxs()
			if err != nil {
				logger.Logger.Error("handleBlockTxs", zap.Error(err))
				time.Sleep(time.Second * sleepDuration)
				continue
			}

			is.currentBlock += 1
		}
	}
}

func (is *InternalService) handleBlockTxs() error {
	blockTxs, err := is.getBlockTxs()
	if err != nil {
		return err
	}

	for _, txRes := range blockTxs {
		if is.config.SkipFailedTxs && txRes.Code != 0 {
			continue
		}

		if len(txRes.Tx.Body.Messages) < 1 {
			continue
		}

		to := txRes.Tx.Body.Messages[0].ToAddress
		from := txRes.Tx.Body.Messages[0].FromAddress
		amount := txRes.Tx.Body.Messages[0].Amount[0].Amount

		_, isReceiver := is.addresses[to]
		_, isSender := is.addresses[from]
		if !isReceiver && !isSender {
			continue
		}

		txInfo := bson.M{
			"Number": txRes.Height,
			"Hash":   txRes.Txhash,
			"From":   from,
			"To":     to,
			"Amount": amount,
			"Events": txRes.Events,
			"Status": txRes.Code,
			"Ts":     txRes.Timestamp,
		}

		err = is.sendTxsToStorage(txInfo)
		if err != nil {
			return err
		}
	}

	err = is.rewriteLastHandledBlock(is.currentBlock)

	return err
}

func (is *InternalService) sendTxsToStorage(tx bson.M) error {
	err, _ := is.storage.Put(is.config.CollectionName, tx)

	return err
}
