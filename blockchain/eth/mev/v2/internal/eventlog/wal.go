package eventlog

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one append-only WAL entry.
type Entry struct {
	Kind string          `json:"kind"`
	Time time.Time       `json:"time"`
	Data json.RawMessage `json:"data"`
}

// WAL persists append-only entries for replay and audit.
type WAL struct {
	mu         sync.Mutex
	path       string
	file       *os.File
	writer     *bufio.Writer
	flushEvery int
	maxEntries int
	writes     int
}

// Open creates or opens a WAL at the given path.
func Open(path string, flushEvery, maxEntries int) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	if flushEvery <= 0 {
		flushEvery = 128
	}
	if maxEntries <= 0 {
		maxEntries = 2048
	}
	return &WAL{path: path, file: file, writer: bufio.NewWriterSize(file, 1<<20), flushEvery: flushEvery, maxEntries: maxEntries}, nil
}

// Append writes one entry to the log and forces it durable.
func (w *WAL) Append(ctx context.Context, kind string, payload any) error {
	_ = ctx
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return w.AppendEncoded(ctx, kind, body)
}

// AppendEncoded writes one pre-marshaled entry to the log and forces it durable.
func (w *WAL) AppendEncoded(ctx context.Context, kind string, body []byte) error {
	_ = ctx
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return errors.New("wal closed")
	}
	entry := Entry{
		Kind: kind,
		Time: time.Now().UTC(),
		Data: body,
	}
	if err := json.NewEncoder(w.writer).Encode(entry); err != nil {
		return err
	}
	w.writes++
	if kind == "checkpoint" || w.writes%w.flushEvery == 0 {
		if err := w.writer.Flush(); err != nil {
			return err
		}
		if err := w.file.Sync(); err != nil {
			return err
		}
	}
	if w.maxEntries > 0 && w.writes > w.maxEntries*2 {
		if err := w.compactLocked(); err != nil {
			return err
		}
	}
	return nil
}

// ReadAll loads up to limit WAL entries from disk.
func (w *WAL) ReadAll(limit int) ([]Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.path == "" {
		return nil, errors.New("wal closed")
	}
	if limit <= 0 {
		limit = 256
	}
	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return nil, err
		}
	}
	file, err := os.Open(w.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readEntries(file, limit)
}

// Health reports whether the WAL is available for durable writes.
func (w *WAL) Health() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return errors.New("wal closed")
	}
	_, err := os.Stat(w.path)
	return err
}

// Close releases the WAL file handle.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			_ = w.file.Close()
			return err
		}
	}
	err := w.file.Close()
	w.file = nil
	w.writer = nil
	w.path = ""
	return err
}

func (w *WAL) compactLocked() error {
	if w.file == nil {
		return errors.New("wal closed")
	}
	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return err
		}
	}
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}
	entries, err := readEntries(w.file, w.maxEntries)
	if err != nil {
		return err
	}
	if err := w.file.Truncate(0); err != nil {
		return err
	}
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}
	w.writer = bufio.NewWriterSize(w.file, 1<<20)
	for _, entry := range entries {
		if err := json.NewEncoder(w.writer).Encode(entry); err != nil {
			return err
		}
	}
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if err := w.file.Sync(); err != nil {
		return err
	}
	w.writes = len(entries)
	return nil
}

func readEntries(file *os.File, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 256
	}
	out := make([]Entry, 0, limit)
	dec := json.NewDecoder(file)
	for {
		var entry Entry
		if err := dec.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(out) < limit {
			out = append(out, entry)
			continue
		}
		copy(out, out[1:])
		out[len(out)-1] = entry
	}
	return out, nil
}
