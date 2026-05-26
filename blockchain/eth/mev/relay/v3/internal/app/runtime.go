package app

import (
	"log"
	"strings"

	"mevrelayv3/internal/backend"
	"mevrelayv3/internal/backend/local"
	"mevrelayv3/internal/broker"
	"mevrelayv3/internal/broker/memory"
	"mevrelayv3/internal/broker/nats"
	"mevrelayv3/internal/checkpoint"
	checkpointMemory "mevrelayv3/internal/checkpoint/memory"
	checkpointMinio "mevrelayv3/internal/checkpoint/minio"
	"mevrelayv3/internal/config"
	"mevrelayv3/internal/eventlog"
	"mevrelayv3/internal/relay"
	"mevrelayv3/internal/state"
	stateMemory "mevrelayv3/internal/state/memory"
	stateValkey "mevrelayv3/internal/state/valkey"
)

type Runtime struct {
	Service *relay.Service
	Backend backend.Adapter
	Broker  broker.Broker
	Store   state.Store
	CP      checkpoint.Store
	WAL     *eventlog.WAL
	Close   func()
}

func Build(cfg config.Config) (*Runtime, error) {
	wal, err := eventlog.Open(cfg.DataDir+"/wal.jsonl", cfg.AuditFlushEvery, cfg.WALMaxEntries)
	if err != nil {
		return nil, err
	}
	var be backend.Adapter
	switch strings.ToLower(cfg.BackendKind) {
	case string(backend.KindLocal), "":
		be = local.New()
	default:
		be = local.New()
	}
	var st state.Store = stateMemory.New()
	switch strings.ToLower(cfg.StateKind) {
	case "valkey":
		st, err = stateValkey.New(cfg.ValkeyURL, cfg.StateRetention, cfg.HistoryLimit, cfg.ShardID)
		if err != nil {
			wal.Close()
			be.Close()
			return nil, err
		}
	case "memory", "":
		st = stateMemory.New()
	default:
		st = stateMemory.New()
	}
	var br broker.Broker
	switch strings.ToLower(cfg.BrokerKind) {
	case "nats":
		br, err = nats.New(cfg.BrokerURL)
		if err != nil {
			wal.Close()
			be.Close()
			st.Close()
			return nil, err
		}
	case "memory", "":
		br = memory.New(cfg.BrokerBuffer)
	default:
		br = memory.New(cfg.BrokerBuffer)
	}
	var cp checkpoint.Store
	switch strings.ToLower(cfg.CheckpointKind) {
	case "minio":
		cp, err = checkpointMinio.New(cfg.CheckpointEndpoint, cfg.CheckpointAccessKey, cfg.CheckpointSecretKey, cfg.CheckpointBucket, cfg.CheckpointUseSSL, "checkpoints")
		if err != nil {
			wal.Close()
			be.Close()
			st.Close()
			br.Close()
			return nil, err
		}
	case "memory", "":
		cp = checkpointMemory.New()
	default:
		cp = checkpointMemory.New()
	}
	svc := relay.New(cfg, be, br, cp, st, wal)
	closeFn := func() {
		if err := cp.Close(); err != nil {
			log.Printf("checkpoint close: %v", err)
		}
		if err := br.Close(); err != nil {
			log.Printf("broker close: %v", err)
		}
		if err := st.Close(); err != nil {
			log.Printf("state close: %v", err)
		}
		if err := be.Close(); err != nil {
			log.Printf("backend close: %v", err)
		}
		if err := wal.Close(); err != nil {
			log.Printf("wal close: %v", err)
		}
	}
	return &Runtime{
		Service: svc,
		Backend: be,
		Broker:  br,
		Store:   st,
		CP:      cp,
		WAL:     wal,
		Close:   closeFn,
	}, nil
}
