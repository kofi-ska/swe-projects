package main

import (
	"context"
	"log"
	"txpool-builder/internal/builder"
	"txpool-builder/internal/client"
)

// - initializes the connection to a Geth node,
// - retrieves pending transactions from  txpool,
// - ranks them by gas price and assembles pseudo block.
func main() {
	ctx := context.Background()

	rpcClient, ethClient := client.Connect()
	defer rpcClient.Close()
	_ = ethClient

	txs := builder.FetchPendingTxs(ctx, rpcClient)
	if len(txs) == 0 {
		log.Println("no pending transactions found in txpool")
		return
	}

	ranked := builder.RankByGasPrice(txs)
	builder.BuildBlock(ranked, 50)

	log.Println("pseudo_block.json generated successfully")
}
