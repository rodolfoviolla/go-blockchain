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
	"github.com/rodolfoviolla/go-blockchain/wallet"
)

type CommandLine struct {}

const (
	GET_BALANCE = "get-balance"
	CREATE_BLOCKCHAIN = "create-blockchain"
	PRINT_CHAIN = "print-chain"
	SEND = "send"
	CREATE_WALLET = "create-wallet"
	LIST_ADDRESSES = "list-addresses"
	REINDEX_UTXO = "reindex-utxo"
	ADDRESS_PARAM = "address"
	FROM_PARAM = "from"
	TO_PARAM = "to"
	AMOUNT_PARAM = "amount"
)

func (cli *CommandLine) printUsage() {
	fmt.Println(color.Purple + "Welcome to the blockchain CLI!" + color.Reset)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println(color.Green + "  " + GET_BALANCE + " " + color.Cyan + "-" + ADDRESS_PARAM + " " + color.Yellow + "ADDRESS           " + color.Reset + "- Gets the balance for an address")
	fmt.Println(color.Green + "  " + CREATE_BLOCKCHAIN + " " + color.Cyan + "-" + ADDRESS_PARAM + " " + color.Yellow + "ADDRESS     " + color.Reset + "- Creates a blockchain and sends genesis reward to address")
	fmt.Println(color.Green + "  " + PRINT_CHAIN + "                            " + color.Reset + "- Prints the blocks in the chain")
	fmt.Println(color.Green + "  " + SEND + " " + color.Cyan + "-" + FROM_PARAM + " " + color.Yellow + "FROM " + color.Cyan + "-" + TO_PARAM + " " + color.Yellow + "TO " + color.Cyan + "-" + AMOUNT_PARAM + " " + color.Yellow + "AMOUNT  " + color.Reset + "- Send amount of coins")
	fmt.Println(color.Green + "  " + CREATE_WALLET + "                          " + color.Reset + "- Creates a new wallet")
	fmt.Println(color.Green + "  " + LIST_ADDRESSES + "                         " + color.Reset + "- List the addresses in our wallet file")
	fmt.Println(color.Green + "  " + REINDEX_UTXO + "                           " + color.Reset + "- Rebuilds the unspent transaction outputs set")
}

func (cli * CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) reIndexUnspentTxOutputs() {
	chain := blockchain.ContinueBlockChain()
	defer chain.Database.Close()
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	unspentTxOutputsSet.ReIndex()
	count := unspentTxOutputsSet.CountTransactions()
	fmt.Printf(color.Green + "Done! There are " + color.Reset + "%d" + color.Green + " transactions in the unspent transaction outputs set.\n", count)
}

func (cli *CommandLine) createWallet() {
	wallets, _ := wallet.CreateWallets()
	address := wallets.AddWallet()
	wallets.SaveFile()
	fmt.Printf("New address is: " + color.Yellow + "%s\n", address)
}

func (cli *CommandLine) listAddresses() {
	wallets, _ := wallet.CreateWallets()
	addresses := wallets.GetAllAddresses()
	for _, address := range addresses {
		fmt.Println(color.Yellow + address)
	}
}

func (cli *CommandLine) printChain() {
	chain := blockchain.ContinueBlockChain()
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

func (cli *CommandLine) createBlockChain(address string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.InitBlockChain(address)
	defer chain.Database.Close()
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	unspentTxOutputsSet.ReIndex()
	fmt.Println(color.Green + "Finished!")
}

func (cli *CommandLine) getBalance(address string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.ContinueBlockChain()
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

func (cli *CommandLine) send(from, to string, amount int) {
	if !wallet.ValidateAddress(to) {
		log.Panic("To address is not valid")
	}
	if !wallet.ValidateAddress(from) {
		log.Panic("From address is not valid")
	}
	chain := blockchain.ContinueBlockChain()
	unspentTxOutputsSet := blockchain.UnspentTxOutputsSet{Blockchain: chain}
	defer chain.Database.Close()
	tx := blockchain.NewTransaction(from, to, amount, &unspentTxOutputsSet)
	coinbaseTx := blockchain.CoinbaseTx(from, "")
	block := chain.AddBlock([]*blockchain.Transaction{coinbaseTx, tx})
	unspentTxOutputsSet.Update(block)
	fmt.Println(color.Green + "Success!")
}

func (cli *CommandLine) Run() {
	cli.validateArgs()
	getBalanceCmd := flag.NewFlagSet(GET_BALANCE, flag.ExitOnError)
	createBlockChainCmd := flag.NewFlagSet(CREATE_BLOCKCHAIN, flag.ExitOnError)
	sendCmd := flag.NewFlagSet(SEND, flag.ExitOnError)
	printChainCmd := flag.NewFlagSet(PRINT_CHAIN, flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet(CREATE_WALLET, flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet(LIST_ADDRESSES, flag.ExitOnError)
	reIndexUnspentTxOutputsCmd := flag.NewFlagSet(REINDEX_UTXO, flag.ExitOnError)
	getBalanceAddress := getBalanceCmd.String(ADDRESS_PARAM, "", "The address to get balance for")
	createBlockChainAddress := createBlockChainCmd.String(ADDRESS_PARAM, "", "The address to send genesis block reward to")
	sendFrom := sendCmd.String(FROM_PARAM, "", "Source wallet address")
	sendTo := sendCmd.String(TO_PARAM, "", "Destination wallet address")
	sendAmount := sendCmd.Int(AMOUNT_PARAM, 0, "Amount to send")
	switch os.Args[1] {
		case GET_BALANCE:
			handler.ErrorHandler(getBalanceCmd.Parse(os.Args[2:]))
		case CREATE_BLOCKCHAIN:
			handler.ErrorHandler(createBlockChainCmd.Parse(os.Args[2:]))
		case SEND:
			handler.ErrorHandler(sendCmd.Parse(os.Args[2:]))
		case PRINT_CHAIN:
			handler.ErrorHandler(printChainCmd.Parse(os.Args[2:]))
		case CREATE_WALLET:
			handler.ErrorHandler(createWalletCmd.Parse(os.Args[2:]))
		case LIST_ADDRESSES:
			handler.ErrorHandler(listAddressesCmd.Parse(os.Args[2:]))
		case REINDEX_UTXO:
			handler.ErrorHandler(reIndexUnspentTxOutputsCmd.Parse(os.Args[2:]))
		default:
			cli.printUsage()
			runtime.Goexit()
	}
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress)
	}
	if createBlockChainCmd.Parsed() {
		if *createBlockChainAddress == "" {
			createBlockChainCmd.Usage()
			runtime.Goexit()
		}
		cli.createBlockChain(*createBlockChainAddress)
	}
	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}
		cli.send(*sendFrom, *sendTo, *sendAmount)
	}
	if printChainCmd.Parsed() {
		cli.printChain()
	}
	if createWalletCmd.Parsed() {
		cli.createWallet()
	}
	if listAddressesCmd.Parsed() {
		cli.listAddresses()
	}
	if reIndexUnspentTxOutputsCmd.Parsed() {
		cli.reIndexUnspentTxOutputs()
	}
}