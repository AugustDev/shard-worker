package main

import (
	"context"
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/pkg/rest"
	"nf-shard-orchestrator/pkg/runner/nextflow"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))

	nfRunnerConfig := nextflow.Config{
		Logger:  logger,
		BinPath: "nextflow",
	}

	runService := nextflow.NewRunner(nfRunnerConfig)

	var wg sync.WaitGroup

	router := rest.Handler(logger, runService, &wg)
	server := &http.Server{
		Addr:    ":4001",
		Handler: router,
	}

	// Channel to signal when the server has stopped
	serverStopped := make(chan struct{})

	// Start the HTTP server in a goroutine
	go func() {
		logger.Info("Starting server on :4001")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Could not start server", "error", err)
		}
		close(serverStopped)
	}()

	// Wait for interrupt signal
	<-sigs
	logger.Info("Shutdown signal received")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Attempt to gracefully shutdown the server
	logger.Info("Shutting down server...")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	// Wait for the server to finish
	<-serverStopped

	// Wait for all jobs to complete
	logger.Info("Waiting for all jobs to complete...")
	wg.Wait()

	logger.Info("All jobs completed. Exiting.")
}
