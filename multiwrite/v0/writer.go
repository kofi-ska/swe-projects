package main

import (
	"bufio"
	"os"
)

// Abstraction for sending words into an output
type Writer interface {
	WriteWord(word string) error
	Close() error
}

// Buffered writes via bufio.Writer
type BufferedFileWriter struct {
	file *os.File
	buffer *bufio.Writer
}

func NewBufferedFileWriter(path string, cfg BufferConfig) (Writer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &BufferedFileWriter{
		file: f,
		buffer: bufio.NewWriterSize(f,cfg.Size),
	},nil
}

func (w *BufferedFileWriter) WriteWord(word string) error {
	// Write word + new line for readability
	_, err := w.buffer.WriteString(word + "\n")
	return err
}

func (w *BufferedFileWriter) Close() error {
	// Flush buffered data to disk
	if err := w.buffer.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}