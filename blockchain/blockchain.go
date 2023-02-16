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
		err := item.Value(func (val []byte) error {
			lastHash = val
			return nil
		})
		return err
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

func (chain *BlockChain) AddBlock(transaction []*Transaction) {
	var lastHash []byte
	handler.ErrorHandler(chain.Database.View(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get([]byte("lh")))
		err := item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		return err
	}))
	newBlock := CreateBlock(transaction, lastHash)
	handler.ErrorHandler(chain.Database.Update(func(txn *badger.Txn) error {
		handler.ErrorHandler(txn.Set(newBlock.Hash, newBlock.Serialize()))
		err := txn.Set([]byte("lh"), newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	}))
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	return &BlockChainIterator{chain.LastHash, chain.Database}
}

func (iterator *BlockChainIterator) Next() *Block {
	var block *Block
	handler.ErrorHandler(iterator.Database.View(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get(iterator.CurrentHash))
		var encodedBlock []byte
		err := item.Value(func(val []byte) error{
			encodedBlock = val
			return nil
		})
		block = Deserialize(encodedBlock)
		return err
	}))
	iterator.CurrentHash = block.PrevHash
	return block
}

func (chain *BlockChain) FindUnspentTransactions(pubKeyHash []byte) []Transaction {
	var unspentTxs []Transaction
	spentTXOs := make(map[string][]int)
	iterator := chain.Iterator()
	for {
		block := iterator.Next()
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)
			Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				if out.IsLockedWithKey(pubKeyHash) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					if in.UsesKey(pubKeyHash) {
						inTxID := hex.EncodeToString(in.ID)
						spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
					}
				}
			}
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
	return unspentTxs
}

func (chain *BlockChain) FindUnspentTransactionsOutputs(pubKeyHash []byte) []TxOutput {
	var unspentTransactionsOutput []TxOutput
	unspentTransactions := chain.FindUnspentTransactions(pubKeyHash)
	for _, tx := range unspentTransactions {
		for _, out := range tx.Outputs {
			if out.IsLockedWithKey(pubKeyHash) {
				unspentTransactionsOutput = append(unspentTransactionsOutput, out)
			}
		}
	}
	return unspentTransactionsOutput
}

func (chain *BlockChain) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTxs := chain.FindUnspentTransactions(pubKeyHash)
	accumulated := 0
	Work:
	for _, tx := range unspentTxs {
		txID := hex.EncodeToString(tx.ID)
		for outIdx, out := range tx.Outputs {
			if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	return accumulated, unspentOuts
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