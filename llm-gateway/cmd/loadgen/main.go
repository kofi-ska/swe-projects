package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8080/v1/generate", "endpoint")
	rps := flag.Int("rps", 25, "requests per second")
	duration := flag.Duration("duration", 30*time.Second, "test duration")
	tokens := flag.Int("tokens", 50, "tokens per request")
	flag.Parse()
	client := &http.Client{Timeout: 20 * time.Second}
	ticker := time.NewTicker(time.Second / time.Duration(*rps))
	defer ticker.Stop()
	deadline := time.After(*duration)
	var sent, ok, failed atomic.Int64
	var wg sync.WaitGroup
	payload := []byte(fmt.Sprintf(`{"prompt":"explain tracing","max_tokens":%d,"priority":"normal"}`, *tokens))
	for {
		select {
		case <-deadline:
			wg.Wait()
			fmt.Printf("sent=%d success=%d failed=%d\n", sent.Load(), ok.Load(), failed.Load())
			return
		case <-ticker.C:
			sent.Add(1)
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := client.Post(*url, "application/json", bytes.NewReader(payload))
				if err != nil {
					failed.Add(1)
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					ok.Add(1)
				} else {
					failed.Add(1)
				}
			}()
		}
	}
}
