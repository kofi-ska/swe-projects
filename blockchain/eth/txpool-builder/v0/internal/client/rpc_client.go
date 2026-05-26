package client

import (
	"log"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// ConnectEnv establishes RPC and Ethereum client connections
// using the GETH_RPC_URL environment variable.
func Connect() (*rpc.Client, *ethclient.Client) {
	url := os.Getenv("GETH_RPC_URL")
	if url == "" {
		log.Fatal("missing GETH_RPC_URL environment variable")
	}

	rpcClient, err := rpc.Dial(url)
	if err != nil {
		log.Fatalf("failed to connect to RPC endpoint: %v", err)
	}

	ethClient := ethclient.NewClient(rpcClient)
	log.Println("connected to Geth node")
	return rpcClient, ethClient
}
