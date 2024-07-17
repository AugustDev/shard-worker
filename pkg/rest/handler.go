package rest

import (
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/pkg/runner/nextflow"
	"sync"

	"github.com/go-chi/chi"
	"github.com/rs/cors"
)

func Handler(logger *slog.Logger, ns *nextflow.Service, wg *sync.WaitGroup, authToken string) http.Handler {
	router := chi.NewRouter()

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	router.Use(corsMiddleware.Handler)

	runResource := NewService(logger, ns, wg)

	router.Route("/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(authToken, logger))
			r.Post("/run", runResource.Run)
			r.Get("/health", health)
		})
	})

	return router
}
