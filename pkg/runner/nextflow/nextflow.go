package nextflow

import (
	"bufio"
	"fmt"
	"log/slog"
	"nf-shard-orchestrator/pkg/runner"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

var _ runner.Runner = &Service{}

type Config struct {
	Logger  *slog.Logger
	Wg      *sync.WaitGroup
	BinPath string
}

type Service struct {
	Config Config
	Wg     *sync.WaitGroup
	Logger *slog.Logger
}

func NewRunner(c Config) *Service {
	return &Service{
		Config: c,
		Wg:     c.Wg,
		Logger: c.Logger,
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

func (s *Service) Execute(run runner.RunConfig) (string, error) {
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
			s.Logger.Info("Command output", "stdout", scanner.Text())
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			s.Logger.Error("Command error output", "stderr", scanner.Text())
		}
	}()

	go func() {
		defer os.RemoveAll(filepath.Dir(filePath))
		err = command.Wait()
		if err != nil {
			s.Logger.Debug("Command exited with error", "error", err)
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
