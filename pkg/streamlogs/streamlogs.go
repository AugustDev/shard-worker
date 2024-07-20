package logstream

import (
	"encoding/json"
	"fmt"
	"github.com/nats-io/nats.go"
	"nf-shard-orchestrator/graph/model"
	"nf-shard-orchestrator/pkg/cache"
	"regexp"
)

const (
	StreamName    = "WORKFLOW_LOGS"
	SubjectPrefix = "workflows"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsiCodes(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func PublishLog(nc *nats.Conn, runName string, log model.Log, logCache *cache.Cache[model.Log]) error {
	log.Message = stripAnsiCodes(log.Message)
	logCache.Add(runName, log)

	subject := fmt.Sprintf("%s.%s", SubjectPrefix, runName)

	logData, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	err = nc.Publish(subject, logData)
	if err != nil {
		return fmt.Errorf("failed to publish log: %w", err)
	}

	return nil
}
