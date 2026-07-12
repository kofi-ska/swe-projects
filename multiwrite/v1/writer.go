package main

import (
	"bufio"
	"fmt"
	"os"
)

type MeasuredWriter interface {
	WriteRecord(record []byte) error
	Flush() error
	Sync() error
	Close() error
}

type DirectFileWriter struct {
	file *os.File
}

func NewDirectFileWriter(path string) (*DirectFileWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("%w: create output: %v", ErrFilesystem, err)
	}
	return &DirectFileWriter{file: f}, nil
}

func (w *DirectFileWriter) WriteRecord(record []byte) error {
	if err := writeFull(w.file, record); err != nil {
		return fmt.Errorf("%w: direct write: %v", ErrFilesystem, err)
	}
	return nil
}

func (w *DirectFileWriter) Flush() error {
	return nil
}

func (w *DirectFileWriter) Sync() error {
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("%w: sync output: %v", ErrFilesystem, err)
	}
	return nil
}

func (w *DirectFileWriter) Close() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("%w: close output: %v", ErrFilesystem, err)
	}
	return nil
}

type BufferedFileWriter struct {
	file   *os.File
	buffer *bufio.Writer
}

func NewBufferedFileWriter(path string, bufferSize int) (*BufferedFileWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("%w: create output: %v", ErrFilesystem, err)
	}
	return &BufferedFileWriter{
		file:   f,
		buffer: bufio.NewWriterSize(f, bufferSize),
	}, nil
}

func (w *BufferedFileWriter) WriteRecord(record []byte) error {
	if _, err := w.buffer.Write(record); err != nil {
		return fmt.Errorf("%w: buffered write: %v", ErrFilesystem, err)
	}
	return nil
}

func (w *BufferedFileWriter) Flush() error {
	if err := w.buffer.Flush(); err != nil {
		return fmt.Errorf("%w: flush output: %v", ErrFilesystem, err)
	}
	return nil
}

func (w *BufferedFileWriter) Sync() error {
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("%w: sync output: %v", ErrFilesystem, err)
	}
	return nil
}

func (w *BufferedFileWriter) Close() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("%w: close output: %v", ErrFilesystem, err)
	}
	return nil
}

func newMeasuredWriter(path string, cfg RunConfig) (MeasuredWriter, error) {
	switch cfg.Writer {
	case WriterDirect:
		return NewDirectFileWriter(path)
	case WriterBuffered:
		return NewBufferedFileWriter(path, cfg.BufferSize)
	default:
		return nil, fmt.Errorf("%w: unknown writer strategy %q", ErrValidation, cfg.Writer)
	}
}

func writeFull(file *os.File, record []byte) error {
	for len(record) > 0 {
		n, err := file.Write(record)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("zero-byte write")
		}
		record = record[n:]
	}
	return nil
}
