package main

import (
	"flag"
	"log"
	"time"
)

func main() {
	// CLI flags for terminal usage
	count := flag.Int64("n", 1_000_000, "number of lines to write")
	out := flag.String("out", "output/words.txt", "output file path")
	flag.Parse()

	start := time.Now()

	// buffer config
	cfg := DefaultBuffer()

	// writer
	w, err := NewBufferedFileWriter(*out, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	// generator fopr words
	gen := NewSimpleGenerator()

	// write loop
	for i := int64(0); i < *count; i++ {
		if err := w.WriteWord(gen.Next()); err != nil {
			log.Fatal(err)
		}
	}

	elapsed := time.Since(start)
	log.Printf("Wrote %d lines in %s", *count, elapsed)
}
