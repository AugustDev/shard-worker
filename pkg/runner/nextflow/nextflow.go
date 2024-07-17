package nextflow

import (
	"log/slog"
	"nf-shard-orchestrator/pkg/runner"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

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

func (s *Service) Execute(run runner.RunConfig) {
	s.Wg.Add(1)
	defer s.Wg.Done()

	filePath, err := injectConfigFile(run.ConfigOverride)
	if err != nil {
		s.Logger.Error("Failed to inject config file", "error", err)
		return
	}
	defer os.RemoveAll(filepath.Dir(filePath))

	args := run.CmdArgs()
	args = append(args, "-c", filePath)

	command := exec.Command(s.Config.BinPath, args...)
	output, err := command.CombinedOutput()

	if err != nil {
		s.Logger.Debug("nextflow exec error", "error", err)
		return
	}
	s.Logger.Debug("nextflow exec output", "output", string(output))
}
