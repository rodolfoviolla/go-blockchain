package blockchain

import (
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/dgraph-io/badger/v3"
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
	db, err := badger.Open(opts)
	ErrorHandler(err)
	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		ErrorHandler(err)
		err = item.Value(func (val []byte) error {
			lastHash = val
			return nil
		})
		return err
	})
	ErrorHandler(err)
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
	db, err := badger.Open(opts)
	ErrorHandler(err)
	err = db.Update(func(txn *badger.Txn) error {
		coinbaseTx := CoinbaseTx(address, genesisData)
		genesis := Genesis(coinbaseTx)
		fmt.Println("Genesis created")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		ErrorHandler(err)
		err = txn.Set([]byte("lh"), genesis.Hash)
		lastHash = genesis.Hash
		return err
	})
	ErrorHandler(err)
	return &BlockChain{lastHash, db}
}

func (chain *BlockChain) AddBlock(transaction []*Transaction) {
	var lastHash []byte
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		ErrorHandler(err)
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		return err
	})
	ErrorHandler(err)
	newBlock := CreateBlock(transaction, lastHash)
	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		ErrorHandler(err)
		err = txn.Set([]byte("lh"), newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	})
	ErrorHandler(err)
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	return &BlockChainIterator{chain.LastHash, chain.Database}
}

func (iterator *BlockChainIterator) Next() *Block {
	var block *Block
	err := iterator.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iterator.CurrentHash)
		ErrorHandler(err)
		var encodedBlock []byte
		err = item.Value(func(val []byte) error{
			encodedBlock = val
			return nil
		})
		block = Deserialize(encodedBlock)
		return err
	})
	ErrorHandler(err)
	iterator.CurrentHash = block.PrevHash
	return block
}

func (chain *BlockChain) FindUnspentTransactions(address string) []Transaction {
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
				if out.CanBeUnlocked(address) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					if in.CanUnlock(address) {
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

func (chain *BlockChain) FindUnspentTransactionsOutputs(address string) []TxOutput {
	var unspentTransactionsOutput []TxOutput
	unspentTransactions := chain.FindUnspentTransactions(address)
	for _, tx := range unspentTransactions {
		for _, out := range tx.Outputs {
			if out.CanBeUnlocked(address) {
				unspentTransactionsOutput = append(unspentTransactionsOutput, out)
			}
		}
	}
	return unspentTransactionsOutput
}

func (chain *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTxs := chain.FindUnspentTransactions(address)
	accumulated := 0
	Work:
	for _, tx := range unspentTxs {
		txID := hex.EncodeToString(tx.ID)
		for outIdx, out := range tx.Outputs {
			if out.CanBeUnlocked(address) && accumulated < amount {
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