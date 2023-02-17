package blockchain

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/rodolfoviolla/go-blockchain/handler"
)


type BlockChainIterator struct {
	CurrentHash []byte
	Database *badger.DB
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