package builder

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"mev-relay/internal/pb"
)

// DBPublisher pushes build results into TimescaleDB
type DBPublisher struct {
	db *pgxpool.Pool
}

// NewDBPublisher creates a DBPublisher with a connection pool
func NewDBPublisher(pool *pgxpool.Pool) *DBPublisher {
	return &DBPublisher{db: pool}
}

// Publish inserts the build result into the database
func (p *DBPublisher) Publish(result *pb.BuildResult) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	INSERT INTO block_builds (
		block_hash,
		included,
		inclusion_reason,
		inclusion_latency_ms,
		timestamp
	) VALUES ($1, $2, $3, $4, NOW());
	`

	_, err := p.db.Exec(ctx, query,
		result.BlockHash,
		result.Included,
		result.InclusionReason,
		result.InclusionLatencyMs,
	)

	if err != nil {
		log.Println("[DBPublisher] Failed to insert build result:", err)
		return err
	}

	log.Printf("[DBPublisher] Stored block %s in TimescaleDB\n", result.BlockHash)
	return nil
}
