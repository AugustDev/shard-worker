package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/pkg/runner"
	"nf-shard-orchestrator/pkg/runner/float"
	"nf-shard-orchestrator/pkg/runner/nextflow"
	"sync"
)

type runResource struct {
	Logger       *slog.Logger
	NfService    *nextflow.Service
	FloatService *float.Service
	Wg           *sync.WaitGroup
}

func NewService(logger *slog.Logger, nfService *nextflow.Service, fService *float.Service, wg *sync.WaitGroup) *runResource {
	return &runResource{
		Logger:       logger,
		NfService:    nfService,
		FloatService: fService,
		Wg:           wg,
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
		Args:           req.Args(),
		PipelineUrl:    req.PipelineUrl,
		ConfigOverride: req.Executor.ComputeOverride,
	}

	err = runner.MockExecute(r.Context(), s.Logger, run, s.NfService.Config.BinPath)
	if err != nil {
		s.Logger.Error("run", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch req.Executor.Name {
	case "float":
		go func() {
			s.Logger.Info("float job started")
			s.FloatService.Execute(run)
			s.Logger.Info("Job ended")
		}()
	case "awsbatch", "google-batch":
		go func() {
			s.Logger.Info("awsbatch job started")
			s.NfService.Execute(run)
			s.Logger.Info("Job ended")
		}()
	default:
		s.Logger.Error("Invalid executor", "executor", req.Executor.Name)
	}

	res := RunResponse{
		Status: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
