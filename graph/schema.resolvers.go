package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"nf-shard-orchestrator/graph/model"
	"nf-shard-orchestrator/pkg/runner"
	logstream "nf-shard-orchestrator/pkg/streamlogs"
	"time"
)

var logsCache = make(map[string][]*model.Log)

// RunJob is the resolver for the runJob field.
func (r *mutationResolver) RunJob(ctx context.Context, input model.RunJobCommand) (*model.RunJobResponse, error) {
	r.Logger.Debug("Received request to launch workflow")

	run := runner.RunConfig{
		Args:           input.Args(),
		PipelineUrl:    input.PipelineURL,
		ConfigOverride: input.Executor.ComputeOverride,
	}
	run = run.SetRunName(input.RunName)

	bgCtx := context.Background()
	err := runner.RemoveNfAssetsDir()
	if err != nil {
		r.Logger.Error("run", "error", err)
		return nil, err
	}

	err = runner.MockExecute(ctx, r.Logger, run, r.NFService.BinPath(), r.Nc, input.RunName, r.LogCache)
	if err != nil {
		r.Logger.Error("run", "error", err)
		return nil, err
	}

	r.Logger.Info("job starting")
	var processId string
	switch input.Executor.Name {
	case "float":
		processId, err = r.FloatService.Execute(bgCtx, run, input.RunName)
	case "awsbatch", "google-batch":
		processId, err = r.NFService.Execute(bgCtx, run, input.RunName)
	default:
		r.Logger.Error("Invalid executor", "executor", input.Executor.Name)
	}

	if err != nil {
		r.Logger.Error("run", "error", err)
		return nil, err
	}

	r.Logger.Info("process running", "process_id", processId)

	return &model.RunJobResponse{
		Status:     true,
		ProcessKey: processId,
		Executor:   input.Executor.Name,
		RunName:    input.RunName,
	}, nil
}

// TerminateJob is the resolver for the terminateJob field.
func (r *mutationResolver) TerminateJob(ctx context.Context, input model.TerminateJobCommand) (bool, error) {
	r.Logger.Debug("Received request to stop job")

	terminate := runner.StopConfig{
		ProcessId:  input.ProcessKey,
		RunnerName: input.Executor,
	}

	var err error
	switch input.Executor {
	case "float":
		err = r.FloatService.Stop(terminate)
	case "awsbatch", "google-batch":
		err = r.NFService.Stop(terminate)
	default:
		return false, errors.New("could not find executor")
	}

	if err != nil {
		r.Logger.Error("stop process", "error", err)
		return false, err
	}

	return true, nil
}

// HealthCheck is the resolver for the healthCheck field.
func (r *queryResolver) HealthCheck(ctx context.Context) (bool, error) {
	fmt.Println("healh check now")
	return true, nil
}

// CheckStatus is the resolver for the checkStatus field.
func (r *queryResolver) CheckStatus(ctx context.Context) (bool, error) {
	return true, nil
}

// StreamLogs is the resolver for the streamLogs field.
func (r *subscriptionResolver) StreamLogs(ctx context.Context, runName string) (<-chan *model.Log, error) {
	if runName == "" {
		return nil, nil
	}

	// Create a buffered channel to handle log spikes
	logChan := make(chan *model.Log, 1000) // Adjust buffer size as needed
	subject := fmt.Sprintf("%s.%s", logstream.SubjectPrefix, runName)

	go func() {
		defer close(logChan)

		sub, err := r.Nc.Subscribe(subject, func(msg *nats.Msg) {
			var log model.Log
			if err := json.Unmarshal(msg.Data, &log); err != nil {
				r.Logger.Error("Failed to unmarshal log", "error", err)
				return
			}

			time.Sleep(50 * time.Millisecond)
			select {
			case logChan <- &log:
			case <-ctx.Done():
				return
			}
		})
		if err != nil {
			select {
			case logChan <- &model.Log{Message: fmt.Sprintf("Failed to subscribe to NATS: %v", err)}:
			case <-ctx.Done():
			}
			return
		}
		defer sub.Unsubscribe()

		// Wait for context cancellation
		<-ctx.Done()
	}()

	go func() {
		// Feed logs from cache
		logs := r.LogCache.Get(runName)
		for _, log := range logs {
			time.Sleep(50 * time.Millisecond)
			select {
			case logChan <- &log:
			case <-ctx.Done():
				return
			}
		}
	}()

	return logChan, nil
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

// Subscription returns SubscriptionResolver implementation.
func (r *Resolver) Subscription() SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
