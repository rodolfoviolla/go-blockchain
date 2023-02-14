package blockchain

import (
	"fmt"

	"github.com/dgraph-io/badger/v3"
)

const dbPath = "./tmp/blocks"

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database *badger.DB
}

func InitBlockChain() *BlockChain {
	var lastHash []byte
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil
	db, err := badger.Open(opts)
	ErrorHandler(err)
	err = db.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get([]byte("lh")); err == badger.ErrKeyNotFound {
			fmt.Println("No existing blockchain found")
			genesis := Genesis()
			err = txn.Set(genesis.Hash, genesis.Serialize())
			ErrorHandler(err)
			err = txn.Set([]byte("lh"), genesis.Hash)
			lastHash = genesis.Hash
			return err
		} else {
			item, err := txn.Get([]byte("lh"))
			ErrorHandler(err)
			err = item.Value(func (val []byte) error {
				lastHash = val
				return nil
			})
			return err
		}
	})
	ErrorHandler(err)
	return &BlockChain{lastHash, db}
}

func (chain *BlockChain) AddBlock(data string) {
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
	newBlock := CreateBlock(data, lastHash)
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

func (iterator *BlockChainIterator) Next() * Block {
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