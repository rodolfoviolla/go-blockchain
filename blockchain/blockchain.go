package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/dgraph-io/badger/v3"
	"github.com/rodolfoviolla/go-blockchain/handler"
)

const (
	dbPath = "./tmp/blocks"
	dbFile = "./tmp/blocks/MANIFEST"
	genesisData = "First Transaction from Genesis"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database *badger.DB
}

func DBExists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}
	return true
}

func ContinueBlockChain() *BlockChain {
	var lastHash []byte
	if !DBExists() {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil
	db := handler.ErrorHandler(badger.Open(opts))
	handler.ErrorHandler(db.Update(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get([]byte("lh")))
		lastHash = handler.ErrorHandler(item.ValueCopy(nil))
		return nil
	}))
	return &BlockChain{lastHash, db}
}

func InitBlockChain(address string) *BlockChain {
	var lastHash []byte
	if DBExists() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil
	db := handler.ErrorHandler(badger.Open(opts))
	handler.ErrorHandler(db.Update(func(txn *badger.Txn) error {
		coinbaseTx := CoinbaseTx(address, genesisData)
		genesis := Genesis(coinbaseTx)
		fmt.Println("Genesis created")
		handler.ErrorHandler(txn.Set(genesis.Hash, genesis.Serialize()))
		err := txn.Set([]byte("lh"), genesis.Hash)
		lastHash = genesis.Hash
		return err
	}))
	return &BlockChain{lastHash, db}
}

func (chain *BlockChain) AddBlock(transaction []*Transaction) *Block {
	var lastHash []byte
	handler.ErrorHandler(chain.Database.View(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get([]byte("lh")))
		lastHash = handler.ErrorHandler(item.ValueCopy(nil))
		return nil
	}))
	newBlock := CreateBlock(transaction, lastHash)
	handler.ErrorHandler(chain.Database.Update(func(txn *badger.Txn) error {
		handler.ErrorHandler(txn.Set(newBlock.Hash, newBlock.Serialize()))
		err := txn.Set([]byte("lh"), newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	}))
	return newBlock
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	return &BlockChainIterator{chain.LastHash, chain.Database}
}

func (iterator *BlockChainIterator) Next() *Block {
	var block *Block
	handler.ErrorHandler(iterator.Database.View(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get(iterator.CurrentHash))
		block = Deserialize(handler.ErrorHandler(item.ValueCopy(nil)))
		return nil
	}))
	iterator.CurrentHash = block.PrevHash
	return block
}

func (chain *BlockChain) FindUnspentTransactionOutputs() map[string]TxOutputs {
	unspentTxOutputs := make(map[string]TxOutputs)
	spentTXOs := make(map[string][]int)
	iterator := chain.Iterator()
	for {
		block := iterator.Next()
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)
			Outputs: for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				outs := unspentTxOutputs[txID]
				outs.Outputs = append(outs.Outputs, out)
				unspentTxOutputs[txID] = outs
			}
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
				}
			}
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
	return unspentTxOutputs
}

func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error) {
	iterator := bc.Iterator()
	for {
		block := iterator.Next()
		for _, tx := range block.Transactions {
			if bytes.Equal(tx.ID, ID) {
				return *tx, nil
			}
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
	return Transaction{}, errors.New("Transaction does not exist")
}

func (bc *BlockChain) getPreviousTransactions(tx *Transaction) (prevTXs map[string]Transaction) {
	prevTXs = make(map[string]Transaction)
	for _, in := range tx.Inputs {
		prevTX := handler.ErrorHandler(bc.FindTransaction(in.ID))
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	return
}

func (bc *BlockChain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := bc.getPreviousTransactions(tx)
	tx.Sign(privKey, prevTXs)
}

func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
	prevTXs := bc.getPreviousTransactions(tx)
	return tx.Verify(prevTXs)
}