package runner

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"log/slog"
	"nf-shard-orchestrator/graph/model"
	"nf-shard-orchestrator/pkg/cache"
	logstream "nf-shard-orchestrator/pkg/streamlogs"
	"os"
	"os/exec"
	"path/filepath"

	petname "github.com/dustinkirkland/golang-petname"
)

func init() {
	petname.NonDeterministicMode()
}

type RunConfig struct {
	PipelineUrl    string
	ConfigOverride string
	Args           []string
}

type StopConfig struct {
	ProcessId  string
	RunnerName string
}

type Runner interface {
	Execute(ctx context.Context, run RunConfig, runName string) (string, error)
	Stop(s StopConfig) error
	BinPath() string
}

func (r RunConfig) CmdArgs() []string {
	return append([]string{"run", r.PipelineUrl}, r.Args...)
}

func (r RunConfig) Mock() RunConfig {
	r.ConfigOverride = `
	tower { enabled = false } 
	process { executor = "local" }
	`
	r.Args = append(r.Args, "-preview")
	return r
}

func (r RunConfig) RemoveWorkDir() RunConfig {
	args := []string{}

	for i, arg := range r.Args {
		if arg == "-work-dir" || arg == "-bucket-dir" {
			i++
			continue
		}
		args = append(args, arg)
	}

	r.Args = args
	return r
}

func (r RunConfig) AddWorkDirIfNotExists() RunConfig {
	workDirExists := false

	for _, arg := range r.Args {
		if arg == "-work-dir" || arg == "-bucket-dir" {
			workDirExists = true
			break
		}
	}

	if !workDirExists {
		r.Args = append(r.Args, "-work-dir", "/tmp")
	}

	return r
}

func (r RunConfig) SetRunName(runName string) RunConfig {
	args := []string{}

	for i, arg := range r.Args {
		if arg == "-name" {
			i++
			continue
		}
		args = append(args, arg)
	}

	r.Args = append(args, "-name", runName)
	return r

}

func MockExecute(ctx context.Context, logger *slog.Logger, run RunConfig, nextflowBinPath string, nc *nats.Conn, runName string, logCache *cache.Cache[model.Log]) error {
	// unable to simulate workflows with `main-script`
	for _, arg := range run.Args {
		if arg == "-main-script" {
			return nil
		}
	}

	// name has to be unique for mock
	run = run.SetRunName(runName + "-mock")

	// for mocking work dir is nescessary
	run = run.AddWorkDirIfNotExists()

	logger.Info("Running nextflow mock")
	tempDir, err := os.MkdirTemp("", "runner-")
	if err != nil {
		logger.Error("Failed to create temporary directory", "error", err)
		return err
	}

	run = run.Mock()

	configFileName := "injected.config"
	configFilePath := filepath.Join(tempDir, configFileName)
	err = os.WriteFile(configFilePath, []byte(run.ConfigOverride), 0644)
	if err != nil {
		return err
	}

	args := run.CmdArgs()
	args = append(args, "-c", configFilePath)

	command := exec.CommandContext(ctx, nextflowBinPath, args...)
	command.Env = os.Environ()
	output, err := command.CombinedOutput()

	if err != nil {
		logger.Info("nextflow mock error", "error", err, "output", string(output))
		err = logstream.PublishLog(nc, runName, model.Log{Message: err.Error()}, logCache)
		if err != nil {
			logger.Error("Failed to publish log", "error", err)
			return err
		}
		err = logstream.PublishLog(nc, runName, model.Log{Message: string(output)}, logCache)
		if err != nil {
			logger.Error("Failed to publish log", "error", err)
			return err
		}

		// open and print .nextflow.log
		logPath := filepath.Join("", ".nextflow.log")
		logFile, nfErr := os.ReadFile(logPath)
		if nfErr != nil {
			logger.Error("Failed to read .nextflow.log", "error", err)
		}
		logger.Info("nextflow log", "log", string(logFile))
		return fmt.Errorf("%w: %s", err, output)
	}
	logger.Info("nextflow mock succeeded", "output", string(output))
	err = logstream.PublishLog(nc, runName, model.Log{Message: string(output)}, logCache)
	if err != nil {
		logger.Error("Failed to publish log", "error", err)
		return err
	}
	return nil
}
