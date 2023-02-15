package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/rodolfoviolla/go-blockchain/blockchain"
	"github.com/rodolfoviolla/go-blockchain/wallet"
)

type CommandLine struct {}

func (cli *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" get-balance -address ADDRESS           - Gets the balance for an address")
	fmt.Println(" create-blockchain -address ADDRESS     - Creates a blockchain and sends genesis reward to address")
	fmt.Println(" print-chain                            - Prints the blocks in the chain")
	fmt.Println(" send - from FROM -to TO -amount AMOUNT - Send amount of coins")
	fmt.Println(" create-wallet                          - Creates a new wallet")
	fmt.Println(" list-addresses                         - List the addresses in our wallet file")
}

func (cli * CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) createWallet() {
	wallets, _ := wallet.CreateWallets()
	address := wallets.AddWallet()
	wallets.SaveFile()
	fmt.Printf("New address is: %s\n", address)
}

func (cli *CommandLine) listAddresses() {
	wallets, _ := wallet.CreateWallets()
	addresses := wallets.GetAllAddresses()
	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CommandLine) printChain() {
	chain := blockchain.ContinueBlockChain()
	defer chain.Database.Close()
	iterator := chain.Iterator()
	for {
		block := iterator.Next()
		fmt.Println()
		fmt.Printf("Previous Hash: %x\n", block.PrevHash)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
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
	chain.Database.Close()
	fmt.Println("Finished!")
}

func (cli *CommandLine) getBalance(address string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.ContinueBlockChain()
	defer chain.Database.Close()
	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1:len(pubKeyHash)-4]
	unspentTransactionsOutput := chain.FindUnspentTransactionsOutputs(pubKeyHash)
	for _, out := range unspentTransactionsOutput {
		balance += out.Value
	}
	fmt.Printf("Balance of %s: %d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int) {
	if !wallet.ValidateAddress(to) {
		log.Panic("To address is not valid")
	}
	if !wallet.ValidateAddress(from) {
		log.Panic("From address is not valid")
	}
	chain := blockchain.ContinueBlockChain()
	defer chain.Database.Close()
	tx := blockchain.NewTransaction(from, to, amount, chain)
	chain.AddBlock([]*blockchain.Transaction{tx})
	fmt.Println("Success!")
}

func (cli *CommandLine) Run() {
	cli.validateArgs()
	getBalanceCmd := flag.NewFlagSet("get-balance", flag.ExitOnError)
	createBlockChainCmd := flag.NewFlagSet("create-blockchain", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("print-chain", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("create-wallet", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("list-addresses", flag.ExitOnError)
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	createBlockChainAddress := createBlockChainCmd.String("address", "", "The address to send genesis block reward to")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
	switch os.Args[1] {
		case "get-balance":
			err := getBalanceCmd.Parse(os.Args[2:])
			blockchain.ErrorHandler(err)
		case "create-blockchain":
			err := createBlockChainCmd.Parse(os.Args[2:])
			blockchain.ErrorHandler(err)
		case "send":
			err := sendCmd.Parse(os.Args[2:])
			blockchain.ErrorHandler(err)
		case "print-chain":
			err := printChainCmd.Parse(os.Args[2:])
			blockchain.ErrorHandler(err)
		case "create-wallet":
			err := createWalletCmd.Parse(os.Args[2:])
			blockchain.ErrorHandler(err)
		case "list-addresses":
			err := listAddressesCmd.Parse(os.Args[2:])
			blockchain.ErrorHandler(err)
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
}