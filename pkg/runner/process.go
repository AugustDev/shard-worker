package runner

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func killProcessByID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process with PID %d: %v", pid, err)
	}

	err = process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill process with PID %d: %v", pid, err)
	}

	_, err = process.Wait()
	if err != nil {
		return fmt.Errorf("error waiting for process %d to exit: %v", pid, err)
	}

	return nil
}

func GracefullyStopProcessByID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process with PID %d: %v", pid, err)
	}

	// Send SIGTERM
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %v", pid, err)
	}

	// Wait for the process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("error waiting for process %d to exit: %v", pid, err)
		}
	case <-time.After(15 * time.Second):
		// If the process doesn't exit within 10 seconds, force kill it
		return killProcessByID(pid)
	}

	return nil
}
