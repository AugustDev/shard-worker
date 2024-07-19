package rest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/pkg/runner"
	"sync"
)

type runResource struct {
	Logger       *slog.Logger
	NfService    runner.Runner
	FloatService runner.Runner
	Wg           *sync.WaitGroup
}

func NewService(logger *slog.Logger, nfService runner.Runner, fService runner.Runner, wg *sync.WaitGroup) *runResource {
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

	err = runner.MockExecute(r.Context(), s.Logger, run, s.NfService.BinPath())
	if err != nil {
		s.Logger.Error("run", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.Logger.Info("job starting")
	var processId string
	switch req.Executor.Name {
	case "float":
		processId, err = s.FloatService.Execute(run)
	case "awsbatch", "google-batch":
		processId, err = s.NfService.Execute(run)
	default:
		s.Logger.Error("Invalid executor", "executor", req.Executor.Name)
	}

	if err != nil {
		s.Logger.Error("run", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("process running with ", processId)

	res := RunResponse{
		Status:     true,
		ProcessKey: processId,
		Executor:   req.Executor.Name,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *runResource) Stop(w http.ResponseWriter, r *http.Request) {
	s.Logger.Debug("Received request to stop job")

	var req StopJobRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.Logger.Error("run", "error", err)
		http.Error(w, "could not decode request", http.StatusBadRequest)
		return
	}

	stop := runner.StopConfig{
		ProcessId:  req.ProcessId,
		RunnerName: req.Executor,
	}

	switch req.Executor {
	case "float":
		err = s.FloatService.Stop(stop)
	case "awsbatch", "google-batch":
		err = s.NfService.Stop(stop)
	default:
		s.Logger.Error("Invalid executor", "executor", req.Executor)
	}

	if err != nil {
		s.Logger.Error("stop process", "error", err)
		http.Error(w, "could not stop process", http.StatusBadRequest)
	}

	res := StopJobResponse{
		Status: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
