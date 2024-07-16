package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/pkg/runner/nextflow"
	"sync"
	"time"
)

type runResource struct {
	Logger    *slog.Logger
	NfService *nextflow.Service
	Wg        *sync.WaitGroup
}

func NewService(logger *slog.Logger, nfService *nextflow.Service, wg *sync.WaitGroup) *runResource {
	return &runResource{
		Logger:    logger,
		NfService: nfService,
		Wg:        wg,
	}
}

func (s *runResource) Run(w http.ResponseWriter, req *http.Request) {
	s.Logger.Debug("Received request to launch workflow")

	s.Wg.Add(1) // Add to WaitGroup before starting the goroutine
	go func() {
		defer s.Wg.Done() // Ensure WaitGroup is decremented when the goroutine finishes
		s.Logger.Info("Job started")
		time.Sleep(5 * time.Second) // Simulating job execution
		s.Logger.Info("Job ended")
	}()

	res := RunResponse{
		Status: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
