package runner

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

type RunConfig struct {
	PipelineUrl    string
	ConfigOverride string
	Args           []string
}

type Runner interface {
	Execute(run RunConfig)
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

func MockExecute(logger *slog.Logger, run RunConfig, nextflowBinPath string) error {
	// unable to simulate workflows with `main-script`
	for _, arg := range run.Args {
		if arg == "-main-script" {
			return nil
		}
	}

	logger.Debug("Running nextflow mock")
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

	command := exec.Command(nextflowBinPath, args...)
	command.Env = []string{fmt.Sprintf("GITHUB_TOKEN=%s", os.Getenv("GITHUB_TOKEN"))}
	output, err := command.CombinedOutput()

	if err != nil {
		logger.Debug("nextflow mock error", "error", err, "output", string(output))

		// open and print .nextflow.log
		logPath := filepath.Join("", ".nextflow.log")
		logFile, nfErr := os.ReadFile(logPath)
		if nfErr != nil {
			logger.Error("Failed to read .nextflow.log", "error", err)
		}
		logger.Debug("nextflow log", "log", string(logFile))

		return fmt.Errorf("%w: %s", err, output)
	}
	logger.Debug("nextflow mock succeeded", "output", string(output))
	return nil
}
