package wallet

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"os"

	"github.com/rodolfoviolla/go-blockchain/handler"
)

const walletFile = "./tmp/wallets_%s.data"

type Wallets struct {
	Wallets map[string]*Wallet
}

func CreateWallets(nodeId string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)
	return &wallets, wallets.LoadFile(nodeId)
}

func (ws *Wallets) AddWallet() string {
	wallet := MakeWallet()
	address := string(wallet.Address())
	ws.Wallets[address] = wallet
	return address
}

func (ws *Wallets) GetAllAddresses() []string {
	var addresses []string
	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}
	return addresses
}

func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}

func (ws *Wallets) LoadFile(nodeId string) error {
	walletFile := fmt.Sprintf(walletFile, nodeId)
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return err
	}
	var wallets Wallets
	fileContent, err := os.ReadFile(walletFile)
	if err != nil {
		return err
	}
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	if err != nil {
		return err
	}
	ws.Wallets = wallets.Wallets
	return nil
}

func (ws *Wallets) SaveFile(nodeId string) {
	var content bytes.Buffer
	walletFile := fmt.Sprintf(walletFile, nodeId)
	gob.Register(elliptic.P256())
	encoder := gob.NewEncoder(&content)
	handler.ErrorHandler(encoder.Encode(ws))
	handler.ErrorHandler(os.WriteFile(walletFile, content.Bytes(), 0644))
}