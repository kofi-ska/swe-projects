package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func BenchmarkDirectWriter(b *testing.B) {
	benchmarkRun(b, WriterDirect, ProfileFixed, 32, 0)
}

func BenchmarkBufferedWriter(b *testing.B) {
	benchmarkRun(b, WriterBuffered, ProfileFixed, 32, 16*1024)
}

func BenchmarkBufferedWriterSizes(b *testing.B) {
	for _, size := range []int{4 * 1024, 16 * 1024, 64 * 1024, 256 * 1024} {
		b.Run(formatSize(size), func(b *testing.B) {
			benchmarkRun(b, WriterBuffered, ProfileFixed, 32, size)
		})
	}
}

func BenchmarkRecordSizes(b *testing.B) {
	for _, size := range []int{8, 32, 128, 1024, 4096} {
		b.Run(formatSize(size), func(b *testing.B) {
			benchmarkRun(b, WriterBuffered, ProfileFixed, size, 16*1024)
		})
	}
}

func BenchmarkGeneratorCost(b *testing.B) {
	for _, profile := range []string{ProfileFixed, ProfileTiny, ProfileWide, ProfileHeavyTail, ProfileHighEntropy, ProfileLowEntropy, ProfileBurst} {
		b.Run(profile, func(b *testing.B) {
			gen := NewRecordGenerator(profile, 7, 64)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				record := gen.Next()
				if len(record) == 0 {
					b.Fatal("empty record")
				}
			}
		})
	}
}

func BenchmarkWritePathNoSync(b *testing.B) {
	b.Run(WriterDirect, func(b *testing.B) {
		benchmarkRun(b, WriterDirect, ProfileFixed, 32, 0)
	})
	b.Run(WriterBuffered, func(b *testing.B) {
		benchmarkRun(b, WriterBuffered, ProfileFixed, 32, 16*1024)
	})
}

func BenchmarkWritePathSync(b *testing.B) {
	if os.Getenv("MULTIWRITE_RUN_SYNC_BENCH") != "1" {
		b.Skip("set MULTIWRITE_RUN_SYNC_BENCH=1 to run environment-sensitive sync benchmark")
	}
	b.Run(WriterDirect, func(b *testing.B) {
		benchmarkRunWithDurability(b, WriterDirect, ProfileFixed, 32, 0, DurabilitySynced)
	})
	b.Run(WriterBuffered, func(b *testing.B) {
		benchmarkRunWithDurability(b, WriterBuffered, ProfileFixed, 32, 16*1024, DurabilitySynced)
	})
}

func benchmarkRun(b *testing.B, writer, profile string, recordSize, bufferSize int) {
	benchmarkRunWithDurability(b, writer, profile, recordSize, bufferSize, DurabilityClosed)
}

func benchmarkRunWithDurability(b *testing.B, writer, profile string, recordSize, bufferSize int, durability string) {
	dir := b.TempDir()
	if bufferSize == 0 {
		bufferSize = 16 * 1024
	}
	const records = int64(4096)
	b.SetBytes(records * int64(recordSize))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg := RunConfig{
			RecordsLimit: records,
			MaxBytes:     256 * 1024 * 1024,
			MaxDuration:  10 * time.Second,
			Writer:       writer,
			Profile:      profile,
			Seed:         123,
			RecordSize:   recordSize,
			BufferSize:   bufferSize,
			BatchSize:    64 * 1024,
			OutputPath:   filepath.Join(dir, "out.txt"),
			ManifestPath: filepath.Join(dir, "manifest.json"),
			Durability:   durability,
			Overwrite:    OverwriteReplace,
		}
		if _, err := Run(cfg); err != nil {
			b.Fatalf("Run() error = %v", err)
		}
	}
}

func formatSize(size int) string {
	if size >= 1024 {
		return strconv.Itoa(size/1024) + "KiB"
	}
	return strconv.Itoa(size) + "B"
}
