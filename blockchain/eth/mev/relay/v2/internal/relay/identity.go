package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"mevrelayv2/internal/model"
)

func bundleHash(p model.BundleRequest) string {
	h := sha256.New()
	for _, tx := range p.Txs {
		h.Write([]byte(tx))
	}
	h.Write([]byte(p.BlockNumber))
	h.Write([]byte(fmt.Sprintf("%d:%d", p.MinTimestamp, p.MaxTimestamp)))
	if p.Replacement != nil {
		h.Write([]byte(*p.Replacement))
	}
	return "0x" + hex.EncodeToString(h.Sum(nil)[:16])
}

func bundleID(hash string, reqID int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", hash, reqID)))
	return "0x" + hex.EncodeToString(h[:12])
}

func checkpointID(bundleID string, version uint64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", bundleID, version)))
	return "cp_" + hex.EncodeToString(h[:12])
}
