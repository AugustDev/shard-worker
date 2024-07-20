package rest

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey struct {
	name string
}

var userCtxKey = &contextKey{"user"}

func AuthMiddleware(validToken string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Info("Authorization header is missing")
				http.Error(w, "Authorization header is missing", http.StatusUnauthorized)
				return
			}

			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || headerParts[0] != "Bearer" {
				logger.Info("Invalid Authorization header format")
				http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
				return
			}

			token := headerParts[1]
			if token != validToken {
				logger.Info("Invalid token provided")
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey, token)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
