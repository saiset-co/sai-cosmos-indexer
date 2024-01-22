package internal

import (
	"os"

	jsoniter "github.com/json-iterator/go"
)

func (is *InternalService) loadWallets() error {
	fileBytes, err := os.ReadFile(filePathWallets)
	if err != nil {
		return err
	}

	var walletsArray []string
	err = jsoniter.Unmarshal(fileBytes, &walletsArray)
	if err != nil {
		return err
	}

	is.mu.Lock()
	for _, wallet := range walletsArray {
		is.wallets[wallet] = struct{}{}
	}
	is.mu.Unlock()

	return nil
}

func (is *InternalService) rewriteWalletsFile() error {
	walletsArray := make([]string, len(is.wallets))
	is.mu.Lock()
	i := 0
	for k, _ := range is.wallets {
		walletsArray[i] = k
		i++
	}
	is.mu.Unlock()

	walletsBytes, err := jsoniter.Marshal(&walletsArray)
	if err != nil {
		return err
	}

	err = os.WriteFile(filePathWallets, walletsBytes, os.ModePerm)

	return err
}
