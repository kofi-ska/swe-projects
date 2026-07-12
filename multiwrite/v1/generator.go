package main

import "math/rand"

type RecordGenerator struct {
	profile    string
	recordSize int
	rng        *rand.Rand
	index      int64
}

func NewRecordGenerator(profile string, seed int64, recordSize int) *RecordGenerator {
	return &RecordGenerator{
		profile:    profile,
		recordSize: recordSize,
		rng:        rand.New(rand.NewSource(seed)),
	}
}

func (g *RecordGenerator) Next() []byte {
	size := g.nextSize()
	record := make([]byte, size)
	payloadLen := size - 1

	switch g.profile {
	case ProfileHighEntropy:
		for i := 0; i < payloadLen; i++ {
			record[i] = byte(33 + g.rng.Intn(94))
		}
	case ProfileLowEntropy:
		for i := 0; i < payloadLen; i++ {
			record[i] = 'a'
		}
	default:
		for i := 0; i < payloadLen; i++ {
			record[i] = byte('a' + g.rng.Intn(26))
		}
	}

	record[payloadLen] = '\n'
	g.index++
	return record
}

func (g *RecordGenerator) nextSize() int {
	switch g.profile {
	case ProfileTiny:
		return 8
	case ProfileWide:
		return maxInt(g.recordSize, 4096)
	case ProfileHeavyTail:
		if g.rng.Intn(100) < 95 {
			return maxInt(16, minInt(g.recordSize, 128))
		}
		return maxInt(g.recordSize, 4096)
	case ProfileBurst:
		if (g.index/100)%2 == 0 {
			return maxInt(16, minInt(g.recordSize, 64))
		}
		return maxInt(g.recordSize, 512)
	default:
		return g.recordSize
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
