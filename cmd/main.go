package main

import (
	"context"
	"errors"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/cors"
	"log"
	"log/slog"
	"net/http"
	"nf-shard-orchestrator/graph"
	"nf-shard-orchestrator/graph/model"
	"nf-shard-orchestrator/pkg/auth"
	"nf-shard-orchestrator/pkg/cache"
	"nf-shard-orchestrator/pkg/runner"
	"nf-shard-orchestrator/pkg/runner/float"
	"nf-shard-orchestrator/pkg/runner/nextflow"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func main() {
	_ = godotenv.Load()

	authToken := os.Getenv("TOKEN")
	if authToken == "" {
		panic("AUTH_TOKEN environment variable is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT environment variable is not set")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	logOpts := &slog.HandlerOptions{Level: slog.LevelDebug}
	logger := slog.New(slog.NewTextHandler(os.Stdout, logOpts))

	nc, _, js, err := RunEmbeddedNatsServer()
	if err != nil {
		logger.Error("Failed to start NATS srv", "error", err)
		return
	}

	var wg sync.WaitGroup

	logCache := cache.NewCache[model.Log]()

	nfRunnerConfig := nextflow.Config{
		Wg:       &wg,
		Logger:   logger,
		BinPath:  "nextflow",
		Nc:       nc,
		Js:       js,
		LogCache: logCache,
	}
	nfService := nextflow.NewRunner(nfRunnerConfig)

	floatConfig := float.Config{
		Logger:          logger,
		Wg:              &wg,
		FloatBinPath:    "float",
		NextflowBinPath: "nextflow",
		Nc:              nc,
		Js:              js,
		LogCache:        logCache,
	}
	floatService := float.NewRunner(floatConfig)

	go RunGraphQLServer(nc, js, logger, nfService, floatService, &wg, port, logCache)

	<-sigs
	logger.Info("Shutdown signal received")

	// Create a context without a deadline for srv shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for all jobs to complete
	logger.Info("Waiting for all jobs to complete...")
	wg.Wait()

	// Wait for the srv to finish shutting down
	<-ctx.Done()

	logger.Info("All jobs completed and srv shut down. Exiting.")
}

// RunEmbeddedNatsServer - Nats Server + Client, to be replaced with a separate service later
func RunEmbeddedNatsServer() (*nats.Conn, *server.Server, jetstream.JetStream, error) {
	natsOpts := &server.Options{}

	ns, err := server.NewServer(natsOpts)
	if err != nil {
		return nil, nil, nil, err
	}

	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		return nil, nil, nil, errors.New("nats server not ready for connections")
	}

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		return nil, nil, nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, nil, nil, err
	}

	return nc, ns, js, nil
}

func RunGraphQLServer(nc *nats.Conn, js jetstream.JetStream, logger *slog.Logger, nfService runner.Runner, floatService runner.Runner, wg *sync.WaitGroup, port string, logCache *cache.Cache[model.Log]) {
	corsOpts := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
	})

	router := chi.NewRouter()
	router.Use(auth.AuthMiddleware(logger))
	router.Use(corsOpts.Handler)

	srv := handler.New(gqlSchema(logger, nc, js, nfService, floatService, wg, logCache))
	srv.AddTransport(transport.SSE{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})

	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	})

	srv.Use(extension.Introspection{})
	srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		resp := next(ctx)

		if resp != nil && resp.Errors != nil && len(resp.Errors) > 0 {
			oc := graphql.GetOperationContext(ctx)
			logger.Error("gql error", "operation_name", oc.OperationName, "raw_query", oc.RawQuery, "errors", resp.Errors)
		}

		return resp
	})

	router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	router.Handle("/query", corsOpts.Handler(srv))
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func gqlSchema(logger *slog.Logger, nc *nats.Conn, js jetstream.JetStream, nfService runner.Runner, floatService runner.Runner, wg *sync.WaitGroup, logCache *cache.Cache[model.Log]) graphql.ExecutableSchema {

	config := graph.Config{
		Resolvers: &graph.Resolver{
			NatsConn:     nc,
			Logger:       logger,
			NFService:    nfService,
			FloatService: floatService,
			Wg:           wg,
			Nc:           nc,
			Js:           js,
			LogCache:     logCache,
		},
	}
	config.Directives.Authorized = auth.Authorized()
	return graph.NewExecutableSchema(config)
}
