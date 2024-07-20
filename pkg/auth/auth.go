package auth

import (
	"context"
	"errors"
	"github.com/99designs/gqlgen/graphql"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type contextKey struct {
	name string
}

var userCtxKey = &contextKey{"user"}

func AuthMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) == 2 || headerParts[0] == "Bearer" {
				token := headerParts[1]
				ctx := context.WithValue(r.Context(), userCtxKey, token)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func Authorized() func(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {
	return func(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {
		appToken := os.Getenv("TOKEN")
		userToken, ok := ctx.Value(userCtxKey).(string)
		if !ok {
			return nil, errors.New("access denied: invalid token")
		}

		if userToken != appToken {
			return nil, errors.New("access denied: invalid token")
		}

		return next(ctx)
	}
}
