package internal

import (
	"net/http"

	"github.com/saiset-co/saiService"
)

func (is *InternalService) NewHandler() saiService.Handler {
	return saiService.Handler{
		"add_wallet": saiService.HandlerElement{
			Name:        "add_wallet",
			Description: "Add new wallet for scan transactions",
			Function: func(data, meta interface{}) (interface{}, int, error) {
				return is.addWallet(data)
			},
		},
		"delete_wallet": saiService.HandlerElement{
			Name:        "delete_wallet",
			Description: "Delete wallet from wallets list",
			Function: func(data, meta interface{}) (interface{}, int, error) {
				return is.deleteWallet(data)
			},
		},
	}
}

func (is *InternalService) addWallet(data interface{}) (string, int, error) {
	wallet, ok := data.(string)
	if !ok {
		return "wallet should be a string", http.StatusBadRequest, nil
	}
	_, exist := is.wallets[wallet]
	if exist {
		return "addWallet", http.StatusOK, nil
	}

	is.mu.Lock()
	is.wallets[wallet] = struct{}{}
	is.mu.Unlock()

	err := is.rewriteWalletsFile()
	if err != nil {
		is.mu.Lock()
		delete(is.wallets, wallet)
		is.mu.Unlock()
		return "addWallet", http.StatusInternalServerError, err
	}

	return "addWallet", http.StatusOK, nil
}

func (is *InternalService) deleteWallet(data interface{}) (string, int, error) {
	wallet, ok := data.(string)
	if !ok {
		return "wallet should be a string", http.StatusBadRequest, nil
	}

	_, exist := is.wallets[wallet]
	if !exist {
		return "deleteWallet", http.StatusOK, nil
	}

	is.mu.Lock()
	delete(is.wallets, wallet)
	is.mu.Unlock()

	err := is.rewriteWalletsFile()
	if err != nil {
		return "deleteWallet", http.StatusInternalServerError, err
	}

	return "deleteWallet", http.StatusOK, nil
}
