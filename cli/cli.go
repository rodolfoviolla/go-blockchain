package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/rodolfoviolla/go-blockchain/blockchain"
	"github.com/rodolfoviolla/go-blockchain/color"
	"github.com/rodolfoviolla/go-blockchain/handler"
	"github.com/rodolfoviolla/go-blockchain/network"
	"github.com/rodolfoviolla/go-blockchain/wallet"
)

type CommandLine struct {}

const (
	GET_BALANCE_CMD = "get-balance"
	CREATE_BLOCKCHAIN_CMD = "create-blockchain"
	PRINT_CHAIN_CMD = "print-chain"
	SEND_CMD = "send"
	CREATE_WALLET_CMD = "create-wallet"
	LIST_ADDRESSES_CMD = "list-addresses"
	REINDEX_UTXO_CMD = "reindex-utxo"
	START_NODE_CMD = "start-node"
	ADDRESS_PARAM = "address"
	FROM_PARAM = "from"
	TO_PARAM = "to"
	AMOUNT_PARAM = "amount"
	MINE_PARAM = "mine"
	MINER_PARAM = "miner"
)

func (cli *CommandLine) printUsage() {
	fmt.Println(color.Purple + "Welcome to the blockchain CLI!" + color.Reset)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println(color.Green + "  " + GET_BALANCE_CMD + " " + color.Cyan + "-" + ADDRESS_PARAM + " " + color.Yellow + "ADDRESS           " + color.Reset + "- Gets the balance for an address")
	fmt.Println(color.Green + "  " + CREATE_BLOCKCHAIN_CMD + " " + color.Cyan + "-" + ADDRESS_PARAM + " " + color.Yellow + "ADDRESS     " + color.Reset + "- Creates a blockchain and sends genesis reward to address")
	fmt.Println(color.Green + "  " + PRINT_CHAIN_CMD + "                            " + color.Reset + "- Prints the blocks in the chain")
	fmt.Println(color.Green + "  " + SEND_CMD + " " + color.Cyan + "-" + FROM_PARAM + " " + color.Yellow + "FROM " + color.Cyan + "-" + TO_PARAM + " " + color.Yellow + "TO " + color.Cyan + "-" + AMOUNT_PARAM + " " + color.Yellow + "AMOUNT  "+ color.Cyan + "-" + MINE_PARAM + color.Reset + "- Send amount of coins")
	fmt.Println(color.Green + "  " + CREATE_WALLET_CMD + "                          " + color.Reset + "- Creates a new wallet")
	fmt.Println(color.Green + "  " + LIST_ADDRESSES_CMD + "                         " + color.Reset + "- List the addresses in our wallet file")
	fmt.Println(color.Green + "  " + REINDEX_UTXO_CMD + "                           " + color.Reset + "- Rebuilds the unspent transaction outputs set")
	fmt.Println(color.Green + "  " + START_NODE_CMD + "                           " + color.Cyan + "-" + MINER_PARAM + " " + color.Yellow + "ADDRESS " + color.Reset + "- Start a node with ID specified in NODE_ID environment variable. To enable mining, pass " + color.Cyan + "-miner " + color.Reset + "param")
}

func (cli * CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) StartNode(nodeId, minerAddress string) {
	fmt.Printf("Starting Node %s\n", nodeId)
	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address!")
		}
	}
	network.StartServer(nodeId, minerAddress)
}

func (cli *CommandLine) reIndexUnspentTxOutputs(nodeId string) {
	chain := blockchain.ContinueBlockChain(nodeId)
	defer chain.Database.Close()
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	unspentTxOutputsSet.ReIndex()
	count := unspentTxOutputsSet.CountTransactions()
	fmt.Printf(color.Green + "Done! There are " + color.Reset + "%d" + color.Green + " transactions in the unspent transaction outputs set.\n", count)
}

func (cli *CommandLine) createWallet(nodeId string) {
	wallets, _ := wallet.CreateWallets(nodeId)
	address := wallets.AddWallet()
	wallets.SaveFile(nodeId)
	fmt.Printf("New address is: " + color.Yellow + "%s\n", address)
}

func (cli *CommandLine) listAddresses(nodeId string) {
	wallets, _ := wallet.CreateWallets(nodeId)
	addresses := wallets.GetAllAddresses()
	for _, address := range addresses {
		fmt.Println(color.Yellow + address)
	}
}

func (cli *CommandLine) printChain(nodeId string) {
	chain := blockchain.ContinueBlockChain(nodeId)
	defer chain.Database.Close()
	iterator := chain.Iterator()
	for {
		block := iterator.Next()
		fmt.Println()
		fmt.Printf(color.Cyan + "Previous Hash %x\n", block.PrevHash)
		fmt.Printf("Hash          %x\n", block.Hash)
		pow := blockchain.NewProof(block)
		fmt.Printf("PoW           %s\n" + color.Reset, strconv.FormatBool(pow.Validate()))
		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
	fmt.Println()
}

func (cli *CommandLine) createBlockChain(address, nodeId string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.InitBlockChain(address, nodeId)
	defer chain.Database.Close()
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	unspentTxOutputsSet.ReIndex()
	fmt.Println(color.Green + "Finished!")
}

func (cli *CommandLine) getBalance(address, nodeId string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.ContinueBlockChain(nodeId)
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	defer chain.Database.Close()
	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-4]
	unspentTransactionsOutput := unspentTxOutputsSet.FindUnspentTransactions(pubKeyHash)
	for _, out := range unspentTransactionsOutput {
		balance += out.Value
	}
	fmt.Printf("Balance of " + color.Yellow + "%s: " + color.Green + "%d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int, nodeId string, mineNow bool) {
	if !wallet.ValidateAddress(to) {
		log.Panic("To address is not valid")
	}
	if !wallet.ValidateAddress(from) {
		log.Panic("From address is not valid")
	}
	chain := blockchain.ContinueBlockChain(nodeId)
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	defer chain.Database.Close()
	wallets := handler.ErrorHandler(wallet.CreateWallets(nodeId))
	wallet := wallets.GetWallet(from)
	transaction := blockchain.NewTransaction(&wallet, to, amount, &unspentTxOutputsSet)
	if mineNow {
		coinbaseTx := blockchain.CoinbaseTx(from, "")
		transactions := []*blockchain.Transaction{coinbaseTx, transaction}
		block := chain.MineBlock(transactions)
		unspentTxOutputsSet.Update(block)
	} else {
		network.SendTransaction(network.KnownNodes[0], transaction)
		fmt.Println("Send transaction")
	}
	fmt.Println(color.Green + "Success!")
}

func (cli *CommandLine) Run() {
	cli.validateArgs()
	nodeId := os.Getenv("NODE_ID")
	if nodeId == "" {
		fmt.Println("NODE_ID environment variable is not set!")
		runtime.Goexit()
	}
	getBalanceCmd := flag.NewFlagSet(GET_BALANCE_CMD, flag.ExitOnError)
	createBlockChainCmd := flag.NewFlagSet(CREATE_BLOCKCHAIN_CMD, flag.ExitOnError)
	sendCmd := flag.NewFlagSet(SEND_CMD, flag.ExitOnError)
	printChainCmd := flag.NewFlagSet(PRINT_CHAIN_CMD, flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet(CREATE_WALLET_CMD, flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet(LIST_ADDRESSES_CMD, flag.ExitOnError)
	reIndexUnspentTxOutputsCmd := flag.NewFlagSet(REINDEX_UTXO_CMD, flag.ExitOnError)
	startNodeCmd := flag.NewFlagSet(START_NODE_CMD, flag.ExitOnError)
	getBalanceAddress := getBalanceCmd.String(ADDRESS_PARAM, "", "The address to get balance for")
	createBlockChainAddress := createBlockChainCmd.String(ADDRESS_PARAM, "", "The address to send genesis block reward to")
	sendFrom := sendCmd.String(FROM_PARAM, "", "Source wallet address")
	sendTo := sendCmd.String(TO_PARAM, "", "Destination wallet address")
	sendAmount := sendCmd.Int(AMOUNT_PARAM, 0, "Amount to send")
	sendMine := sendCmd.Bool(MINE_PARAM, false, "Mine immediately on the same node")
	startNodeMiner := startNodeCmd.String(MINER_PARAM, "", "Enable mining mode and send reward to ADDRESS")
	switch os.Args[1] {
		case GET_BALANCE_CMD:
			handler.ErrorHandler(getBalanceCmd.Parse(os.Args[2:]))
		case CREATE_BLOCKCHAIN_CMD:
			handler.ErrorHandler(createBlockChainCmd.Parse(os.Args[2:]))
		case SEND_CMD:
			handler.ErrorHandler(sendCmd.Parse(os.Args[2:]))
		case PRINT_CHAIN_CMD:
			handler.ErrorHandler(printChainCmd.Parse(os.Args[2:]))
		case CREATE_WALLET_CMD:
			handler.ErrorHandler(createWalletCmd.Parse(os.Args[2:]))
		case LIST_ADDRESSES_CMD:
			handler.ErrorHandler(listAddressesCmd.Parse(os.Args[2:]))
		case REINDEX_UTXO_CMD:
			handler.ErrorHandler(reIndexUnspentTxOutputsCmd.Parse(os.Args[2:]))
		case START_NODE_CMD:
			handler.ErrorHandler(startNodeCmd.Parse(os.Args[2:]))
		default:
			cli.printUsage()
			runtime.Goexit()
	}
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress, nodeId)
	}
	if createBlockChainCmd.Parsed() {
		if *createBlockChainAddress == "" {
			createBlockChainCmd.Usage()
			runtime.Goexit()
		}
		cli.createBlockChain(*createBlockChainAddress, nodeId)
	}
	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}
		cli.send(*sendFrom, *sendTo, *sendAmount, nodeId, *sendMine)
	}
	if printChainCmd.Parsed() {
		cli.printChain(nodeId)
	}
	if createWalletCmd.Parsed() {
		cli.createWallet(nodeId)
	}
	if listAddressesCmd.Parsed() {
		cli.listAddresses(nodeId)
	}
	if reIndexUnspentTxOutputsCmd.Parsed() {
		cli.reIndexUnspentTxOutputs(nodeId)
	}
	if startNodeCmd.Parsed() {
		cli.StartNode(nodeId, *startNodeMiner)
	}
}