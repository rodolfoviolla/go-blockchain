package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dgraph-io/badger/v3"
	"github.com/rodolfoviolla/go-blockchain/handler"
)

const (
	dbPath = "./tmp/blocks_%s"
	genesisData = "First Transaction from Genesis"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

func DBExists(path string) bool {
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}
	return true
}

func ContinueBlockChain(nodeId string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeId)
	if !DBExists(path) {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}
	var lastHash []byte
	opts := badger.DefaultOptions(path)
	opts.Logger = nil
	db := handler.ErrorHandler(openDB(path, opts))
	handler.ErrorHandler(db.Update(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get([]byte("lh")))
		lastHash = handler.ErrorHandler(item.ValueCopy(nil))
		return nil
	}))
	return &BlockChain{lastHash, db}
}

func InitBlockChain(address, nodeId string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeId)
	if DBExists(path) {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}
	var lastHash []byte
	opts := badger.DefaultOptions(path)
	opts.Logger = nil
	db := handler.ErrorHandler(openDB(path, opts))
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

func (chain *BlockChain) AddBlock(block *Block) {
	handler.ErrorHandler(chain.Database.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(block.Hash); err == nil {
			return nil
		}
		blockData := block.Serialize()
		handler.ErrorHandler(txn.Set(block.Hash, blockData))
		item := handler.ErrorHandler(txn.Get([]byte("lh")))
		lastHash, _ := item.ValueCopy(nil)
		item = handler.ErrorHandler(txn.Get(lastHash))
		lastBlockData, _ := item.ValueCopy(nil)
		lastBlock := Deserialize(lastBlockData)
		if block.Height > lastBlock.Height {
			handler.ErrorHandler(txn.Set([]byte("lh"), block.Hash))
			chain.LastHash = block.Hash
		}
		return nil
	}))
}

func (chain *BlockChain) GetBlock(blockHash []byte) (Block, error) {
	var block Block
	err	 := chain.Database.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(blockHash); err != nil {
			return errors.New("Block is not found")
		} else {
			blockData, _ := item.ValueCopy(nil)
			block = *Deserialize(blockData)
		}
		return nil
	})
	return block, err
}

func (chain *BlockChain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	iterator := chain.Iterator()
	for {
		block := iterator.Next()
		blocks = append(blocks, block.Hash)
		if len (block.PrevHash) == 0 {
			break
		}
	}
	return blocks
}

func (chain *BlockChain) GetBestHeight() int {
	var lastBlock Block
	handler.ErrorHandler(chain.Database.View(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get([]byte("lh")))
		lastHash, _ := item.ValueCopy(nil)
		item = handler.ErrorHandler(txn.Get(lastHash))
		lastBlockData, _ := item.ValueCopy(nil)
		lastBlock = *Deserialize(lastBlockData)
		return nil
	}))
	return lastBlock.Height
}

func (chain *BlockChain) MineBlock(transaction []*Transaction) *Block {
	var lastHash []byte
	var lastHeight int
	handler.ErrorHandler(chain.Database.View(func(txn *badger.Txn) error {
		item := handler.ErrorHandler(txn.Get([]byte("lh")))
		lastHash = handler.ErrorHandler(item.ValueCopy(nil))
		item = handler.ErrorHandler(txn.Get(lastHash))
		lastBlockData, _ := item.ValueCopy(nil)
		lastBlock := Deserialize(lastBlockData)
		lastHeight = lastBlock.Height
		return nil
	}))
	newBlock := CreateBlock(transaction, lastHash, lastHeight+1)
	handler.ErrorHandler(chain.Database.Update(func(txn *badger.Txn) error {
		handler.ErrorHandler(txn.Set(newBlock.Hash, newBlock.Serialize()))
		err := txn.Set([]byte("lh"), newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	}))
	return newBlock
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
	if tx.IsCoinbase() {
		return true
	}
	prevTXs := bc.getPreviousTransactions(tx)
	return tx.Verify(prevTXs)
}

func retry(dir string, originalOpts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK")
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK": %s`, err)
	}
	return badger.Open(originalOpts)
}

func openDB(dir string, opts badger.Options) (*badger.DB, error) {
	if db, err := badger.Open(opts); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err := retry(dir, opts); err != nil {
				log.Println("Database unlocked, value log truncated")
				return db, err
			}
			log.Println("Could not unlock database:", err)
		}
		return nil, err
	} else {
		return db, nil
	}
}