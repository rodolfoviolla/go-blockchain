package wallet

import (
	"github.com/mr-tron/base58"
	"github.com/rodolfoviolla/go-blockchain/handler"
)

func Base58Encode(input []byte) []byte {
	return []byte(base58.Encode(input))
}

func Base58Decode(input []byte) []byte {
	decode := handler.ErrorHandler(base58.Decode(string(input[:])))
	return decode
}