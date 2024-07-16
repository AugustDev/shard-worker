package rest

import (
	"log/slog"
	"net/http"
	"time"
)

type Router struct {
	*http.ServeMux
	logger *slog.Logger
}

func NewRouter(logger *slog.Logger) *Router {
	return &Router{
		ServeMux: http.NewServeMux(),
		logger:   logger,
	}
}

func (r *Router) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, req)
		r.logger.Info("request processed",
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.String("remote_addr", req.RemoteAddr),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func (r *Router) Handle(pattern string, handler http.Handler) {
	r.ServeMux.Handle(pattern, r.logMiddleware(handler))
}

func (r *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.ServeMux.HandleFunc(pattern, r.logMiddleware(http.HandlerFunc(handler)).ServeHTTP)
}
