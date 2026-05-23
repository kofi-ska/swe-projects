package commitment

import "crypto/sha256"

// HashLeaf applies domain separation to a leaf value.
func HashLeaf(value []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(value)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// HashPair hashes two child nodes with domain separation.
func HashPair(left, right [32]byte) [32]byte {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(left[:])
	h.Write(right[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// Root computes a Merkle root over the given leaves.
func Root(leaves ...[]byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256([]byte("empty-merkle-root"))
	}
	nodes := make([][32]byte, 0, len(leaves))
	for _, leaf := range leaves {
		nodes = append(nodes, HashLeaf(leaf))
	}
	for len(nodes) > 1 {
		next := make([][32]byte, 0, (len(nodes)+1)/2)
		for i := 0; i < len(nodes); i += 2 {
			if i+1 >= len(nodes) {
				next = append(next, HashPair(nodes[i], nodes[i]))
				continue
			}
			next = append(next, HashPair(nodes[i], nodes[i+1]))
		}
		nodes = next
	}
	return nodes[0]
}

// RootBytes returns the root as a byte slice.
func RootBytes(leaves ...[]byte) []byte {
	root := Root(leaves...)
	out := make([]byte, len(root))
	copy(out, root[:])
	return out
}
