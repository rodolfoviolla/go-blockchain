package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/rodolfoviolla/go-blockchain/color"
	"github.com/rodolfoviolla/go-blockchain/handler"
	"github.com/rodolfoviolla/go-blockchain/wallet"
)

type Transaction struct {
	ID []byte
	Inputs []TxInput
	Outputs []TxOutput
}

func (tx *Transaction) Hash() []byte {
	var hash [32]byte
	txCopy := *tx
	txCopy.ID = []byte{}
	hash = sha256.Sum256(txCopy.Serialize())
	return hash[:]
}

func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	handler.ErrorHandler(enc.Encode(tx))
	return encoded.Bytes()
}

func DeserializeTransaction(data []byte) Transaction {
	var transaction Transaction
	decoder := gob.NewDecoder(bytes.NewReader(data))
	handler.ErrorHandler(decoder.Decode(&transaction))
	return transaction
}

func CoinbaseTx(to, data string) *Transaction {
	if data == "" {
		randomData := make([]byte, 24)
		handler.ErrorHandler(rand.Read(randomData))
		data = fmt.Sprintf("%x", randomData)
	}
	txIn := TxInput{[]byte{}, -1, nil, []byte(data)}
	txOut := *NewTXOutput(20, to)
	tx := Transaction{nil, []TxInput{txIn}, []TxOutput{txOut}}
	tx.ID = tx.Hash()
	return &tx
}

func NewTransaction(w *wallet.Wallet, to string, amount int, unspentTxOutputs *UnspentTxOutputsSet) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput
	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)
	acc, validOutputs := unspentTxOutputs.FindSpendableOutputs(pubKeyHash, amount)
	if acc < amount {
		log.Panic("Error: not enough funds")
	}
	for encodedTxID, outs := range validOutputs {
		txID := handler.ErrorHandler(hex.DecodeString(encodedTxID))
		for _, out := range outs {
			input := TxInput{txID, out, nil, w.PublicKey}
			inputs = append(inputs, input)
		}
	}
	from := string(w.Address())
	outputs = append(outputs, *NewTXOutput(amount, to))
	if acc > amount {
		outputs = append(outputs, *NewTXOutput(acc - amount, from))
	}
	tx := Transaction{nil, inputs, outputs}
	tx.ID = tx.Hash()
	unspentTxOutputs.Blockchain.SignTransaction(&tx, w.PrivateKey)
	return &tx
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].ID) == 0 && tx.Inputs[0].Out == -1
}

func hashTransactionID(tx *Transaction, prevTXs map[string]Transaction, in TxInput, inId int) {
	prevTX := prevTXs[hex.EncodeToString(in.ID)]
	tx.Inputs[inId].Signature = nil
	tx.Inputs[inId].PubKey = prevTX.Outputs[in.Out].PubKeyHash
	tx.ID = tx.Hash()
	tx.Inputs[inId].PubKey = nil
}

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}
	txCopy := tx.TrimmedCopy()
	for inId, in := range txCopy.Inputs {
		hashTransactionID(&txCopy, prevTXs, in, inId)
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, txCopy.ID)
		handler.ErrorHandler(err)
		signature := append(r.Bytes(), s.Bytes()...)
		tx.Inputs[inId].Signature = signature
	}
}

func formatBytes(field []byte) (big.Int, big.Int) {
	x := big.Int{}
	y := big.Int{}
	fieldLen := len(field)
	x.SetBytes(field[:fieldLen/2])
	y.SetBytes(field[fieldLen/2:])
	return x, y
}

func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("Previous transaction does not exist")
		}
	}
	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()
	for inId, in := range tx.Inputs {
		hashTransactionID(&txCopy, prevTXs, in, inId)
		r, s := formatBytes(in.Signature)
		x, y := formatBytes(in.PubKey)
		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}
		if !ecdsa.Verify(&rawPubKey, txCopy.ID, &r, &s) {
			return false
		}
	}
	return true
}

func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TxInput
	var outputs []TxOutput
	for _, in := range tx.Inputs {
		inputs = append(inputs, TxInput{in.ID, in.Out, nil, nil})
	}
	for _, out := range tx.Outputs {
		outputs = append(outputs, TxOutput{out.Value, out.PubKeyHash})
	}
	return Transaction{tx.ID, inputs, outputs}
}

func (tx Transaction) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Transaction   %x", tx.ID))
	for i, input := range tx.Inputs {
		lines = append(lines, fmt.Sprintf(color.Green + "  Input       %d", i))
		lines = append(lines, fmt.Sprintf("    TxID      %x", input.ID))
		lines = append(lines, fmt.Sprintf("    Out       %d", input.Out))
		lines = append(lines, fmt.Sprintf("    Signature %x", input.Signature))
		lines = append(lines, fmt.Sprintf("    PubKey    %x" + color.Reset, input.PubKey))
	}
	for i, output := range tx.Outputs {
		lines = append(lines, fmt.Sprintf(color.Red + "  Output      %d", i))
		lines = append(lines, fmt.Sprintf("    Value     %d", output.Value))
		lines = append(lines, fmt.Sprintf("    Script    %x" + color.Reset, output.PubKeyHash))
	}
	return strings.Join(lines, "\n")
}