package commitment

import "crypto/sha256"

func Root(leaves ...[]byte) [32]byte {
	if len(leaves) == 0 {
		return [32]byte{}
	}
	layer := make([][32]byte, 0, len(leaves))
	for _, leaf := range leaves {
		layer = append(layer, sha256.Sum256(leaf))
	}
	for len(layer) > 1 {
		next := make([][32]byte, 0, (len(layer)+1)/2)
		for i := 0; i < len(layer); i += 2 {
			left := layer[i]
			right := left
			if i+1 < len(layer) {
				right = layer[i+1]
			}
			buf := append(left[:], right[:]...)
			next = append(next, sha256.Sum256(buf))
		}
		layer = next
	}
	return layer[0]
}
