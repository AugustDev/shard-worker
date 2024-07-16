package nextflow

import (
	"log/slog"
	"nf-shard-orchestrator/pkg/runner"
	"os/exec"
	"sync"
)

type Config struct {
	Logger  *slog.Logger
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
		Wg:     &sync.WaitGroup{},
		Logger: c.Logger,
	}
}

func (s *Service) Execute(run runner.RunConfig) {
	s.Wg.Add(1)
	defer s.Wg.Done()

	command := exec.Command(s.Config.BinPath, run.CmdArgs()...)
	output, err := command.CombinedOutput()

	s.Logger.Debug("nextflow exec output", "output", string(output))

	if err != nil {
		s.Logger.Debug("nextflow exec error", "error", err)
		return
	}
}
