package graph

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"log/slog"
	"nf-shard-orchestrator/graph/model"
	"nf-shard-orchestrator/pkg/cache"
	"nf-shard-orchestrator/pkg/runner"
	"sync"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	NatsConn     *nats.Conn
	Logger       *slog.Logger
	NFService    runner.Runner
	FloatService runner.Runner
	Wg           *sync.WaitGroup
	Nc           *nats.Conn
	Js           jetstream.JetStream
	LogCache     *cache.Cache[model.Log]
}
