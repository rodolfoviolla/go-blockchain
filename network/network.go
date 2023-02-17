package network

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/rodolfoviolla/go-blockchain/blockchain"
	"github.com/rodolfoviolla/go-blockchain/handler"
	"github.com/vrecan/death/v3"
)

const (
	protocol = "tcp"
	version = 1
	commandLength = 12
	ADDRESS_CMD = "address"
	BLOCK_CMD = "block"
	INVENTORY_CMD = "inventory"
	GET_BLOCKS_CMD = "get-blocks"
	GET_DATA_CMD = "get-data"
	TRANSACTION_CMD = "transaction"
	VERSION_CMD = "version"
)

var (
	nodeAddress string
	mineAddress string
	KnownNodes = []string{"localhost:3000"}
	blocksInTransit = [][]byte{}
	memoryPool = make(map[string]blockchain.Transaction)
)

type Address struct {
	AddressList []string
}

type Block struct {
	AddressFrom string
	Block []byte
}

type GetBlocks struct {
	AddressFrom string
}

type GetData struct {
	AddressFrom string
	Type string
	ID []byte
}

type Inventory struct {
	AddressFrom string
	Type string
	Items [][]byte
}

type Transaction struct {
	AddressFrom string
	Transaction []byte
}

type Version struct {
	Version int
	BestHeight int
	AddressFrom string
}

func CmdToBytes(cmd string) []byte {
	var bytes [commandLength]byte
	for i, c := range cmd {
		bytes[i] = byte(c)
	}
	return bytes[:]
}

func BytesToCmd(bytes []byte) string {
	var cmd []byte
	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(cmd, b)
		}
	}
	return string(cmd)
}

func ExtractCmd(request []byte) []byte {
	return request[:commandLength]
}

func RequestBlocks() {
	for _, node := range KnownNodes {
		SendGetBlocks(node)
	}
}

func sendCmd(cmd string, data interface{}, address string) {
	payload := GobEncode(data)
	request := append(CmdToBytes(cmd), payload...)
	SendData(address, request)
}

func SendAddress(address string) {
	nodes := Address{KnownNodes}
	nodes.AddressList = append(nodes.AddressList, nodeAddress)
	sendCmd(ADDRESS_CMD, nodes, address)
}

func SendBlock(address string, b *blockchain.Block) {
	sendCmd(BLOCK_CMD, Block{nodeAddress, b.Serialize()}, address)
}

func SendData(address string, data []byte) {
	conn, err := net.Dial(protocol, address)
	if err != nil {
		fmt.Printf("%s is not available\n", address)
		var updatedNodes []string
		for _, node := range KnownNodes {
			if node != address {
				updatedNodes = append(updatedNodes, node)
			}
		}
		KnownNodes = updatedNodes
		return
	}
	defer conn.Close()
	handler.ErrorHandler(io.Copy(conn, bytes.NewReader(data)))
}

func SendInventory(address, kind string, items [][]byte) {
	sendCmd(INVENTORY_CMD, Inventory{nodeAddress, kind, items}, address)
}

func SendGetBlocks(address string) {
	sendCmd(GET_BLOCKS_CMD, GetBlocks{nodeAddress}, address)
}

func SendGetData(address, kind string, id []byte) {
	sendCmd(GET_DATA_CMD, GetData{nodeAddress, kind, id}, address)
}

func SendTransaction(address string, transaction *blockchain.Transaction) {
	sendCmd(TRANSACTION_CMD, Transaction{nodeAddress, transaction.Serialize()}, address)
}

func SendVersion(address string, chain *blockchain.BlockChain) {
	bestHeight := chain.GetBestHeight()
	sendCmd(VERSION_CMD, Version{version, bestHeight, nodeAddress}, address)
}

func getDecodedPayload[T interface{}](request []byte) T {
	var buff bytes.Buffer
	var payload T
	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	handler.ErrorHandler(dec.Decode(&payload))
	return payload
}

func HandleAddress(request []byte) {
	payload := getDecodedPayload[Address](request)
	KnownNodes = append(KnownNodes, payload.AddressList...)
	fmt.Printf("There are %d known nodes\n", len(KnownNodes))
	RequestBlocks()
}

func HandleBlock(request []byte, chain*blockchain.BlockChain) {
	payload := getDecodedPayload[Block](request)
	blockData := payload.Block
	block := blockchain.Deserialize(blockData)
	fmt.Println("Received a new block!")
	chain.AddBlock(block)
	fmt.Printf("Added block %x\n", block.Hash)
	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddressFrom, BLOCK_CMD, blockHash)
		blocksInTransit = blocksInTransit[1:]
	} else {
		unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
		unspentTxOutputsSet.ReIndex()
	}
}

func HandleInventory(request []byte) {
	payload := getDecodedPayload[Inventory](request)
	fmt.Printf("Received inventory with %d %s\n", len(payload.Items), payload.Type)
	if payload.Type == BLOCK_CMD {
		blocksInTransit = payload.Items
		blockHash := payload.Items[0]
		SendGetData(payload.AddressFrom, BLOCK_CMD, blockHash)
		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if !bytes.Equal(b, blockHash) {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}
	if payload.Type == TRANSACTION_CMD {
		txID := payload.Items[0]
		if memoryPool[hex.EncodeToString(txID)].ID == nil {
			SendGetData(payload.AddressFrom, TRANSACTION_CMD, txID)
		}
	}
}

func HandleGetBlocks(request []byte, chain*blockchain.BlockChain) {
	payload := getDecodedPayload[GetBlocks](request)
	blocks := chain.GetBlockHashes()
	SendInventory(payload.AddressFrom, BLOCK_CMD, blocks)
}

func HandleGetData(request []byte, chain*blockchain.BlockChain) {
	payload := getDecodedPayload[GetData](request)
	if payload.Type == BLOCK_CMD {
		block := handler.ErrorHandler(chain.GetBlock([]byte(payload.ID)))
		SendBlock(payload.AddressFrom, &block)
	}
	if payload.Type == TRANSACTION_CMD {
		txID := hex.EncodeToString(payload.ID)
		tx := memoryPool[txID]
		SendTransaction(payload.AddressFrom, &tx)
	}
}

func HandleTransaction(request []byte, chain*blockchain.BlockChain) {
	payload := getDecodedPayload[Transaction](request)
	txData := payload.Transaction
	tx := blockchain.DeserializeTransaction(txData)
	memoryPool[hex.EncodeToString(tx.ID)] = tx
	fmt.Printf("%s, %d\n", nodeAddress, len(memoryPool))
	if nodeAddress == KnownNodes[0] {
		for _, node := range KnownNodes {
			if node != nodeAddress && node != payload.AddressFrom {
				SendInventory(node, TRANSACTION_CMD, [][]byte{tx.ID})
			}
		}
	} else {
		fmt.Println(len(memoryPool), len(mineAddress))
		if len(memoryPool) >= 2 && len(mineAddress) > 0 {
			MineTransaction(chain)
		}
	}
}

func MineTransaction(chain *blockchain.BlockChain) {
	var transactions []*blockchain.Transaction
	for id := range memoryPool {
		fmt.Printf("Transaction: %s\n", memoryPool[id].ID)
		tx := memoryPool[id]
		if chain.VerifyTransaction(&tx) {
			transactions = append(transactions, &tx)
		}
	}
	if len(transactions) == 0 {
		fmt.Println("All Transactions are valid")
		return
	}
	cbTx := blockchain.CoinbaseTx(mineAddress, "")
	transactions = append(transactions, cbTx)
	newBlock := chain.MineBlock(transactions)
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	unspentTxOutputsSet.ReIndex()
	fmt.Println("New block mined")
	for _, tx := range transactions {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}
	for _, node := range KnownNodes {
		if node != nodeAddress {
			SendInventory(node, BLOCK_CMD, [][]byte{newBlock.Hash})
		}
	}
	if len(memoryPool) > 0 {
		MineTransaction(chain)
	}
}

func HandleVersion(request []byte, chain*blockchain.BlockChain) {
	payload := getDecodedPayload[Version](request)
	bestHeight := chain.GetBestHeight()
	otherHeight := payload.BestHeight
	if bestHeight < otherHeight {
		SendGetBlocks(payload.AddressFrom)
	} else if bestHeight > otherHeight {
		SendVersion(payload.AddressFrom, chain)
	}
	if !NodeIsKnown(payload.AddressFrom) {
		KnownNodes = append(KnownNodes, payload.AddressFrom)
	}
}

func HandleConnection(conn net.Conn, chain *blockchain.BlockChain) {
	defer conn.Close()
	req := handler.ErrorHandler(io.ReadAll(conn))
	command := BytesToCmd(req[:commandLength])
	fmt.Printf("Received %s command\n", command)
	switch command {
			case ADDRESS_CMD: HandleAddress(req)
			case BLOCK_CMD: HandleBlock(req, chain)
			case INVENTORY_CMD: HandleInventory(req)
			case GET_BLOCKS_CMD: HandleGetBlocks(req, chain)
			case GET_DATA_CMD: HandleGetData(req, chain)
			case TRANSACTION_CMD: HandleTransaction(req, chain)
			case VERSION_CMD: HandleVersion(req, chain)
			default: fmt.Println("Unknown command")
	}
}

func StartServer(nodeID, minerAddress string) {
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	mineAddress = minerAddress
	listener := handler.ErrorHandler(net.Listen(protocol, nodeAddress))
	defer listener.Close()
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	go CloseDB(chain)
	if nodeAddress != KnownNodes[0] {
		SendVersion(KnownNodes[0], chain)
	}
	for {
		conn := handler.ErrorHandler(listener.Accept())
		go HandleConnection(conn, chain)
	}
}

func GobEncode(data interface{}) []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	handler.ErrorHandler(enc.Encode(data))
	return buff.Bytes()
}

func NodeIsKnown(address string) bool {
	for _, node := range KnownNodes {
		if node == address {
			return true
		}
	}
	return false
}

func CloseDB(chain *blockchain.BlockChain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}

