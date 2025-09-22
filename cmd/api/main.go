package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Werneck0live/cadastro-empresa/internal/admin"
	"github.com/Werneck0live/cadastro-empresa/internal/broker"
	"github.com/Werneck0live/cadastro-empresa/internal/config"
	"github.com/Werneck0live/cadastro-empresa/internal/db"
	"github.com/Werneck0live/cadastro-empresa/internal/handlers"
	"github.com/Werneck0live/cadastro-empresa/internal/repository"
)

// var _ handlers.Publisher = (*NoopPublisher)(nil)

// type NoopPublisher struct{}

// func (NoopPublisher) Publish(ctx context.Context, body string, headers amqp091.Table) error {
// 	return nil
// }
// func (NoopPublisher) Close() error { return nil }

func main() {
	var (
		task = flag.String("task", "", "admin task: seed|migrate|index")
	)
	flag.Parse()

	cfg := config.Load()

	_ = config.InitLogger(cfg.LogLevel)
	slog.Info("starting", "port", cfg.Port, "mongo_db", cfg.MongoDB)

	// conecta Mongo
	client, err := db.NewMongoClient(cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect error: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	// repo ANTES do switch (seed precisa dele)
	repo := repository.NewCompanyRepository(client.Database(cfg.MongoDB))

	// --- ADMIN TASKS Ex.: rodar as seeds - (rodam e saem)
	switch *task {
	case "seed":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := admin.SeedCompanies(ctx, repo, slog.Default()); err != nil {
			slog.Error("seed_error", "err", err)
			os.Exit(1)
		}
		slog.Info("seed_done")
		return

	case "migrate", "index":
		slog.Info("task_not_implemented", "task", *task)
		return
	}

	// --- Modo servidor normal: conecta Rabbit com backoff
	pub, err := connectRabbitWithRetry(cfg.RabbitURI, cfg.RabbitQueue, 60*time.Second, slog.Default())
	if err != nil {
		slog.Error("rabbitmq_connect_error", "uri", cfg.RabbitURI, "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	h := &handlers.CompanyHandler{Repo: repo, Pub: pub}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Health)
	mux.HandleFunc("/api/companies", h.Companies)
	mux.HandleFunc("/api/companies/", h.CompanyByID)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           logMiddleware(mux),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	// start server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful_shutdown_error", "err", err)
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start)
		// usando slog com campos estruturados
		slog.Info("http_request",
			"method", r.Method, "path", r.URL.Path, "duration", fmtDuration(dur),
		)
	})
}

func fmtDuration(d time.Duration) string { return fmt.Sprintf("%dms", d.Milliseconds()) }

// retorna *broker.Publisher (implementa handlers.Publisher)
func connectRabbitWithRetry(uri, queue string, maxWait time.Duration, log *slog.Logger) (*broker.Publisher, error) {
	deadline := time.Now().Add(maxWait)
	base := 200 * time.Millisecond
	for attempt := 1; ; attempt++ {
		pub, err := broker.NewPublisher(uri, queue)
		if err == nil {
			return pub, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		sleep := time.Duration(1<<uint(min(attempt, 6))) * base
		jitter := time.Duration(rand.Int63n(int64(sleep / 3)))
		wait := sleep/2 + jitter
		slog.Warn("rabbit_connect_retry", "attempt", attempt, "wait", wait, "err", err)
		time.Sleep(wait)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
