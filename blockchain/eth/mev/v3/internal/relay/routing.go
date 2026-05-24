package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"mevrelayv3/internal/graph"
	"mevrelayv3/internal/model"
)

func bundleHash(req model.BundleRequest) string {
	sum := sha256.New()
	for _, tx := range req.Txs {
		sum.Write([]byte(tx))
		sum.Write([]byte{0})
	}
	sum.Write([]byte(req.BlockNumber))
	sum.Write([]byte(fmt.Sprintf(":%d:%d", req.MinTimestamp, req.MaxTimestamp)))
	if req.Replacement != nil {
		sum.Write([]byte(*req.Replacement))
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func bundleID(hash string, id int64) string {
	return fmt.Sprintf("%s:%d", hash, id)
}

func bundleKey(req model.BundleRequest, networkID string) string {
	return strings.Join([]string{bundleHash(req), networkID, req.BlockNumber}, ":")
}

func (s *Service) routeShard(req model.BundleRequest) string {
	return s.routing.Route(bundleKey(req, s.cfg.NetworkID))
}

func encodeJSON(v any) []byte {
	body, _ := json.Marshal(v)
	return body
}

func authorityKey(auth graph.Authority) string {
	return graph.AuthorityKey(auth.ShardID, auth.Epoch, auth.FenceToken)
}

func checkpointID(bundleID string, sequence uint64) string {
	return fmt.Sprintf("%s:%d", bundleID, sequence)
}

func checkpointObjectKey(shardID, bundleID string, sequence uint64) string {
	if shardID == "" {
		return fmt.Sprintf("checkpoints/%s/%s.json", bundleID, checkpointID(bundleID, sequence))
	}
	return fmt.Sprintf("checkpoints/%s/%s.json", shardID, checkpointID(bundleID, sequence))
}
