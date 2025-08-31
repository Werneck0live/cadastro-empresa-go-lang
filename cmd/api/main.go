package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
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

// cmd/api/main.go
func main() {
	cfg := config.Load() // .env

	// Logger JSON "global" - permite usar slog.Info/slog.Error/Warn em qualquer lugar
	_ = config.InitLogger(cfg.LogLevel)
	slog.Info("starting", "port", cfg.Port, "mongo_db", cfg.MongoDB)

	// HOOK: admin job (one-off)
	task := flag.String("task", "", "admin task: seed")
	flag.Parse()
	if *task != "" {
		switch *task {
		case "seed":
			// conecta somente o necessário para o seed
			client, err := db.NewMongoClient(cfg.MongoURI)
			if err != nil {
				slog.Error("mongo_connect_error", "err", err)
				os.Exit(1)
			}
			defer func() { _ = client.Disconnect(context.Background()) }()

			repo := repository.NewCompanyRepository(client.Database(cfg.MongoDB))
			if err := admin.SeedCompanies(context.Background(), repo, slog.Default()); err != nil {
				slog.Error("seed_failed", "err", err)
				os.Exit(1)
			}
			slog.Info("seed_done")
			return // encerra o processo sem subir HTTP
		default:
			slog.Error("unknown_admin_task", "task", *task)
			os.Exit(2)
		}
	}

	// conecta Mongo
	client, err := db.NewMongoClient(cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect error: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	// publisher (Rabbit)
	pub, err := broker.NewPublisher(cfg.RabbitURI, cfg.RabbitQueue)
	if err != nil {
		log.Fatalf("rabbitmq connect error: %v", err)
	}
	defer pub.Close()

	repo := repository.NewCompanyRepository(client.Database(cfg.MongoDB))
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
		// log.Printf("graceful shutdown error: %v", err)
		slog.Error("graceful shutdown error", "err", err)
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start)
		// log.Printf("%s %s %s", r.Method, r.URL.Path, fmtDuration(dur))
		slog.Info("%s %s %s", r.Method, r.URL.Path, fmtDuration(dur))
	})
}

func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}
