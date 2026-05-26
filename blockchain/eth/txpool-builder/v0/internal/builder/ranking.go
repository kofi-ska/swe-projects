package builder

import (
	"math/big"
	"sort"
)

// RankByGasPrice sorts transactions in descending order of gas price.
// Returns a reordered slice with highest-paying transactions first.
func RankByGasPrice(txs []TxMeta) []TxMeta {
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].GasPrice.Cmp(txs[j].GasPrice) > 0
	})
	return txs
}

// TotalGasPrice computes the sum of gas prices for a list of transactions.
func TotalGasPrice(txs []TxMeta) *big.Int {
	total := big.NewInt(0)
	for _, t := range txs {
		total.Add(total, t.GasPrice)
	}
	return total
}
