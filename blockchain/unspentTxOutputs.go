package blockchain

import (
	"bytes"
	"encoding/hex"

	"github.com/dgraph-io/badger/v3"
	"github.com/rodolfoviolla/go-blockchain/handler"
)

var unspentTxOutputsPrefix = []byte("utxo-")


type UnspentTxOutputsSet struct {
	Blockchain *BlockChain
}

func (u *UnspentTxOutputsSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.Database
	handler.ErrorHandler(db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		iterator := txn.NewIterator(opts)
		defer iterator.Close()
		for iterator.Seek(unspentTxOutputsPrefix); iterator.ValidForPrefix(unspentTxOutputsPrefix); iterator.Next() {
			item := iterator.Item()
			key := bytes.TrimPrefix(item.Key(), unspentTxOutputsPrefix)
			txID := hex.EncodeToString(key)
			outs := DeserializeOutputs(handler.ErrorHandler(item.ValueCopy(nil)))
			for outIdx, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
			}
		}
		return nil
	}))
	return accumulated, unspentOuts
}

func (u *UnspentTxOutputsSet) FindUnspentTransactions(pubKeyHash []byte) []TxOutput {
	var unspentTransactionsOutput []TxOutput
	db := u.Blockchain.Database
	handler.ErrorHandler(db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		iterator := txn.NewIterator(opts)
		defer iterator.Close()
		for iterator.Seek(unspentTxOutputsPrefix); iterator.ValidForPrefix(unspentTxOutputsPrefix); iterator.Next() {
			item := iterator.Item()
			outs := DeserializeOutputs(handler.ErrorHandler(item.ValueCopy(nil)))
			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					unspentTransactionsOutput = append(unspentTransactionsOutput, out)
				}
			}
		}
		return nil
	}))
	return unspentTransactionsOutput
}

func (u UnspentTxOutputsSet) CountTransactions() int {
	db := u.Blockchain.Database
	counter := 0
	handler.ErrorHandler(db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		iterator := txn.NewIterator(opts)
		defer iterator.Close()
		for iterator.Seek(unspentTxOutputsPrefix); iterator.ValidForPrefix(unspentTxOutputsPrefix); iterator.Next() {
			counter++
		}
		return nil
	}))
	return counter
}

func (u UnspentTxOutputsSet) ReIndex() {
	db := u.Blockchain.Database
	u.DeleteByPrefix(unspentTxOutputsPrefix)
	unspentTxOutputs := u.Blockchain.FindUnspentTransactionOutputs()
	handler.ErrorHandler(db.Update(func(txn *badger.Txn) error {
		for txId, outs := range unspentTxOutputs {
			key, err := hex.DecodeString(txId)
			if err != nil {
				return err
			}
			key = append(unspentTxOutputsPrefix, key...)
			handler.ErrorHandler(txn.Set(key, outs.Serialize()))
		}
		return nil
	}))
}

func (u *UnspentTxOutputsSet) Update(block *Block) {
	db := u.Blockchain.Database
	handler.ErrorHandler(db.Update(func(txn *badger.Txn) error {
		for _, tx := range block.Transactions {
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					updatedOuts := TxOutputs{}
					inID := append(unspentTxOutputsPrefix, in.ID...)
					item := handler.ErrorHandler(txn.Get(inID))					
					outs := DeserializeOutputs(handler.ErrorHandler(item.ValueCopy(nil)))
					for outIdx, out := range outs.Outputs {
						if outIdx != in.Out {
							updatedOuts.Outputs = append(updatedOuts.Outputs, out)
						}
					}
					if len(updatedOuts.Outputs) == 0 {
						handler.ErrorHandler(txn.Delete(inID))
					} else {
						handler.ErrorHandler(txn.Set(inID, updatedOuts.Serialize()))
					}
				}
			}
			newOutputs := TxOutputs{}
			newOutputs.Outputs = append(newOutputs.Outputs, tx.Outputs...)
			txID := append(unspentTxOutputsPrefix, tx.ID...)
			handler.ErrorHandler(txn.Set(txID, newOutputs.Serialize()))
		}
		return nil
	}))
}

func (unspentTxOutputs *UnspentTxOutputsSet) DeleteByPrefix(prefix []byte) {
	deleteKeys := func(keysForDelete [][]byte) error {
		if err := unspentTxOutputs.Blockchain.Database.Update(func (txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return nil
		}
		return nil
	}
	collectSize := 100000
	unspentTxOutputs.Blockchain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iterator := txn.NewIterator(opts)
		defer iterator.Close()
		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0
		for iterator.Seek(prefix); iterator.ValidForPrefix(prefix); iterator.Next() {
			key := iterator.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++
			if keysCollected == collectSize {
				handler.ErrorHandler(deleteKeys(keysForDelete))
				keysForDelete = make([][]byte, 0, collectSize)
				keysCollected = 0
			}
		}
		if keysCollected > 0 {
			handler.ErrorHandler(deleteKeys(keysForDelete))
		}
		return nil
	})
}