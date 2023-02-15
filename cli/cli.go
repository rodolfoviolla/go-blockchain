package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/rodolfoviolla/go-blockchain/blockchain"
)

type CommandLine struct {}

func (cli *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" get-balance -address ADDRESS           - Gets the balance for an address")
	fmt.Println(" create-blockchain -address ADDRESS     - Creates a blockchain and sends genesis reward to address")
	fmt.Println(" print-chain                            - Prints the blocks in the chain")
	fmt.Println(" send - from FROM -to TO -amount AMOUNT - Send amount of coins")
}

func (cli * CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
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
		if len(block.PrevHash) == 0 {
			break
		}
	}
	fmt.Println()
}

func (cli *CommandLine) createBlockChain(address string) {
	chain := blockchain.InitBlockChain(address)
	chain.Database.Close()
	fmt.Println("Finished!")
}

func (cli *CommandLine) getBalance(address string) {
	chain := blockchain.ContinueBlockChain()
	defer chain.Database.Close()
	balance := 0
	unspentTransactionsOutput := chain.FindUnspentTransactionsOutputs(address)
	for _, out := range unspentTransactionsOutput {
		balance += out.Value
	}
	fmt.Printf("Balance of %s: %d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int) {
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
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	createBlockChainAddress := createBlockChainCmd.String("address", "", "The address to send genesis block reward to")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.String("amount", "", "Amount to send")
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
		if *sendFrom == "" || *sendTo == "" || *sendAmount == "" {
			sendCmd.Usage()
			runtime.Goexit()
		}
		amount, err := strconv.Atoi(*sendAmount)
		if err != nil {
			blockchain.ErrorHandler(err)
			sendCmd.Usage()
			runtime.Goexit()
		}
		cli.send(*sendFrom, *sendTo, amount)
	}
	if printChainCmd.Parsed() {
		cli.printChain()
	}
}