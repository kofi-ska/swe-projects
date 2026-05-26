package relay

import "mevrelayv3/internal/model"

func validateBundleRequest(req model.JSONRPCRequest) error {
	if req.JSONRPC != "2.0" {
		return ErrInvalidJSONRPC
	}
	if req.Method != "eth_sendBundle" {
		return ErrInvalidMethod
	}
	if len(req.Params) == 0 {
		return ErrMissingParams
	}
	if len(req.Params[0].Txs) == 0 {
		return ErrMissingTxs
	}
	return nil
}
