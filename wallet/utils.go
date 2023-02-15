package wallet

import (
	"github.com/mr-tron/base58"
	"github.com/rodolfoviolla/go-blockchain/blockchain"
)

func Base58Encode(input []byte) []byte {
	return []byte(base58.Encode(input))
}

func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input[:]))
	blockchain.ErrorHandler(err)
	return decode
}