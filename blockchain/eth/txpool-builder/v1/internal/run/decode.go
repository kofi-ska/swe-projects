package run

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"txpool-builder/v1/internal/model"
	rpcx "txpool-builder/v1/internal/rpc"
)

// decodeTx keeps field validation local so a bad transaction fails with a precise reason.
func decodeTx(sender, nonceKey string, raw json.RawMessage, baseFee *big.Int, blockGasLimit uint64, stage string) (model.Transaction, *model.TxDecision, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return model.Transaction{}, &model.TxDecision{TxHash: "", From: sender, Accepted: false, PrimaryReason: model.ReasonDecodeError, ReasonDetail: err.Error(), Stage: stage}, err
	}

	tx := model.Transaction{From: sender, RawMetadata: map[string]any{}, Value: big.NewInt(0)}
	tx.Hash = parseString(obj, "hash")
	if tx.Hash == "" {
		return model.Transaction{}, &model.TxDecision{TxHash: "", From: sender, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing hash", Stage: stage}, fmt.Errorf("missing hash")
	}
	from := parseString(obj, "from")
	if from == "" {
		from = sender
	}
	tx.From = from
	if !isValidHexAddress(tx.From) {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidAddress, ReasonDetail: "invalid sender", Stage: stage}, fmt.Errorf("invalid sender")
	}
	to := parseNullableString(obj, "to")
	tx.To = to
	nonce, ok := parseUint(obj, "nonce")
	if !ok {
		nonce, ok = parseUintString(nonceKey)
	}
	if !ok {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing nonce", Stage: stage}, fmt.Errorf("missing nonce")
	}
	tx.Nonce = nonce
	txType, ok := parseUint(obj, "type")
	if !ok {
		txType = 0
	}
	tx.TxType = uint8(txType)
	if tx.TxType > 2 || hasBlobFields(obj) {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonUnsupportedTxType, ReasonDetail: "unsupported tx type", Stage: stage}, fmt.Errorf("unsupported tx type")
	}
	tx.GasLimit, ok = parseUint(obj, "gas")
	if !ok || tx.GasLimit == 0 {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidGasLimit, ReasonDetail: "invalid gas limit", Stage: stage}, fmt.Errorf("invalid gas limit")
	}
	if blockGasLimit > 0 && tx.GasLimit > blockGasLimit {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonExceedsBlockGas, ReasonDetail: "gas limit exceeds block limit", Stage: stage}, fmt.Errorf("gas limit exceeds block")
	}
	if tx.TxType == 0 || tx.TxType == 1 {
		tx.GasPrice, _ = parseBig(obj, "gasPrice")
		if tx.GasPrice == nil {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing gasPrice", Stage: stage}, fmt.Errorf("missing gasPrice")
		}
	}
	if tx.TxType == 2 {
		tx.MaxFeePerGas, _ = parseBig(obj, "maxFeePerGas")
		tx.MaxPriorityFeePerGas, _ = parseBig(obj, "maxPriorityFeePerGas")
		if tx.MaxFeePerGas == nil || tx.MaxPriorityFeePerGas == nil {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing dynamic fee fields", Stage: stage}, fmt.Errorf("missing dynamic fee fields")
		}
		if baseFee == nil {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidFeeModel, ReasonDetail: "missing base fee for dynamic fee tx", Stage: stage}, fmt.Errorf("missing base fee")
		}
		if tx.MaxFeePerGas.Cmp(baseFee) < 0 {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInsufficientEffectiveFee, ReasonDetail: "maxFeePerGas below baseFee", Stage: stage}, fmt.Errorf("maxFeePerGas below baseFee")
		}
	}
	tx.Value, _ = parseBig(obj, "value")
	if tx.Value == nil {
		tx.Value = big.NewInt(0)
	}
	input := parseString(obj, "input")
	if input == "" {
		input = parseString(obj, "data")
	}
	if !strings.HasPrefix(input, "0x") {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidHex, ReasonDetail: "invalid input hex", Stage: stage}, fmt.Errorf("invalid input hex")
	}
	dataBytes, err := hex.DecodeString(strings.TrimPrefix(input, "0x"))
	if err != nil {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidHex, ReasonDetail: err.Error(), Stage: stage}, err
	}
	tx.InputLen = len(dataBytes)
	tx.AccessList = countAccessList(obj["accessList"])
	tx.IntrinsicGas, err = computeIntrinsicGas(tx, dataBytes)
	if err != nil {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidGasLimit, ReasonDetail: err.Error(), Stage: stage}, err
	}
	if tx.GasLimit < tx.IntrinsicGas {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidGasLimit, ReasonDetail: "gas below intrinsic gas", Stage: stage}, fmt.Errorf("gas below intrinsic gas")
	}
	if tx.TxType == 0 || tx.TxType == 1 {
		tx.EffectiveGasPrice = new(big.Int).Set(tx.GasPrice)
		tx.EffectivePriorityFee = new(big.Int).Set(tx.GasPrice)
		if baseFee != nil {
			tx.EffectivePriorityFee = new(big.Int).Sub(tx.GasPrice, baseFee)
			if tx.EffectivePriorityFee.Sign() < 0 {
				return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInsufficientEffectiveFee, ReasonDetail: "gas price below base fee", Stage: stage}, fmt.Errorf("gas price below base fee")
			}
		}
	}
	if tx.TxType == 2 {
		tip := new(big.Int).Sub(tx.MaxFeePerGas, baseFee)
		if tip.Sign() < 0 {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInsufficientEffectiveFee, ReasonDetail: "maxFeePerGas below baseFee", Stage: stage}, fmt.Errorf("maxFeePerGas below baseFee")
		}
		if tip.Cmp(tx.MaxPriorityFeePerGas) > 0 {
			tip = new(big.Int).Set(tx.MaxPriorityFeePerGas)
		}
		tx.EffectivePriorityFee = tip
		tx.EffectiveGasPrice = new(big.Int).Add(baseFee, tip)
	}
	tx.Score = new(big.Int).Mul(tx.EffectivePriorityFee, new(big.Int).SetUint64(tx.GasLimit))
	return tx, nil, nil
}

// parseBlockHeader extracts just the header fields selection actually needs.
func parseBlockHeader(header rpcx.BlockHeader) (*big.Int, uint64, error) {
	var baseFee *big.Int
	if header.BaseFeePerGas != "" && header.BaseFeePerGas != "null" {
		baseFee = parseHexBig(header.BaseFeePerGas)
		if baseFee == nil {
			return nil, 0, fmt.Errorf("invalid base fee")
		}
	}
	gasLimit := uint64(0)
	if header.GasLimit != "" && header.GasLimit != "null" {
		v, err := parseHexUint64(header.GasLimit)
		if err != nil {
			return nil, 0, err
		}
		gasLimit = v
	}
	return baseFee, gasLimit, nil
}

// computeIntrinsicGas keeps gas checks numeric and predictable.
func computeIntrinsicGas(tx model.Transaction, data []byte) (uint64, error) {
	const (
		txGas                = 21000
		txCreationGas        = 32000
		txZeroByteGas        = 4
		txNonZeroByteGas     = 16
		accessListAddressGas = 2400
		accessListStorageGas = 1900
	)
	gas := uint64(txGas)
	if tx.To == nil {
		gas += txCreationGas
	}
	for _, b := range data {
		if b == 0 {
			gas += txZeroByteGas
		} else {
			gas += txNonZeroByteGas
		}
	}
	if tx.AccessList > 0 {
		gas += accessListAddressGas
		gas += accessListStorageGas
	}
	return gas, nil
}

// countAccessList tracks access-list size without retaining the full JSON payload.
func countAccessList(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return 0
	}
	return len(arr)
}
