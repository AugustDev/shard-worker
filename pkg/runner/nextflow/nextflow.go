package nextflow

import (
	"bufio"
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"log/slog"
	"nf-shard-orchestrator/graph/model"
	"nf-shard-orchestrator/pkg/cache"
	"nf-shard-orchestrator/pkg/runner"
	logstream "nf-shard-orchestrator/pkg/streamlogs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

var _ runner.Runner = &Service{}

type Config struct {
	Logger   *slog.Logger
	Wg       *sync.WaitGroup
	BinPath  string
	Js       jetstream.JetStream
	Nc       *nats.Conn
	LogCache *cache.Cache[model.Log]
}

type Service struct {
	Config   Config
	Wg       *sync.WaitGroup
	Logger   *slog.Logger
	Js       jetstream.JetStream
	Nc       *nats.Conn
	LogCache *cache.Cache[model.Log]
}

func NewRunner(c Config) *Service {
	return &Service{
		Config:   c,
		Wg:       c.Wg,
		Logger:   c.Logger,
		Js:       c.Js,
		Nc:       c.Nc,
		LogCache: c.LogCache,
	}
}

func injectConfigFile(configOverride string) (string, error) {
	tempDir, err := os.MkdirTemp("", "float-runner-")
	if err != nil {
		return "", err
	}
	fileName := "injected.config"
	filePath := filepath.Join(tempDir, fileName)
	err = os.WriteFile(filePath, []byte(configOverride), 0644)
	if err != nil {
		return "", err
	}
	return filePath, nil
}

func (s *Service) Execute(bgCtx context.Context, run runner.RunConfig, runName string) (string, error) {
	s.Wg.Add(1)
	defer s.Wg.Done()

	filePath, err := injectConfigFile(run.ConfigOverride)
	if err != nil {
		s.Logger.Error("Failed to inject config file", "error", err)
		return "", err
	}

	args := run.CmdArgs()
	args = append(args, "-c", filePath)

	command := exec.Command(s.Config.BinPath, args...)
	command.Env = os.Environ()

	// Create pipes for stdout and stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		s.Logger.Error("Failed to create stdout pipe", "error", err)
		return "", err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		s.Logger.Error("Failed to create stderr pipe", "error", err)
		return "", err
	}

	err = command.Start()
	if err != nil {
		s.Logger.Error("Failed to start command", "error", err)
		return "", err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			text := scanner.Text()
			msg := model.Log{
				Message: text,
			}
			err = logstream.PublishLog(s.Nc, runName, msg, s.LogCache)
			if err != nil {
				s.Logger.Error("Failed to publish log", "error", err)
			}
			s.Logger.Info("Command output", "stdout", text)

		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			text := scanner.Text()
			s.Logger.Error("Command error output", "stderr", text)
			msg := model.Log{
				Message: text,
			}
			_ = logstream.PublishLog(s.Nc, runName, msg, s.LogCache)
		}
	}()

	go func() {
		defer os.RemoveAll(filepath.Dir(filePath))
		err = command.Wait()
		if err != nil {
			s.Logger.Info("Command exited with error", "error", err)
		}
		wg.Wait()
	}()

	return strconv.Itoa(command.Process.Pid), nil
}

func (s *Service) Stop(c runner.StopConfig) error {
	pid, err := strconv.Atoi(c.ProcessId)
	if err != nil {
		return fmt.Errorf("invalid process ID: %s", c.ProcessId)
	}

	err = runner.GracefullyStopProcessByID(pid)
	if err != nil {
		s.Logger.Info("Failed to stop process", "error", err)
		return err
	}

	return nil
}

func (s *Service) BinPath() string {
	return s.Config.BinPath
}
