package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8080/v1/generate", "generation endpoint")
	rps := flag.Int("rps", 25, "requests per second")
	duration := flag.Duration("duration", 30*time.Second, "test duration")
	tokens := flag.Int("tokens", 50, "tokens per request")
	maxInFlight := flag.Int("max-inflight", 200, "maximum concurrent requests")
	flag.Parse()
	if *rps < 1 || *duration <= 0 || *tokens < 1 || *maxInFlight < 1 {
		flag.Usage()
		return
	}

	client := &http.Client{Timeout: 20 * time.Second}
	payload := []byte(fmt.Sprintf(`{"prompt":"explain tracing","max_tokens":%d,"priority":"normal"}`, *tokens))
	ticker := time.NewTicker(time.Second / time.Duration(*rps))
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	started := time.Now().UTC()
	fmt.Printf("started_utc=%s started_unix=%d\n", started.Format(time.RFC3339Nano), started.Unix())
	sem := make(chan struct{}, *maxInFlight)
	var sent, succeeded, failed, dropped atomic.Int64
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			ended := time.Now().UTC()
			fmt.Printf("ended_utc=%s ended_unix=%d elapsed=%s\n", ended.Format(time.RFC3339Nano), ended.Unix(), ended.Sub(started).Round(time.Millisecond))
			fmt.Printf("sent=%d success=%d failed=%d dropped=%d\n", sent.Load(), succeeded.Load(), failed.Load(), dropped.Load())
			return
		case <-ticker.C:
			select {
			case sem <- struct{}{}:
				sent.Add(1)
				wg.Add(1)
				go send(client, *url, payload, &wg, sem, &succeeded, &failed)
			default:
				dropped.Add(1)
			}
		}
	}
}

func send(client *http.Client, url string, payload []byte, wg *sync.WaitGroup, sem chan struct{}, succeeded, failed *atomic.Int64) {
	defer wg.Done()
	defer func() { <-sem }()
	response, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		failed.Add(1)
		return
	}
	_, copyErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if response.StatusCode == http.StatusOK && copyErr == nil && closeErr == nil {
		succeeded.Add(1)
	} else {
		failed.Add(1)
	}
}
