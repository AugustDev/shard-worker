package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/pkg/runner"
	"nf-shard-orchestrator/pkg/runner/nextflow"
	"sync"
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

func (s *runResource) Run(w http.ResponseWriter, r *http.Request) {
	s.Logger.Debug("Received request to launch workflow")

	var req RunRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.Logger.Error("run", "error", err)
		http.Error(w, "could not decode request", http.StatusBadRequest)
		return
	}

	run := runner.RunConfig{
		Args:        req.Args(),
		PipelineUrl: req.PipelineUrl,
	}

	go func() {
		s.Logger.Info("Job started")
		s.NfService.Execute(run)
		s.Logger.Info("Job ended")
	}()

	res := RunResponse{
		Status: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
