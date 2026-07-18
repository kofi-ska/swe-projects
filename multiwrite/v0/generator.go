package main

import "math/rand"

// Generator to produce words
type Generator interface {
	Next() string
}

// SimpleGenerator for pseudo-random words
type SimpleGenerator struct {
	words []string
}

func NewSimpleGenerator() Generator {
	return &SimpleGenerator{
		words: []string{
			"alpha","beta","gamma","delta","omega",
			"future","quant","trade","python","golang",
		},
	}
}

func (g *SimpleGenerator) Next() string {
	return g.words[rand.Intn(len(g.words))]
}