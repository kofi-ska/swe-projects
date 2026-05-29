package builder

import (
	"encoding/json"
	"log"

	"mev-relay/internal/pb"
)

type LogPublisher struct{}

func (l *LogPublisher) Publish(result *pb.BuildResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	log.Println("[LogPublisher] Build Result:")
	log.Println(string(data))
	return nil
}
