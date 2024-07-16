package rest

import (
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/pkg/runner/nextflow"
	"sync"

	"github.com/go-chi/chi"
)

func Handler(logger *slog.Logger, ns *nextflow.Service, wg *sync.WaitGroup) http.Handler {
	router := chi.NewRouter()

	runResource := NewService(logger, ns, wg)

	router.Route("/v1", func(r chi.Router) {
		r.Post("/run", runResource.Run)
	})

	return router
}
