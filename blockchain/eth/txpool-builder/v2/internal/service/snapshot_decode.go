package service

import (
	"encoding/json"
	"fmt"
	"math/big"

	"txpool-builder/v2/internal/model"
)

type rawPool struct {
	Pending map[string]map[string]json.RawMessage `json:"pending"`
	Queued  map[string]map[string]json.RawMessage `json:"queued"`
}

type rawTransaction struct {
	Hash                 string `json:"hash"`
	From                 string `json:"from"`
	To                   string `json:"to"`
	Gas                  string `json:"gas"`
	GasPrice             string `json:"gasPrice"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
	Value                string `json:"value"`
	Type                 string `json:"type"`
	Input                string `json:"input"`
}

// decodePool isolates schema parsing so bad upstream payloads fail early.
func decodePool(raw json.RawMessage) (rawPool, error) {
	var pool rawPool
	if err := json.Unmarshal(raw, &pool); err != nil {
		return rawPool{}, err
	}
	if pool.Pending == nil {
		pool.Pending = map[string]map[string]json.RawMessage{}
	}
	if pool.Queued == nil {
		pool.Queued = map[string]map[string]json.RawMessage{}
	}
	return pool, nil
}

// decodeTx keeps per-transaction validation local so rejection reasons stay precise.
func decodeTx(sender, nonceKey string, raw json.RawMessage, baseFee *big.Int, blockGasLimit uint64, stage string) (model.Transaction, *model.TxDecision, error) {
	var txraw rawTransaction
	if err := json.Unmarshal(raw, &txraw); err != nil {
		return model.Transaction{}, nil, err
	}

	nonce, ok := parseUintString(nonceKey)
	if !ok {
		decision := rejectDecision(model.Transaction{From: sender}, model.ReasonInvalidNonce, "invalid nonce key", stage)
		return model.Transaction{}, &decision, nil
	}
	if txraw.Hash == "" {
		decision := rejectDecision(model.Transaction{From: sender, Nonce: nonce}, model.ReasonMissingField, "missing tx hash", stage)
		return model.Transaction{}, &decision, nil
	}
	gasLimit, ok := parseUintString(txraw.Gas)
	if !ok || gasLimit == 0 {
		decision := rejectDecision(model.Transaction{Hash: txraw.Hash, From: sender, Nonce: nonce}, model.ReasonInvalidGas, "invalid gas", stage)
		return model.Transaction{}, &decision, nil
	}
	if blockGasLimit > 0 && gasLimit > blockGasLimit {
		decision := rejectDecision(model.Transaction{Hash: txraw.Hash, From: sender, Nonce: nonce, GasLimit: gasLimit}, model.ReasonExceedsBlockGas, "tx gas exceeds block gas limit", stage)
		return model.Transaction{}, &decision, nil
	}

	txType, _ := parseUintString(txraw.Type)
	tx := model.Transaction{
		Hash:     txraw.Hash,
		From:     firstNonEmpty(txraw.From, sender),
		To:       txraw.To,
		Nonce:    nonce,
		TxType:   uint8(txType),
		GasLimit: gasLimit,
		InputLen: inputLen(txraw.Input),
	}

	value, ok := parseBigIntString(txraw.Value)
	if ok {
		tx.Value = value
	}

	if err := resolveFeeFields(&tx, txraw, baseFee); err != nil {
		decision := rejectDecision(tx, model.ReasonInvalidFeeModel, err.Error(), stage)
		return model.Transaction{}, &decision, nil
	}
	return tx, nil, nil
}

// resolveFeeFields keeps fee normalization in one place so ranking math stays consistent.
func resolveFeeFields(tx *model.Transaction, txraw rawTransaction, baseFee *big.Int) error {
	switch {
	case txraw.MaxFeePerGas != "" || txraw.MaxPriorityFeePerGas != "":
		if baseFee == nil {
			return fmt.Errorf("dynamic fee tx requires base fee")
		}
		maxFee, ok := parseBigIntString(txraw.MaxFeePerGas)
		if !ok {
			return fmt.Errorf("invalid maxFeePerGas")
		}
		maxTip, ok := parseBigIntString(txraw.MaxPriorityFeePerGas)
		if !ok {
			return fmt.Errorf("invalid maxPriorityFeePerGas")
		}
		if maxFee.Cmp(baseFee) < 0 {
			return fmt.Errorf("maxFeePerGas below base fee")
		}
		priorityCap := new(big.Int).Sub(maxFee, baseFee)
		effectiveTip := minBig(maxTip, priorityCap)
		tx.MaxFeePerGas = maxFee
		tx.MaxPriorityFeePerGas = maxTip
		tx.EffectiveFee = effectiveTip
		tx.Score = new(big.Int).Mul(new(big.Int).Set(effectiveTip), new(big.Int).SetUint64(tx.GasLimit))
		return nil
	case txraw.GasPrice != "":
		gasPrice, ok := parseBigIntString(txraw.GasPrice)
		if !ok {
			return fmt.Errorf("invalid gasPrice")
		}
		tx.GasPrice = gasPrice
		tx.EffectiveFee = gasPrice
		tx.Score = new(big.Int).Mul(new(big.Int).Set(gasPrice), new(big.Int).SetUint64(tx.GasLimit))
		return nil
	default:
		return fmt.Errorf("missing fee fields")
	}
}
