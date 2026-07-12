package tests

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	multiwrite "multiwrite/src"
)

func BenchmarkDirectWriter(b *testing.B) {
	benchmarkWriterMicro(b, multiwrite.WriterDirect, 32, 16*1024)
}

func BenchmarkBufferedWriter(b *testing.B) {
	benchmarkWriterMicro(b, multiwrite.WriterBuffered, 32, 16*1024)
}

func BenchmarkBufferedWriterSizes(b *testing.B) {
	for _, size := range []int{4 * 1024, 16 * 1024, 64 * 1024, 256 * 1024} {
		b.Run(formatSize(size), func(b *testing.B) {
			benchmarkWriterMicro(b, multiwrite.WriterBuffered, 32, size)
		})
	}
}

func BenchmarkRecordSizes(b *testing.B) {
	for _, size := range []int{8, 32, 128, 1024, 4096} {
		b.Run(formatSize(size), func(b *testing.B) {
			benchmarkWriterMicro(b, multiwrite.WriterBuffered, size, 16*1024)
		})
	}
}

func BenchmarkGeneratorCost(b *testing.B) {
	for _, profile := range []string{multiwrite.ProfileFixed, multiwrite.ProfileTiny, multiwrite.ProfileWide, multiwrite.ProfileHeavyTail, multiwrite.ProfileHighEntropy, multiwrite.ProfileLowEntropy, multiwrite.ProfileBurst} {
		b.Run(profile, func(b *testing.B) {
			gen := multiwrite.NewRecordGenerator(profile, 7, 64)
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
	b.Run(multiwrite.WriterDirect, func(b *testing.B) {
		benchmarkWriterMicro(b, multiwrite.WriterDirect, 32, 16*1024)
	})
	b.Run(multiwrite.WriterBuffered, func(b *testing.B) {
		benchmarkWriterMicro(b, multiwrite.WriterBuffered, 32, 16*1024)
	})
}

func BenchmarkEngineLifecycleNoSync(b *testing.B) {
	b.Run(multiwrite.WriterDirect, func(b *testing.B) {
		benchmarkRun(b, multiwrite.WriterDirect, multiwrite.ProfileFixed, 32, 16*1024)
	})
	b.Run(multiwrite.WriterBuffered, func(b *testing.B) {
		benchmarkRun(b, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 32, 16*1024)
	})
}

func BenchmarkWritePathSync(b *testing.B) {
	if os.Getenv("MULTIWRITE_RUN_SYNC_BENCH") != "1" {
		b.Skip("set MULTIWRITE_RUN_SYNC_BENCH=1 to run environment-sensitive sync benchmark")
	}
	b.Run(multiwrite.WriterDirect, func(b *testing.B) {
		benchmarkRunWithDurability(b, multiwrite.WriterDirect, multiwrite.ProfileFixed, 32, 16*1024, multiwrite.DurabilitySynced)
	})
	b.Run(multiwrite.WriterBuffered, func(b *testing.B) {
		benchmarkRunWithDurability(b, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 32, 16*1024, multiwrite.DurabilitySynced)
	})
}

func benchmarkWriterMicro(b *testing.B, writer string, recordSize, bufferSize int) {
	dir := b.TempDir()
	records := prebuiltRecords(multiwrite.ProfileFixed, 123, recordSize, 4096)
	b.SetBytes(recordsTotalBytes(records))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		w, err := newBenchmarkWriter(filepath.Join(dir, "writer-micro.txt"), writer, bufferSize)
		if err != nil {
			b.Fatalf("new writer: %v", err)
		}
		b.StartTimer()

		for _, record := range records {
			if err := w.WriteRecord(record); err != nil {
				b.Fatalf("write record: %v", err)
			}
		}
		if err := w.Flush(); err != nil {
			b.Fatalf("flush: %v", err)
		}
		if err := w.Close(); err != nil {
			b.Fatalf("close: %v", err)
		}
	}
}

func benchmarkRun(b *testing.B, writer, profile string, recordSize, bufferSize int) {
	benchmarkRunWithDurability(b, writer, profile, recordSize, bufferSize, multiwrite.DurabilityClosed)
}

func benchmarkRunWithDurability(b *testing.B, writer, profile string, recordSize, bufferSize int, durability string) {
	dir := b.TempDir()
	const records = int64(4096)
	b.SetBytes(recordsTotalBytes(prebuiltRecords(profile, 123, recordSize, int(records))))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg := multiwrite.RunConfig{
			RecordsLimit: records,
			MaxBytes:     256 * 1024 * 1024,
			MaxDuration:  10 * time.Second,
			Writer:       writer,
			Profile:      profile,
			Seed:         123,
			RecordSize:   recordSize,
			BufferSize:   bufferSize,
			BatchSize:    multiwrite.DefaultBatchSize,
			OutputPath:   filepath.Join(dir, "out.txt"),
			ManifestPath: filepath.Join(dir, "manifest.json"),
			Durability:   durability,
			Overwrite:    multiwrite.OverwriteReplace,
		}
		if _, err := multiwrite.Run(cfg); err != nil {
			b.Fatalf("Run() error = %v", err)
		}
	}
}

func prebuiltRecords(profile string, seed int64, recordSize, count int) [][]byte {
	gen := multiwrite.NewRecordGenerator(profile, seed, recordSize)
	records := make([][]byte, count)
	for i := range records {
		records[i] = gen.Next()
	}
	return records
}

func newBenchmarkWriter(path, writer string, bufferSize int) (multiwrite.MeasuredWriter, error) {
	switch writer {
	case multiwrite.WriterDirect:
		return multiwrite.NewDirectFileWriter(path)
	case multiwrite.WriterBuffered:
		return multiwrite.NewBufferedFileWriter(path, bufferSize)
	default:
		return nil, os.ErrInvalid
	}
}

func recordsTotalBytes(records [][]byte) int64 {
	var total int64
	for _, record := range records {
		total += int64(len(record))
	}
	return total
}

func formatSize(size int) string {
	if size >= 1024 {
		return strconv.Itoa(size/1024) + "KiB"
	}
	return strconv.Itoa(size) + "B"
}
