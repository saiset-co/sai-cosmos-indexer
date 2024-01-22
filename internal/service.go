package internal

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/saiset-co/saiCosmosIndexer/internal/model"
	"github.com/saiset-co/saiService"
	"github.com/saiset-co/saiStorageUtil"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	filePathAddresses     = "./addresses.json"
	filePathServiceConfig = "./service_config.json"
	filePathLatestBlock   = "./latest_handled_block"
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
	is.mu = &sync.Mutex{}
	is.client = http.Client{
		Timeout: 5 * time.Second,
	}
	is.addresses = make(map[string]struct{})

	fileBytes, err := os.ReadFile(filePathServiceConfig)
	if err != nil {
		log.Fatal(err)
	}

	err = jsoniter.Unmarshal(fileBytes, &is.config)
	if err != nil {
		log.Fatal(err)
	}

	is.storage = saiStorageUtil.Storage(is.config.Storage.URL, is.config.Storage.Email, is.config.Storage.Token)

	err = is.loadAddresses()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println(err)
	}

	fileBytes, err = os.ReadFile(filePathLatestBlock)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Println(err)
		}
	} else {
		latestHandledBlock, err := strconv.Atoi(string(fileBytes))
		if err != nil {
			log.Println(err)
		}

		is.currentBlock = int64(latestHandledBlock)
	}

	if is.currentBlock < is.config.StartBlock {
		is.currentBlock = is.config.StartBlock
	}
}

func (is *InternalService) Process() {
	for {
		select {
		case <-is.Context.Context.Done():
			log.Println("saiCosmosIndexer loop is done")
			return
		default:
			if len(is.addresses) == 0 {
				time.Sleep(time.Second * time.Duration(is.config.SleepDuration))
				continue
			}

			latestBlockHeight, err := is.getLatestBlock()
			if err != nil {
				log.Println(err)
				time.Sleep(time.Second * time.Duration(is.config.SleepDuration))
				continue
			}

			if is.currentBlock >= latestBlockHeight {
				time.Sleep(time.Second * time.Duration(is.config.SleepDuration))
				continue
			}

			err = is.handleBlockTxs()
			if err != nil {
				log.Println(err)
				time.Sleep(time.Second * time.Duration(is.config.SleepDuration))
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
	err, _ := is.storage.Put(is.config.Storage.CollectionName, tx)

	return err
}
