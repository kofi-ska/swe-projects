package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"

	multiwrite "multiwrite/src"
)

func main() {
	if err := runCLI(os.Args[1:]); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func runCLI(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "run":
			args = args[1:]
		case "help", "-h", "--help":
			printUsage()
			return nil
		default:
			if args[0][0] != '-' {
				return fmt.Errorf("unknown command %q", args[0])
			}
		}
	}

	cfg := multiwrite.DefaultRunConfig()
	var cpuProfilePath string
	var memProfilePath string
	var tracePath string

	fs := flag.NewFlagSet("multiwrite run", flag.ContinueOnError)
	fs.Int64Var(&cfg.RecordsLimit, "records", cfg.RecordsLimit, "maximum records to write")
	fs.Int64Var(&cfg.RecordsLimit, "n", cfg.RecordsLimit, "maximum records to write")
	fs.Int64Var(&cfg.MaxBytes, "max-bytes", cfg.MaxBytes, "maximum bytes to write")
	fs.DurationVar(&cfg.MaxDuration, "max-duration", cfg.MaxDuration, "maximum run duration")
	fs.StringVar(&cfg.Writer, "writer", cfg.Writer, "writer strategy: direct or buffered")
	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "workload profile")
	fs.Int64Var(&cfg.Seed, "seed", cfg.Seed, "deterministic workload seed")
	fs.IntVar(&cfg.RecordSize, "record-size", cfg.RecordSize, "record size in bytes")
	fs.IntVar(&cfg.BufferSize, "buffer-size", cfg.BufferSize, "buffer size in bytes")
	fs.IntVar(&cfg.BatchSize, "batch-size", cfg.BatchSize, "batch size in bytes")
	fs.IntVar(&cfg.QueueDepth, "queue-depth", cfg.QueueDepth, "bounded queue depth; zero means direct path")
	fs.StringVar(&cfg.OutputPath, "out", cfg.OutputPath, "output artifact path")
	fs.StringVar(&cfg.ManifestPath, "manifest", cfg.ManifestPath, "manifest path")
	fs.StringVar(&cfg.Durability, "durability", cfg.Durability, "durability mode: closed, synced, committed")
	fs.StringVar(&cfg.Overwrite, "overwrite", cfg.Overwrite, "overwrite policy: fail, replace, version")
	fs.StringVar(&cpuProfilePath, "cpuprofile", "", "write CPU profile to path")
	fs.StringVar(&memProfilePath, "memprofile", "", "write heap profile to path")
	fs.StringVar(&tracePath, "trace", "", "write Go execution trace to path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	stopProfiles, err := startProfiles(cpuProfilePath, tracePath)
	if err != nil {
		return err
	}
	defer stopProfiles()

	result, runErr := multiwrite.Run(cfg)
	if memProfilePath != "" {
		if err := writeHeapProfile(memProfilePath); err != nil && runErr == nil {
			runErr = err
		}
	}

	log.Printf(
		"status=%s writer=%s profile=%s records=%d bytes=%d duration=%dns mib_per_sec=%.3f manifest=%s",
		result.Status,
		result.Config.Writer,
		result.Config.Profile,
		result.RecordsWritten,
		result.BytesWritten,
		result.DurationTotalNanos,
		result.MiBPerSecond,
		result.ManifestPath,
	)
	return runErr
}

func startProfiles(cpuProfilePath, tracePath string) (func(), error) {
	var files []*os.File
	var stopCPU bool
	var stopTrace bool

	if cpuProfilePath != "" {
		f, err := os.Create(cpuProfilePath)
		if err != nil {
			return nil, fmt.Errorf("%w: create CPU profile: %v", multiwrite.ErrFilesystem, err)
		}
		files = append(files, f)
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("%w: start CPU profile: %v", multiwrite.ErrApplicationBug, err)
		}
		stopCPU = true
	}

	if tracePath != "" {
		f, err := os.Create(tracePath)
		if err != nil {
			return nil, fmt.Errorf("%w: create trace: %v", multiwrite.ErrFilesystem, err)
		}
		files = append(files, f)
		if err := trace.Start(f); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("%w: start trace: %v", multiwrite.ErrApplicationBug, err)
		}
		stopTrace = true
	}

	return func() {
		if stopTrace {
			trace.Stop()
		}
		if stopCPU {
			pprof.StopCPUProfile()
		}
		for _, f := range files {
			_ = f.Close()
		}
	}, nil
}

func writeHeapProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%w: create heap profile: %v", multiwrite.ErrFilesystem, err)
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("%w: write heap profile: %v", multiwrite.ErrApplicationBug, err)
	}
	return nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: multiwrite run [flags]")
}
