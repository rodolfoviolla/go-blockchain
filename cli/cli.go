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

func (cli *CommandLine) printUsage() {
	fmt.Println(color.Purple + "Welcome to the blockchain CLI!" + color.Reset)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println(color.Green + "  get-balance " + color.Cyan + "-address " + color.Yellow + "ADDRESS           " + color.Reset + "- Gets the balance for an address")
	fmt.Println(color.Green + "  create-blockchain " + color.Cyan + "-address " + color.Yellow + "ADDRESS     " + color.Reset + "- Creates a blockchain and sends genesis reward to address")
	fmt.Println(color.Green + "  print-chain                            " + color.Reset + "- Prints the blocks in the chain")
	fmt.Println(color.Green + "  send " + color.Cyan + "-from " + color.Yellow + "FROM " + color.Cyan + "-to " + color.Yellow + "TO " + color.Cyan + "-amount " + color.Yellow + "AMOUNT  " + color.Reset + "- Send amount of coins")
	fmt.Println(color.Green + "  create-wallet                          " + color.Reset + "- Creates a new wallet")
	fmt.Println(color.Green + "  list-addresses                         " + color.Reset + "- List the addresses in our wallet file")
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
	chain.Database.Close()
	fmt.Println(color.Green + "Finished!")
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
	defer chain.Database.Close()
	tx := blockchain.NewTransaction(from, to, amount, chain)
	chain.AddBlock([]*blockchain.Transaction{tx})
	fmt.Println(color.Green + "Success!")
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
			handler.ErrorHandler(getBalanceCmd.Parse(os.Args[2:]))
		case "create-blockchain":
			handler.ErrorHandler(createBlockChainCmd.Parse(os.Args[2:]))
		case "send":
			handler.ErrorHandler(sendCmd.Parse(os.Args[2:]))
		case "print-chain":
			handler.ErrorHandler(printChainCmd.Parse(os.Args[2:]))
		case "create-wallet":
			handler.ErrorHandler(createWalletCmd.Parse(os.Args[2:]))
		case "list-addresses":
			handler.ErrorHandler(listAddressesCmd.Parse(os.Args[2:]))
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